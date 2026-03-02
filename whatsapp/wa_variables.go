package whatsapp

import (
	"net/http"
	"sync"
)

var TriggerGetFeedbackFromFU = make(chan FeedbackTriggerData)
var TriggerUpdateDatainODOO = make(chan UpdatedODOODataTriggerItem)

var allowedJenisKunjungan = []string{
	"Installation",
	"Withdrawal",
	"Corrective Maintenance",
	"Preventive Maintenance",
	"Replacement",
	"Pindah Vendor",
	"Re-Initialization",
	"Roll Out",
}

var allowedOrderinWhatsapp = []string{
	"Re-Confirm",
	"Merchant Confirmation",
	"Merchant Confirmation & Technician Will Visit",
}

var (
	OdooSessionCookies          []*http.Cookie
	odooSessionMutex            sync.Mutex
	resetStatusIsOnCallingMutex sync.Mutex
)

// Map for translating weather descriptions
var weatherTranslations = map[string]string{
	"clear sky":                       "Langit cerah",
	"few clouds":                      "Sedikit berawan",
	"scattered clouds":                "Berawan tipis",
	"broken clouds":                   "Berawan tebal",
	"overcast clouds":                 "Mendung",
	"rain":                            "Hujan",
	"light rain":                      "Hujan rintik-rintik",
	"moderate rain":                   "Hujan sedang",
	"heavy intensity rain":            "Hujan lebat",
	"very heavy rain":                 "Hujan sangat lebat",
	"extreme rain":                    "Hujan ekstrem",
	"freezing rain":                   "Hujan beku",
	"light intensity shower rain":     "Gerimis ringan",
	"shower rain":                     "Hujan deras sesekali",
	"heavy intensity shower rain":     "Hujan deras",
	"ragged shower rain":              "Hujan deras tidak merata",
	"thunderstorm":                    "Badai petir",
	"thunderstorm with light rain":    "Badai petir dengan hujan ringan",
	"thunderstorm with rain":          "Badai petir dengan hujan",
	"thunderstorm with heavy rain":    "Badai petir dengan hujan lebat",
	"light thunderstorm":              "Badai petir ringan",
	"heavy thunderstorm":              "Badai petir kuat",
	"ragged thunderstorm":             "Badai petir tidak merata",
	"thunderstorm with drizzle":       "Badai petir dengan gerimis",
	"thunderstorm with heavy drizzle": "Badai petir dengan gerimis lebat",
	"snow":                            "Salju",
	"light snow":                      "Salju ringan",
	"heavy snow":                      "Salju lebat",
	"sleet":                           "Hujan es",
	"light shower sleet":              "Gerimis hujan es",
	"shower sleet":                    "Hujan es deras",
	"light rain and snow":             "Hujan rintik-rintik dan salju",
	"rain and snow":                   "Hujan dan salju",
	"light shower snow":               "Hujan salju ringan",
	"shower snow":                     "Hujan salju",
	"heavy shower snow":               "Hujan salju lebat",
	"mist":                            "Berkabut",
	"smoke":                           "Berasap",
	"haze":                            "Kabut asap",
	"sand/dust whirls":                "Pusaran pasir/debu",
	"fog":                             "Kabut",
	"sand":                            "Badai pasir",
	"dust":                            "Badai debu",
	"volcanic ash":                    "Abu vulkanik",
	"squalls":                         "Angin kencang",
	"tornado":                         "Tornado",
}

var digitNoTelp = 9
