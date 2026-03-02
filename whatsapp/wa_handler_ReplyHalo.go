package whatsapp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/TigorLazuardi/tanggal"
	"github.com/briandowns/openweathermap"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

// replyWithPong replies with "Halo" message
func (h *WhatsmeowHandler) replyWithHalo(v *events.Message, stanzaID, originalSenderJID string) {
	var sb strings.Builder

	sender := v.Info.Sender.String()
	number := strings.Split(sender, ":")[0] // Remove device-specific suffix
	number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

	// Customize reply based on the sender number
	if number == h.YamlCfg.Whatsmeow.WaSu {
		sb.WriteString("HALO!")
	} else if number == h.YamlCfg.Whatsmeow.WaBot {
		sb.WriteString("H A L O !!")
	} else {
		sb.WriteString("Halo!")
	}

	senderName := v.Info.PushName
	if senderName == "" {
		senderName = number
	} else {
		senderName = fmt.Sprintf("*%v*", senderName)
	}

	sb.WriteString(fmt.Sprintf(" %v\n", senderName))

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	tgl, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	formattedDate := tgl.Format(" ", []tanggal.Format{
		tanggal.NamaHariDenganKoma, // e.g., "Jumat,"
		tanggal.Hari,               // e.g., "07"
		tanggal.NamaBulan,          // e.g., "Maret"
		tanggal.Tahun,              // e.g., "2025"
	})

	// Format time in 12-hour format with AM/PM
	formattedTime := now.Format("03:04 PM")

	message := fmt.Sprintf("Sekarang pukul %s dan hari ini adalah %s. \n", formattedTime, formattedDate)
	sb.WriteString(message)

	weatherApiKey := h.YamlCfg.Whatsmeow.OpenWeatherMapAPIKey
	w, err := openweathermap.NewCurrent("C", "EN", weatherApiKey)
	if err != nil {
		log.Print(err)
		return
	}
	w2, err := openweathermap.NewCurrent("C", "EN", weatherApiKey)
	if err != nil {
		log.Print(err)
		return
	}
	w3, err := openweathermap.NewCurrent("C", "EN", weatherApiKey)
	if err != nil {
		log.Print(err)
		return
	}
	w4, err := openweathermap.NewCurrent("C", "EN", weatherApiKey)
	if err != nil {
		log.Print(err)
		return
	}

	w3.CurrentByName("Jakarta")

	locationRMOffice := &openweathermap.Coordinates{
		Latitude:  h.YamlCfg.Whatsmeow.LatitudeRM,
		Longitude: h.YamlCfg.Whatsmeow.LongitudeRM,
	}
	w.CurrentByCoordinates(locationRMOffice)

	locationCidengOffice := &openweathermap.Coordinates{
		Latitude:  h.YamlCfg.Whatsmeow.LatitudeCideng,
		Longitude: h.YamlCfg.Whatsmeow.LongitudeCideng,
	}
	w2.CurrentByCoordinates(locationCidengOffice)

	locationMetlandOffice := &openweathermap.Coordinates{
		Latitude:  h.YamlCfg.Whatsmeow.LatitudeMetland,
		Longitude: h.YamlCfg.Whatsmeow.LongitudeMetland,
	}
	w4.CurrentByCoordinates(locationMetlandOffice)

	translatedDesc := translateWeather(w.Weather[0].Description)
	translatedDesc2 := translateWeather(w2.Weather[0].Description)
	translatedDesc3 := translateWeather(w4.Weather[0].Description)

	emoji1 := getWeatherEmoji(w.Weather[0].Description)
	emoji2 := getWeatherEmoji(w2.Weather[0].Description)
	emoji3 := getWeatherEmoji(w4.Weather[0].Description)

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s Cuaca di Kantor Cideng: %s\n", emoji2, translatedDesc2))
	sb.WriteString(fmt.Sprintf("%s Cuaca di Kantor Rawamangun: %s\n", emoji1, translatedDesc))
	sb.WriteString(fmt.Sprintf("%s Cuaca di Kantor Metland: %s\n", emoji3, translatedDesc3))
	sb.WriteString("🌆 *Jakarta*\n")
	sb.WriteString(fmt.Sprintf("🌡️ Suhu: %.2f°C\n", w3.Main.Temp))
	sb.WriteString(fmt.Sprintf("💧 Kelembaban: %d%%\n", w3.Main.Humidity))
	sb.WriteString(fmt.Sprintf("💨 Kecepatan Angin: %.2f m/s\n", w3.Wind.Speed))
	sb.WriteString("\nTetap semangat dan jaga kesehatan! 💪")
	sb.WriteString("\n✨ _Semoga harimu menyenangkan!_ 😊")

	replyMessage := sb.String()

	quotedMsg := &waProto.ContextInfo{
		StanzaID:      &stanzaID,
		Participant:   &originalSenderJID,
		QuotedMessage: v.Message,
	}

	_, err = h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        &replyMessage,
			ContextInfo: quotedMsg,
		},
	})

	if err != nil {
		log.Println("[ERROR] Failed to send halo reply in group:", err)
		return
	}
}
