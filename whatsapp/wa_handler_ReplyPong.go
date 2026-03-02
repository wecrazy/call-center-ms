package whatsapp

import (
	"context"
	"log"
	"strings"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

// replyWithPong replies with "Pong" message
func (h *WhatsmeowHandler) replyWithPong(v *events.Message, stanzaID, originalSenderJID string) {
	replyMessage := "Pong"
	sender := v.Info.Sender.String()
	number := strings.Split(sender, ":")[0] // Remove device-specific suffix
	number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

	if number == h.YamlCfg.Whatsmeow.WaSu {
		replyMessage = "PONG!"
	} else if number == h.YamlCfg.Whatsmeow.WaBot {
		replyMessage = "P O N G !!"
	}

	quotedMsg := &waProto.ContextInfo{
		StanzaID:      &stanzaID,
		Participant:   &originalSenderJID,
		QuotedMessage: v.Message,
	}

	_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        &replyMessage,
			ContextInfo: quotedMsg,
		},
	})

	if err != nil {
		log.Println("[ERROR] Failed to send Pong reply in group:", err)
		return
	}
	// else {
	// log.Printf("[INFO] Successfully replied with 'Pong' quoting message ID '%s' from user %s in group %s\n",
	// 	stanzaID, originalSenderJID, v.Info.Chat.String())
	// }

	// return
}
