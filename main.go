package main

import (
	"call_center_app/config"
	"call_center_app/database"
	"call_center_app/database/seed"
	"call_center_app/handlers"
	"call_center_app/routes"
	"call_center_app/whatsapp"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fmt"
	"io"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
	"gopkg.in/natefinch/lumberjack.v2"
)

func windowsServiceInstall(yamlCfg *config.YamlConfig) {
	fmt.Println("🪟  Windows detected — running Windows install steps...")
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	serviceName := yamlCfg.App.Name
	serviceName = strings.ReplaceAll(strings.TrimSpace(serviceName), " ", "")
	if len(serviceName) == 0 {
		log.Fatalf("Service name cannot be empty or whitespace only")
	}

	nssmPath, err := exec.LookPath("nssm")
	if err != nil {
		nssmPath = yamlCfg.Default.NssmFullPath
		if _, err := os.Stat(nssmPath); os.IsNotExist(err) {
			log.Fatalf("NSSM not found in PATH and default path is invalid: %v", err)
		}
	}

	cmd := exec.Command(nssmPath, "install", serviceName, execPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to install service via NSSM: %v", err)
	}

	fmt.Println("✅ Windows service installed successfully using NSSM.")
	fmt.Printf("🔗 Try Win + R, and input services.msc and search for the service name: %s\n", serviceName)
}

func linuxServiceInstall(yamlCfg *config.YamlConfig) {
	fmt.Println("🐧 Linux detected — running Linux install steps...")

	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	serviceName := yamlCfg.App.Name
	serviceName = strings.ReplaceAll(strings.TrimSpace(serviceName), " ", "")
	if len(serviceName) == 0 {
		log.Fatalf("Service name cannot be empty or whitespace only")
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
ExecStart=%s
Restart=always
`, yamlCfg.App.Name, execPath)

	isRoot := os.Geteuid() == 0
	var servicePath string
	var enableCmd, startCmd, daemonReloadCmd *exec.Cmd

	if isRoot {
		// Install as a system-wide service
		serviceContent += "User=root\n\n[Install]\nWantedBy=multi-user.target\n"
		servicePath = fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
		daemonReloadCmd = exec.Command("systemctl", "daemon-reexec")
		enableCmd = exec.Command("systemctl", "enable", serviceName)
		startCmd = exec.Command("systemctl", "start", serviceName)
	} else {
		// Install as a user-level service
		userServiceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
		err := os.MkdirAll(userServiceDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create user systemd directory: %v", err)
		}
		serviceContent += "\n[Install]\nWantedBy=default.target\n"
		servicePath = filepath.Join(userServiceDir, fmt.Sprintf("%s.service", serviceName))
		daemonReloadCmd = exec.Command("systemctl", "--user", "daemon-reexec")
		enableCmd = exec.Command("systemctl", "--user", "enable", serviceName)
		startCmd = exec.Command("systemctl", "--user", "start", serviceName)
	}

	// Write the service file
	err = os.WriteFile(servicePath, []byte(serviceContent), 0644)
	if err != nil {
		log.Fatalf("Failed to write service file: %v", err)
	}

	// Run systemctl commands
	for _, cmd := range []*exec.Cmd{daemonReloadCmd, enableCmd, startCmd} {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to execute command: %v", err)
		}
	}

	fmt.Println("✅ Linux service installed and started successfully.")
	if isRoot {
		fmt.Printf("🔗 Check status using: systemctl status %s\n", serviceName)
	} else {
		fmt.Printf("🔗 Check status using: systemctl --user status %s\n", serviceName)
		fmt.Println("💡 Optional: Run `loginctl enable-linger $(whoami)` to enable service after reboot")
	}
}

func HandleCLIArgs(yamlCfg *config.YamlConfig) bool {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		switch arg {
		case "--install":
			fmt.Println("🔧 Running install process...")

			switch runtime.GOOS {
			case "windows":
				windowsServiceInstall(yamlCfg)
			case "linux":
				linuxServiceInstall(yamlCfg)
			case "darwin":
				fmt.Println("🍎 macOS detected — but we are sorry, we don't have macOS installer yet")
			default:
				fmt.Printf("⚠️ Unsupported OS: %s\n", runtime.GOOS)
			}

			return true
		default:
			fmt.Printf("⚠️ Unknown argument: %s\n", arg)
			return false
			// os.Exit(1)
		}
	}
	return false
}

func main() {
	// Dynamic update conf.yaml
	if err := config.LoadConfig(); err != nil {
		log.Fatal(err)
	}

	go config.WatchConfig()

	yamlConfig := config.GetConfig()

	if HandleCLIArgs(&yamlConfig) {
		return
	}

	db, err := database.InitDB(&yamlConfig)
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}

	seed.UserSeed(db, &yamlConfig)

	appEngine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{
		AppName:       fmt.Sprintf("%v - v%v", yamlConfig.App.Name, yamlConfig.App.Version),
		Views:         appEngine,
		Prefork:       false,
		CaseSensitive: true,
		// ErrorHandler: routes.CustomErrorHandler(),
		// RequestMethods: []string{"POST", "HEAD", "GET"},
		// BodyLimit:      10 * 1024 * 1024,
		// Concurrency:   100,
	})

	logFile := &lumberjack.Logger{
		Filename:   "./logs/app.log",
		MaxSize:    70,
		MaxBackups: 7,
		MaxAge:     7,
		Compress:   true,
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)

	app.Use(logger.New(logger.Config{
		Output: multiWriter,
		Format: "${time} " +
			"${pid} | " +
			"Latency: ${latency} | " +
			"${status} | " +
			"${method} | " +
			"${protocol} - ${host} ${path} | " +
			"Referer: ${referer} | " +
			"Port: ${port} | " +
			"IP: ${ip} | " +
			"IPs: ${ips} | " +
			"UA: ${ua} | " +
			"RequestHeaders: ${reqHeaders} | " +
			"ResponseHeaders: ${respHeaders} | " +
			// "Body: ${body} | " +
			// "ResponseBody: ${respBody} | " +
			"Cookies: ${cookie} | " +
			"QueryParams: ${queryParams} | " +
			"Route: ${route} | " +
			"BytesSent: ${bytesSent} | " +
			"BytesReceived: ${bytesReceived} | " +
			"Error: ${error}\n",
		TimeZone: "Asia/Jakarta",
		// TimeFormat:    "02-Jan-2006 15:04:05.000 MST",
		TimeFormat:    "2006/01/02 15:04:05",
		DisableColors: false,
	}))

	app.Static("/static", "./public", fiber.Static{
		CacheDuration: 24 * time.Hour,
		Compress:      true,
		MaxAge:        31536000,
	})

	// app.Static("/static", "./public", fiber.Static{
	// 	// Compress:      true,           // Enable compression for static files.
	// 	// CacheDuration: 24 * time.Hour, // Cache inactive file handlers for 24 hours.
	// 	// MaxAge:        31536000,
	// 	// MaxAge:        31536000,
	// 	// ModifyResponse: func(c *fiber.Ctx) error {
	// 	// 	c.Response().Header.Set("Cache-Control", "public, max-age=31536000, immutable")
	// 	// 	return nil
	// 	// },
	// })

	routes.ServerRoutes(app, &yamlConfig, db)

	/* 🚀 Start Go Routines (Running Background Processes) */
	// 📲 Handles WhatsApp messaging with WhatsMeow
	go handlers.WaWhatsmeow(db, &yamlConfig)

	// ☎️ Call Center Handler
	// go goroutine.CSCall(db, &yamlConfig) // ⚠️ DO NOT comment this soon!!

	// 📢 WhatsApp Group Follow-Up Feedback Processing
	go whatsapp.FeedbackResultfromFUCC()

	// 🔄 Sync Updated ODOO Data from WhatsApp Feedback Requests
	go whatsapp.UpdateDatainODOOFromDataRequestWhatsapp()

	appPort := yamlConfig.App.Port
	if err := app.Listen(fmt.Sprintf(":%v", appPort)); err != nil {
		log.Fatalf("failed to start the server: %v", err)
	}
}
