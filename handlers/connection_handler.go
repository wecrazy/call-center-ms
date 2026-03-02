package handlers

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

func GetWiFi_IPv4() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to list network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Name != "Wi-Fi" {
			continue
		}
		if iface.Name != "Wi-Fi 2" {
			continue
		}
		if iface.Name != "Wi-Fi2" {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return "", fmt.Errorf("failed to get addresses for interface %s: %w", iface.Name, err)
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no active Wi-Fi IPv4 address found")
}

func GetWIFIIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("Error getting network interfaces:", err)
		return ""
	}

	localIP := ""
	for _, iface := range interfaces {
		// Skip interfaces that are not up or are loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Match Wi-Fi interfaces by name (case-insensitive comparison)
		if !strings.Contains(strings.ToLower(iface.Name), "wi-fi") && !strings.Contains(strings.ToLower(iface.Name), "wlan") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Println("Error getting addresses for interface:", iface.Name, "-", err)
			continue
		}

		var ipv6 string
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ipNet.IP.To4() != nil {
					// Return IPv4 if available
					return ipNet.IP.String()
				} else if ipNet.IP.To16() != nil {
					// Store IPv6 as a fallback
					ipv6 = ipNet.IP.String()
				}
			}
		}

		// Use IPv6 if no IPv4 was found
		if localIP == "" && ipv6 != "" {
			localIP = ipv6
		}
	}

	return localIP
}

func GetWiFi_SSID() (string, error) {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get SSID on Windows: %w", err)
	}

	outputStr := string(output)
	ssidIndex := strings.Index(outputStr, "SSID")
	if ssidIndex == -1 {
		return "", fmt.Errorf("SSID not found")
	}

	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "SSID") {
			ssid := strings.TrimSpace(strings.Split(line, ":")[1])
			return ssid, nil
		}
	}

	return "", fmt.Errorf("SSID not found")
}
