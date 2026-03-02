package whatsapp

import (
	"context"
	"fmt"
	"log"
	"strings"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

// replyWithTemplateRequest replies with a template request message
func (h *WhatsmeowHandler) replyWithTemplateRequest(v *events.Message, stanzaID, originalSenderJID string) {
	var contohTemplateReqDapur strings.Builder
	contohTemplateReqDapur.WriteString("Berikut contoh template untuk melakukan request agar di _follow up_ oleh tim Call Center ‼\n")
	contohTemplateReqDapur.WriteString("Silahkan _copy_ template dibawah ini untuk melakukan request:\n\n")
	contohTemplateReqDapur.WriteString("[REQUEST *Follow Up EDC Yang Gagal Transaksi*]\n")
	contohTemplateReqDapur.WriteString("Jenis Kunjungan: *Withdrawal*\n")
	contohTemplateReqDapur.WriteString("Merchant: *Cideng*\n")
	contohTemplateReqDapur.WriteString("PIC: *Kepala Suku*\n")
	contohTemplateReqDapur.WriteString("No/Telp: *08555555*\n")
	contohTemplateReqDapur.WriteString("SN EDC: *12345*\n")
	contohTemplateReqDapur.WriteString("MID: *000222*\n")
	contohTemplateReqDapur.WriteString("TID: *222000*\n")
	contohTemplateReqDapur.WriteString("RC: *A00*\n")
	contohTemplateReqDapur.WriteString("Alamat Merchant: *Jalan ke cideng macet kalo balik lewat monas.*\n")
	contohTemplateReqDapur.WriteString("Order: *Re-Confirm*\n")
	contohTemplateReqDapur.WriteString("Request to CC: *Tolong konfirmasi posisi EDC dimana, apakah masih di lokasi merchant atau sudah pindah ke tempat lain*\n")
	contohTemplateReqDapur.WriteString("Plan Schedule: *28/02/2025*\n")
	templateReqDapur := contohTemplateReqDapur.String()

	quotedMsg := &waProto.ContextInfo{
		StanzaID:      &stanzaID,
		Participant:   &originalSenderJID,
		QuotedMessage: v.Message,
	}

	sentMsg, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        &templateReqDapur,
			ContextInfo: quotedMsg,
		},
	})

	if err != nil {
		log.Println("[ERROR] Failed to send template request dapur in group:", err)
		return
	} else {
		// log.Printf("[INFO] Successfully replied quoting %s message ID '%s' from user %s in group %s\n",
		// 	messageText,
		// 	stanzaID,
		// 	originalSenderJID,
		// 	v.Info.Chat.String())

		var fuMsg strings.Builder
		fuMsg.WriteString("⚠ *Pastikan Anda Mengisi Data Dengan Benar!* ⚠\n")
		fuMsg.WriteString("_Harap diperhatikan bagian yang ditandai dengan *BOLD*._\n\n")
		fuMsg.WriteString("📌 _Notes:_\n")
		fuMsg.WriteString("1️⃣ Pastikan nama kolom sesuai dengan yang ada di template!\n")
		fuMsg.WriteString("2️⃣ Setelah *REQUEST*, pastikan Anda memasukkan jenis request tersebut.\n")
		fuMsg.WriteString("3️⃣ *Jenis Kunjungan, No/Telp, SN EC & RC* tidak wajib diisi.\n")
		fuMsg.WriteString("4️⃣ *Order, Request to CC, MID & TID* wajib diisi!\n")
		fuMsg.WriteString(fmt.Sprintf("5️⃣ *No/Telp* merupakan nomor telepon dengan format Indonesia. Contoh: +62xx, 08xx, 8xx, 62xx, tidak boleh menggunakan 021xxx karena bukan nomor telepon yang terdaftar di _Whatsapp_. Dan pastikan nomor telepon > %d digit!\n", digitNoTelp))
		fuMsg.WriteString("6️⃣ *Jenis Kunjungan* yang dapat dimasukkan, di antaranya:\n")
		for _, jenis := range allowedJenisKunjungan {
			fuMsg.WriteString("     - " + jenis + "\n")
		}
		fuMsg.WriteString("\n_*Selain kunjungan dari contoh yang ada di atas akan ditolak!_\n\n")
		fuMsg.WriteString("7️⃣ *Order* yang dapat dimasukkan, di antaranya:\n")
		for _, wishOrder := range allowedOrderinWhatsapp {
			fuMsg.WriteString("     - " + wishOrder + "\n")
		}
		fuMsg.WriteString("\n_*Selain order dari contoh yang ada di atas akan ditolak!_\n\n")
		fuMsg.WriteString("8️⃣ *Request to CC* adalah arahan atau keperluan yang perlu disampaikan ke tim Call Center, contohnya: tolong konfirmasi keberadaan EDC.\n")
		fuMsg.WriteString("9️⃣ *Plan Schedule* adalah saran tanggal dari Tim kapan teknisinya bisa kunjungan. Tanggal harus dalam format *DD/MM/YYYY* atau 28/02/2025.\n")
		fuMsg.WriteString("🔟 *Reason Code* yang dapat dimasukkan, di antaranya:\n")
		for _, rc := range ReasonCodeAllowed {
			fuMsg.WriteString("     - " + rc + "\n")
		}
		fuMsg.WriteString("\n_*Selain reason code dari contoh yang ada di atas akan ditolak!_\n\n")
		fuMsg.WriteString("\n_Terima kasih atas perhatiannya._\n")
		fuMsg.WriteString(fmt.Sprintf("_~Regards, Call Center Team *%v*_", h.YamlCfg.Default.PT))

		followUpMessage := fuMsg.String()

		// Get the message ID of the sent template (so we can reply to it)
		templateMsgID := sentMsg.ID

		followUpQuotedMsg := &waProto.ContextInfo{
			StanzaID:    &templateMsgID,     // 🔥 Quoting the **sent template message**
			Participant: &originalSenderJID, // Your bot's JID (assuming you have it)
			QuotedMessage: &waProto.Message{ // Create a minimal quoted message
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text: &templateReqDapur,
				},
			},
		}

		// Send the follow-up message quoting the **template message**
		_, err = h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &followUpMessage,
				ContextInfo: followUpQuotedMsg, // Attach the quoted message
			},
		})

		if err != nil {
			log.Println("[ERROR] Failed to send follow-up reply for contoh template request:", err)
			return
		}
		// else {
		// 	log.Println("[INFO] Follow-up reply sent successfully to request dapur template sent!")
		// }

		return
	}
}
