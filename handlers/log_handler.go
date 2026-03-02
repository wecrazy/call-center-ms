package handlers

import (
	"os"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func RenderLogs() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Render("logs", fiber.Map{})
	}
}

func GetLogs() fiber.Handler {
	return func(c *fiber.Ctx) error {
		{
			files, err := os.ReadDir("./logs")
			if err != nil {
				return c.Status(500).SendString("Failed to read log directory")
			}
			var list []string
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".log") {
					list = append(list, f.Name())
				}
			}
			return c.JSON(list)
		}
	}
}

func LogContent() fiber.Handler {
	return func(c *fiber.Ctx) error {
		type DataTableReq struct {
			Draw   string `form:"draw"`
			Start  int    `form:"start"`
			Length int    `form:"length"`
			Search struct {
				Value string `form:"value"`
			} `form:"search"`
			Order []struct {
				Column int    `form:"column"`
				Dir    string `form:"dir"`
			} `form:"order"`
			Columns []struct {
				Data string `form:"data"`
			} `form:"columns"`
			File string `form:"file"`
		}
		req := new(DataTableReq)
		if err := c.BodyParser(req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		if req.File == "" {
			return c.Status(400).JSON(fiber.Map{"error": "No file selected"})
		}

		raw, err := os.ReadFile("./logs/" + req.File)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Unable to read file"})
		}

		lines := strings.Split(string(raw), "\n")

		// Filter
		search := strings.ToLower(req.Search.Value)
		var filtered []string
		for _, line := range lines {
			if search == "" || strings.Contains(strings.ToLower(line), search) {
				filtered = append(filtered, line)
			}
		}

		// Sort (only 1 column, basic ASC/DESC)
		if len(req.Order) > 0 {
			dir := strings.ToLower(req.Order[0].Dir)
			sort.Slice(filtered, func(i, j int) bool {
				if dir == "desc" {
					return filtered[i] > filtered[j]
				}
				return filtered[i] < filtered[j]
			})
		}

		// Paginate
		end := req.Start + req.Length
		if end > len(filtered) {
			end = len(filtered)
		}

		data := [][]string{}
		for _, line := range filtered[req.Start:end] {
			data = append(data, []string{line})
		}

		return c.JSON(fiber.Map{
			"draw":            req.Draw,
			"recordsTotal":    len(lines),
			"recordsFiltered": len(filtered),
			"data":            data,
		})
	}
}
