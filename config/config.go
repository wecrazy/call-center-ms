package config

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

var (
	config      YamlConfig
	configMutex sync.RWMutex
	configPath  string
)

var yamlFilePaths = []string{
	"/config/conf.yaml",
	"config/conf.yaml",
	"../config/conf.yaml",
	"/../config/conf.yaml",
	"../../config/conf.yaml",
	"/../../config/conf.yaml",
}

type YamlConfig struct {
	App struct {
		Name    string `yaml:"NAME"`
		Logo    string `yaml:"LOGO"`
		Port    string `yaml:"PORT"`
		Version string `yaml:"VERSION"`
	} `yaml:"APP"`

	Db struct {
		User                   string `yaml:"USER"`
		Password               string `yaml:"PASSWORD"`
		Host                   string `yaml:"HOST"`
		Port                   int    `yaml:"PORT"`
		Name                   string `yaml:"NAME"`
		MaxRetry               int    `yaml:"MAX_RETRY"`
		RetryDelay             string `yaml:"RETRY_DELAY"`
		TbUser                 string `yaml:"TB_USER"`
		TbDataMerchantHmin1    string `yaml:"TB_MERCHANT_HMIN1"`
		TbMerchantHmin1CallLog string `yaml:"TB_CALL_LOG_MERCHANT_HMIN1"`
		TbReqDapur             string `yaml:"TB_REQUEST_DAPUR"`
		TbReqDapurLog          string `yaml:"TB_REQUEST_DAPUR_LOG"`
		TbOdooRC               string `yaml:"TB_ODOO_REASON_CODE"`
		TbWaReq                string `yaml:"TB_WAREQUEST"`
		TbWaMention            string `yaml:"TB_WAMENTION"`
		TbUnsolvedTicket       string `yaml:"TB_UNSOLVED_TICKET"`
		TbBasedOnWoNumber      string `yaml:"TB_BASED_ON_WO_NUMBER"`
		TbCannotFollowUp       string `yaml:"TB_CANNOT_FOLLOW_UP"`
		// TbRole   string `yaml:"TB_ROLE"`
	} `yaml:"DB"`

	Default struct {
		PT                               string `yaml:"PT"`
		PTLogo                           string `yaml:"PT_LOGO"`
		UserName                         string `yaml:"USERNAME"`
		FirstName                        string `yaml:"FIRST_NAME"`
		LastName                         string `yaml:"LAST_NAME"`
		Email                            string `yaml:"EMAIL"`
		Phone                            string `yaml:"PHONE"`
		Password                         string `yaml:"PASSWORD"`
		HostServer                       string `yaml:"HOST_SERVER"`
		FilestoreServer                  string `yaml:"FILESTORE_SERVER"`
		WoDetailServer                   string `yaml:"WODETAIL_SERVER"`
		WoDetailPort                     int    `yaml:"WODETAIL_PORT"`
		OdooDashboardReportingPHPServer  string `yaml:"ODOO_DASHBOARD_REPORTING_SERVER"`
		OdooDashboardReportingPHPPort    int    `yaml:"ODOO_DASHBOARD_REPORTING_PORT"`
		OdooDashboardReportingGolangPort int    `yaml:"ODOO_DASHBOARD_GOLANG_REPORTING_PORT"`
		FilestoreMWServer                string `yaml:"FILE_STORE_MIDDLEWARE_SERVER"`
		FilestoreMWPort                  int    `yaml:"FILE_STORE_MIDDLEWARE_PORT"`
		FilestoreMWTAPort                int    `yaml:"FILE_STORE_MIDDLEWARE_TA_PORT"`
		FilestoreMWPhotosPort            int    `yaml:"FILE_STORE_MIDDLEWARE_PHOTOS_PORT"`
		TokenVMCCDs                      string `yaml:"TOKEN_VM_CALLDENTER_DS"`
		HeaderAuthFSMWKukuh              string `yaml:"HEADER_AUTH_FS_KUKUH"`
		MagickFullPath                   string `yaml:"MAGIC_FULLPATH"`
		LibreOfficeFullPath              string `yaml:"LIBREOFFICE_FULLPATH"`
		PdfToPngFullPath                 string `yaml:"PDFTOPNG_FULLPATH"`
		PdfInfoFullPath                  string `yaml:"PDFINFO_FULLPATH"`
		NssmFullPath                     string `yaml:"NSSM_FULLPATH"`
		FontTTFFullPath                  string `yaml:"FONTTTF_FULLPATH"`
		ExcelUploadedMaxSize             uint64 `yaml:"EXCEL_UPLOADED_MAX_SIZE"`
		// Role   string `yaml:"ROLE"`
	} `yaml:"DEFAULT"`

	ApiWA struct {
		Host string `yaml:"HOST"`
		Port int    `yaml:"PORT"`
	} `yaml:"API_WA"`

	ApiODOO struct {
		JSONRPC        string `yaml:"JSONRPC"`
		Login          string `yaml:"LOGIN"`
		Password       string `yaml:"PASSWORD"`
		Db             string `yaml:"DB"`
		UrlSession     string `yaml:"URL_SESSION"`
		UrlGetData     string `yaml:"URL_GETDATA"`
		UrlUpdateData  string `yaml:"URL_UPDATEDATA"`
		UrlCreateData  string `yaml:"URL_CREATEDATA"`
		MaxRetry       string `yaml:"MAX_RETRY"`
		RetryDelay     string `yaml:"RETRY_DELAY"`
		SessionTimeout string `yaml:"SESSION_TIMEOUT"`
		GetDataTimeout string `yaml:"GETDATA_TIMEOUT"`
		CompanyAllowed []int  `yaml:"COMPANY_ALLOWED"`
	} `yaml:"ODOO_API"`

	Email struct {
		Host       string   `yaml:"HOST"`
		Port       int      `yaml:"PORT"`
		Username   string   `yaml:"USERNAME"`
		Password   string   `yaml:"PASSWORD"`
		MaxRetry   int      `yaml:"MAX_RETRY"`
		RetryDelay int      `yaml:"RETRY_DELAY"`
		To         []string `yaml:"TO"`
		Cc         []string `yaml:"CC"`
	} `yaml:"EMAIL"`

	GoRoutine struct {
		CallWAPICMerchant int `yaml:"CALL_PIC_MERCHANT"`
	} `yaml:"GOROUTINE"`

	Scheduler struct {
		GetDataSLAHmin2             []string `yaml:"GET_DATA_SLA_H-2"`
		GetSolvedPendingTicket      []string `yaml:"GET_SOLVED_PENDING_TICKET"`
		GenerateCallCenterReport    []string `yaml:"GENERATE_CALL_CENTER_REPORT"`
		DumpCCDataForFU             []string `yaml:"DUMP_CC_DATA_FU"`
		SanitizeCCReasonCodeAllowed []string `yaml:"SANITIZE_CC_RC_ALLOWED"`
		CheckMrOliverReport         []string `yaml:"CHECK_MR_OLIVER_REPORT"`
		GetDataPlannedHPlus0        []string `yaml:"GET_DATA_PLANNED_H_PLUS_0"`
		ResetStatusIsOncalling      []string `yaml:"RESET_STATUS_IS_ON_CALLING"`
	} `yaml:"SCHEDULER"`

	Whatsmeow struct {
		SqlDriver                      string   `yaml:"SQL_DRIVER"`
		SqlSource                      string   `yaml:"SQL_SOURCE"`
		WaGroup                        string   `yaml:"WA_GROUP"`
		WaSu                           string   `yaml:"WA_SU"`
		WaBot                          string   `yaml:"WA_BOT"`
		WaSupport                      string   `yaml:"WA_SUPPORT"`
		WaTetty                        string   `yaml:"WA_TETTY"`
		WaBuLina                       string   `yaml:"WA_BU_LINA"`
		WaGroupRequest                 []string `yaml:"WAG"`
		GroupTestJID                   string   `yaml:"GROUP_TEST_JID"`
		GroupCCJID                     string   `yaml:"GROUP_CC_JID"`
		GroupTAJID                     string   `yaml:"GROUP_TA_JID"`
		GroupKoordinasiJID             string   `yaml:"GROUP_KOORDINASI"`
		GroupInventoryOprsRMMetlandJID string   `yaml:"GROUP_INVENTORY_OPRS_RM_METLAND"`
		OpenWeatherMapAPIKey           string   `yaml:"OPEN_WEATHER_MAP_API"`
		LatitudeRM                     float64  `yaml:"LATITUDE_RAWAMANGUN"`
		LongitudeRM                    float64  `yaml:"LONGITUDE_RAWAMANGUN"`
		LatitudeCideng                 float64  `yaml:"LATITUDE_CIDENG"`
		LongitudeCideng                float64  `yaml:"LONGITUDE_CIDENG"`
		LatitudeMetland                float64  `yaml:"LATITUDE_METLAND"`
		LongitudeMetland               float64  `yaml:"LONGITUDE_METLAND"`
	} `yaml:"WHATSMEOW"`
}

func LoadConfig() error {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}

	// // Print to console
	fmt.Println("Current Working Directory:", dir)
	for _, path := range yamlFilePaths {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Yaml path found in: %v\n", path)
			configPath = path
			break
		}
	}
	if configPath == "" {
		return fmt.Errorf("no valid config file found from paths: %v", yamlFilePaths)
	}

	file, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var newConfig YamlConfig
	if err := yaml.Unmarshal(file, &newConfig); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	configMutex.Lock()
	config = newConfig
	configMutex.Unlock()

	return nil
}

func WatchConfig() {
	if configPath == "" {
		log.Println("no valid config file found. Skipping watcher.")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("failed to initialize config watcher:%v", err)
	}
	defer watcher.Close()

	err = watcher.Add(configPath)
	if err != nil {
		log.Printf("failed to watch config file:%v", err)
	}

	fmt.Printf("watching for yaml config changes: %v\n", configPath)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op == fsnotify.Write {
				fmt.Println("config file updated. Reloading...")
				if err := LoadConfig(); err != nil {
					log.Printf("failed to reload config:%v", err)
				} else {
					fmt.Println("config reloaded successfully.")
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("config watcher error: %v", err)
		}
	}
}

func GetConfig() YamlConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config
}
