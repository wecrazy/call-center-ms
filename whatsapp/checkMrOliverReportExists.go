package whatsapp

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"
)

var checkMrOliverReportMutex sync.Mutex

type Report struct {
	ReportType  string
	Links       []string
	CanDownload map[string]bool // Stores link -> status
}

// checkLinkAvailability checks if a link returns a 200 status
func checkLinkAvailability(url string) bool {
	resp, err := http.Head(url) // HEAD request for faster response
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func findRealURL(baseURL string) string {
	for hour := 0; hour < 24; hour++ {
		for minute := 0; minute < 60; minute++ {
			realTime := fmt.Sprintf("%02d_%02d", hour, minute)        // Format as HH_MM
			testURL := strings.Replace(baseURL, "xx_xx", realTime, 1) // Replace in the URL

			if checkLinkAvailability(testURL) { // Check if it exists
				return testURL // Return the first valid link
			}
		}
	}
	return "" // Return empty if no valid URL found
}

// fetchFileList gets a list of .xlsx files from the directory URL
func fetchFileList(directoryURL string) ([]string, error) {
	resp, err := http.Get(directoryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %v", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract file names using regex
	fileRegex := regexp.MustCompile(`href="([^"]+\.xlsx)"`)
	matches := fileRegex.FindAllStringSubmatch(string(body), -1)

	var files []string
	for _, match := range matches {
		files = append(files, match[1])
	}

	return files, nil
}

// getLastModified fetches the "Last-Modified" header for a given file URL
func getLastModified(fileURL string) (time.Time, error) {
	resp, err := http.Head(fileURL) // HEAD request for metadata
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	lastModifiedStr := resp.Header.Get("Last-Modified")
	if lastModifiedStr == "" {
		return time.Time{}, fmt.Errorf("no Last-Modified header found")
	}

	lastModified, err := time.Parse(time.RFC1123, lastModifiedStr)
	if err != nil {
		return time.Time{}, err
	}

	return lastModified, nil
}

// getLatestFile finds the most recently modified file
func getLatestFile(directoryURL string) (string, error) {
	files, err := fetchFileList(directoryURL)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no .xlsx files found")
	}

	var latestFile string
	var latestTime time.Time

	// Check each file's "Last-Modified" header
	for _, file := range files {
		fileURL := directoryURL + file
		modifiedTime, err := getLastModified(fileURL)
		if err != nil {
			fmt.Println("⚠ Error checking:", fileURL, "->", err)
			continue
		}

		// Keep the most recent file
		if modifiedTime.After(latestTime) {
			latestTime = modifiedTime
			latestFile = fileURL
		}
	}

	if latestFile == "" {
		return "", fmt.Errorf("could not determine the latest modified file")
	}

	return latestFile, nil
}

func (h *WhatsmeowHandler) CheckMrOliverReportIsExists() {
	if !checkMrOliverReportMutex.TryLock() {
		log.Println("CheckMrOliverReportIsExists is already running, skipping execution.")
		return
	}
	defer checkMrOliverReportMutex.Unlock()

	taskDoing := "Check data report for Mr. Oliver is already exists or not"

	log.Printf("Running scheduler %v @%v", taskDoing, time.Now())

	jidString := h.YamlCfg.Whatsmeow.GroupTestJID + "@g.us"
	jid, err := types.ParseJID(jidString)
	if err != nil {
		h.sendWhatsAppMessage(jid, "⚠ Invalid JID format. Report generation aborted.")
		return
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		h.sendWhatsAppMessage(jid, err.Error())
		return
	}
	now := time.Now().In(loc)

	var yesterdayHistoryJOPlannedForTechniciansLink string
	testURLyesterdayHistoryJOPlannedForTechniciansLink := fmt.Sprintf("%v:%v/task_schedule/DAILY_TECHNICIAN_REPORT/file/HistoryJOTechnicians_%v_xx_xx_.xlsx",
		h.YamlCfg.Default.OdooDashboardReportingPHPServer,
		h.YamlCfg.Default.OdooDashboardReportingPHPPort,
		now.AddDate(0, 0, -1).Format("02_JAN_2006"),
	)
	realURLyesterdayHistoryJOPlannedForTechniciansLink := findRealURL(testURLyesterdayHistoryJOPlannedForTechniciansLink)
	if realURLyesterdayHistoryJOPlannedForTechniciansLink != "" {
		yesterdayHistoryJOPlannedForTechniciansLink = realURLyesterdayHistoryJOPlannedForTechniciansLink
	} else {
		yesterdayHistoryJOPlannedForTechniciansLink = testURLyesterdayHistoryJOPlannedForTechniciansLink
	}

	oldTemplateEngProd := `%v:%v/task_schedule/ENGINEER_PRODUCTIVITY_OLD/log/%v/_Old_Template__%s_EngineerProductivityReport_%v.xlsx`
	oldTemplateEngProdAllTaskURL := fmt.Sprintf(oldTemplateEngProd,
		h.YamlCfg.Default.OdooDashboardReportingPHPServer,
		h.YamlCfg.Default.OdooDashboardReportingPHPPort,
		now.Format("2006-01-02"),
		"All_Task_Type",
		now.Format("02Jan2006"),
	)

	oldTemplateEngProdPMOnlyURL := fmt.Sprintf(oldTemplateEngProd,
		h.YamlCfg.Default.OdooDashboardReportingPHPServer,
		h.YamlCfg.Default.OdooDashboardReportingPHPPort,
		now.Format("2006-01-02"),
		"PM_Only",
		now.Format("02Jan2006"),
	)

	technicianLoginReportDirURL := fmt.Sprintf("%v:%v/webview_odoo/public/files/%v/",
		h.YamlCfg.Default.OdooDashboardReportingPHPServer,
		h.YamlCfg.Default.OdooDashboardReportingPHPPort,
		now.Format("2006-01-02"),
	)
	var linkForTechnicianLoginReport string
	latestFileURLTechLoginReport, err := getLatestFile(technicianLoginReportDirURL)
	if err != nil {
		linkForTechnicianLoginReport = err.Error()
	} else {
		linkForTechnicianLoginReport = latestFileURLTechLoginReport
	}

	reports := []Report{
		{
			ReportType: fmt.Sprintf("History JO Planned %v For Technicians", now.AddDate(0, 0, -1).Format("02 January 2006")),
			Links: []string{
				yesterdayHistoryJOPlannedForTechniciansLink,
			},
			CanDownload: make(map[string]bool),
		},
		{
			ReportType: fmt.Sprintf("Engineers Productivity Report %v", now.Format("02 JAN 2006")),
			Links: []string{
				fmt.Sprintf("%v:%v/task_schedule/ENGINEER_PRODUCTIVITY/log/%v/_%v_AllTicketType_Report.xlsx",
					h.YamlCfg.Default.OdooDashboardReportingPHPServer,
					h.YamlCfg.Default.OdooDashboardReportingPHPPort,
					now.Format("2006-01-02"),
					now.Format("02January2006"),
				),
				fmt.Sprintf("%v:%v/task_schedule/ENGINEER_PRODUCTIVITY/log/%v/_%v_PMOnly_Report.xlsx",
					h.YamlCfg.Default.OdooDashboardReportingPHPServer,
					h.YamlCfg.Default.OdooDashboardReportingPHPPort,
					now.Format("2006-01-02"),
					now.Format("02January2006"),
				),
			},
			CanDownload: make(map[string]bool),
		},
		{
			ReportType: fmt.Sprintf("[Old Template] Engineer Productivity %v", now.Format("02 JAN 2006")),
			Links: []string{
				oldTemplateEngProdAllTaskURL,
				oldTemplateEngProdPMOnlyURL,
			},
			CanDownload: make(map[string]bool),
		},
		{
			ReportType: fmt.Sprintf("Technician Login Report @%v", now.Format("02 JAN 2006")),
			Links: []string{
				linkForTechnicianLoginReport,
			},
			CanDownload: make(map[string]bool),
		},
		{
			ReportType: fmt.Sprintf("SLA Report @%v", time.Now().Format("02 Jan 2006")),
			Links: []string{
				fmt.Sprintf("%v:%v/report/file/sla_report/%v/(%v)SLAReport_Master.xlsx",
					h.YamlCfg.Default.WoDetailServer,
					h.YamlCfg.Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
				fmt.Sprintf("%v:%v/report/file/sla_report/%v/(%v)SLAReport_CM.xlsx",
					h.YamlCfg.Default.WoDetailServer,
					h.YamlCfg.Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
				fmt.Sprintf("%v:%v/report/file/sla_report/%v/(%v)SLAReport_PM.xlsx",
					h.YamlCfg.Default.WoDetailServer,
					h.YamlCfg.Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
				fmt.Sprintf("%v:%v/report/file/sla_report/%v/(%v)SLAReport_NonPM.xlsx",
					h.YamlCfg.Default.WoDetailServer,
					h.YamlCfg.Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
				fmt.Sprintf("%v:%v/report/file/sla_report/%v/(%v)SLAReport_SolvedPending.xlsx",
					h.YamlCfg.Default.WoDetailServer,
					h.YamlCfg.Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
			},
			CanDownload: make(map[string]bool),
		},
		{
			ReportType: fmt.Sprintf("Artajasa ATM Task Data Report @%v", time.Now().Format("02 Jan 2006")),
			Links: []string{
				fmt.Sprintf("%v:%v/report/file/odoo_atm_task_report/%v/(%v)ArtajasaATMReport_Master.xlsx",
					h.YamlCfg.Default.WoDetailServer,
					h.YamlCfg.Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
			},
			CanDownload: make(map[string]bool),
		},
		{
			ReportType: fmt.Sprintf("Engineers Productivity Report @%v", time.Now().Format("02 Jan 2006")),
			Links: []string{
				fmt.Sprintf("%v:%v/report/file/engineers_productivity/%v/(%v)EngineersProductivityReport_Master.xlsx",
					h.YamlCfg.Default.WoDetailServer,
					h.YamlCfg.Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
			},
			CanDownload: make(map[string]bool),
		},
	}

	// Check which links are downloadable
	for i := range reports {
		for _, link := range reports[i].Links {
			reports[i].CanDownload[link] = checkLinkAvailability(link)
		}
	}

	var sb strings.Builder
	sb.WriteString("Berikut informasi mengenai list report ke Mr. Oliver:\n")
	for _, report := range reports {
		sb.WriteString(fmt.Sprintf("\n📌 *%v*\n", report.ReportType))
		for _, link := range report.Links {
			if report.CanDownload[link] {
				sb.WriteString(fmt.Sprintf("✅ Downloadable: %s\n", fmt.Sprintf("`%s`", link)))
			} else {
				sb.WriteString(fmt.Sprintf("❌ Cannot Download: %s\n", fmt.Sprintf("`%s`", link)))
			}
		}
	}
	msgToSend := sb.String()

	h.sendWhatsAppMessage(jid, msgToSend)
	log.Printf("Scheduler %v successfully executed @%v", taskDoing, time.Now())
}
