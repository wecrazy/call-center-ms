package whatsapp

import (
	"context"
	"fmt"
	"log"
	"strings"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

// processMessage processes incoming messages
func (h *WhatsmeowHandler) processMessage(v *events.Message) {
	originalSenderJID := v.Info.Sender.String()
	if !strings.HasSuffix(originalSenderJID, "@s.whatsapp.net") {
		originalSenderJID = strings.Split(originalSenderJID, ":")[0] + "@s.whatsapp.net"
	}
	stanzaID := v.Info.ID

	var messageText string
	if v.Message.Conversation != nil {
		messageText = *v.Message.Conversation
	} else if v.Message.ExtendedTextMessage != nil {
		messageText = *v.Message.ExtendedTextMessage.Text
	} else {
		messageText = "[Non-text message]"
	}

	// // Handle Private Messages
	// SOON
	// if !v.Info.IsGroup {
	// 	log.Print("Private Message Received!")
	// 	log.Printf("Sender JID: %s\n", v.Info.Sender.String())
	// 	log.Printf("Sender Phone: +%s\n", v.Info.Sender.User)
	// 	log.Printf("Message: %s\n", messageText)
	// 	return
	// }

	// Handle Group Messages
	waGroup := h.YamlCfg.Whatsmeow.WaGroupRequest

	if v.Info.IsGroup && containsJID(waGroup, v.Info.Chat) {
		// Check if the message contains a document attachment
		if v.Message.DocumentMessage != nil {
			doc := v.Message.DocumentMessage

			if doc.FileName != nil {
				switch strings.ToLower(*doc.FileName) {
				case "unsolved_visit.xlsx":
					h.ProcessUnsolvedVisitData(v, stanzaID, originalSenderJID, doc)
				case "based_on_wo_number.xlsx":
					h.ProcessBasedOnWoNumber(v, stanzaID, originalSenderJID, doc)
				}
			}
		}

		if strings.ToLower(messageText) == "ping" {
			h.replyWithPong(v, stanzaID, originalSenderJID)
			return
		}

		if strings.ToLower(messageText) == "generate call center report" {
			sender := v.Info.Sender.String()
			number := strings.Split(sender, ":")[0] // Remove device-specific suffix
			number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

			if number == h.YamlCfg.Whatsmeow.WaSu ||
				number == h.YamlCfg.Whatsmeow.WaTetty ||
				number == h.YamlCfg.Whatsmeow.WaBuLina {
				h.GenerateReportCallCenter()
				return
			} else {
				var sb strings.Builder
				sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
				sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
				msgToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &msgToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed to send reply:", err)
					return
				}
				return
			}
		}

		if strings.ToLower(messageText) == "run scheduler get data sla h-2" {
			sender := v.Info.Sender.String()
			number := strings.Split(sender, ":")[0] // Remove device-specific suffix
			number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

			if number == h.YamlCfg.Whatsmeow.WaSu || number == h.YamlCfg.Whatsmeow.WaTetty {
				h.GetDataSLAHmin2forFU()
				return
			} else {
				var sb strings.Builder
				sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
				sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
				msgToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &msgToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed to send reply:", err)
					return
				}
				return
			}
		}

		if strings.ToLower(messageText) == "run scheduler get data ticket solved pending" {
			sender := v.Info.Sender.String()
			number := strings.Split(sender, ":")[0] // Remove device-specific suffix
			number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

			if number == h.YamlCfg.Whatsmeow.WaSu || number == h.YamlCfg.Whatsmeow.WaTetty {
				h.GetDataSolvedPendingforFU()
				return
			} else {
				var sb strings.Builder
				sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
				sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
				msgToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &msgToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed to send reply:", err)
					return
				}
				return
			}
		}

		// if strings.ToLower(messageText) == "run scheduler get data unsolved visit" {
		// 	sender := v.Info.Sender.String()
		// 	number := strings.Split(sender, ":")[0] // Remove device-specific suffix
		// 	number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

		// 	if number == h.YamlCfg.Whatsmeow.WaSu || number == h.YamlCfg.Whatsmeow.WaTetty {
		// 		h.GetDataUnsolvedTicketForFU()
		// 		return
		// 	} else {
		// 		var sb strings.Builder
		// 		sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
		// 		sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
		// 		msgToSend := sb.String()

		// 		quotedMsg := &waProto.ContextInfo{
		// 			StanzaID:      &stanzaID,
		// 			Participant:   &originalSenderJID,
		// 			QuotedMessage: v.Message,
		// 		}

		// 		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		// 			ExtendedTextMessage: &waProto.ExtendedTextMessage{
		// 				Text:        &msgToSend,
		// 				ContextInfo: quotedMsg,
		// 			},
		// 		})

		// 		if err != nil {
		// 			log.Println("[ERROR] Failed to send reply:", err)
		// 			return
		// 		}
		// 		return
		// 	}
		// }

		// if strings.ToLower(messageText) == "run scheduler get data based on wo number" {
		// 	sender := v.Info.Sender.String()
		// 	number := strings.Split(sender, ":")[0] // Remove device-specific suffix
		// 	number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

		// 	if number == h.YamlCfg.Whatsmeow.WaSu || number == h.YamlCfg.Whatsmeow.WaTetty {
		// 		h.GetDataBasedOnWONumber()
		// 		return
		// 	} else {
		// 		var sb strings.Builder
		// 		sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
		// 		sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
		// 		msgToSend := sb.String()

		// 		quotedMsg := &waProto.ContextInfo{
		// 			StanzaID:      &stanzaID,
		// 			Participant:   &originalSenderJID,
		// 			QuotedMessage: v.Message,
		// 		}

		// 		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		// 			ExtendedTextMessage: &waProto.ExtendedTextMessage{
		// 				Text:        &msgToSend,
		// 				ContextInfo: quotedMsg,
		// 			},
		// 		})

		// 		if err != nil {
		// 			log.Println("[ERROR] Failed to send reply:", err)
		// 			return
		// 		}
		// 		return
		// 	}
		// }

		if strings.ToLower(messageText) == "backup call center data follow up" {
			sender := v.Info.Sender.String()
			number := strings.Split(sender, ":")[0] // Remove device-specific suffix
			number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

			if number == h.YamlCfg.Whatsmeow.WaSu {
				h.DumpDataCCForFU()
				return
			} else {
				var sb strings.Builder
				sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
				sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
				msgToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &msgToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed to send reply:", err)
					return
				}
				return
			}
		}

		if strings.ToLower(messageText) == "sanitize call center data follow up" {
			sender := v.Info.Sender.String()
			number := strings.Split(sender, ":")[0] // Remove device-specific suffix
			number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

			if number == h.YamlCfg.Whatsmeow.WaSu || number == h.YamlCfg.Whatsmeow.WaTetty {
				h.SanitizeCCRCAllowed()
				return
			} else {
				var sb strings.Builder
				sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
				sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
				msgToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &msgToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed to send reply:", err)
					return
				}
				return
			}
		}

		if strings.ToLower(messageText) == "check mr. oliver report availability" {
			sender := v.Info.Sender.String()
			number := strings.Split(sender, ":")[0] // Remove device-specific suffix
			number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

			if number == h.YamlCfg.Whatsmeow.WaSu {
				h.CheckMrOliverReportIsExists()
				return
			} else {
				var sb strings.Builder
				sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
				sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
				msgToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &msgToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed to send reply:", err)
					return
				}
				return
			}
		}

		if strings.ToLower(messageText) == "run scheduler get data planned h+0" {
			sender := v.Info.Sender.String()
			number := strings.Split(sender, ":")[0] // Remove device-specific suffix
			number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

			if number == h.YamlCfg.Whatsmeow.WaSu || number == h.YamlCfg.Whatsmeow.WaTetty {
				h.GetDataPlannedHPlus0()
				return
			} else {
				var sb strings.Builder
				sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
				sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
				msgToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &msgToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed to send reply:", err)
					return
				}
				return
			}
		}

		if strings.ToLower(messageText) == "halo, ada berapa data follow up call center hari ini?" {
			sender := v.Info.Sender.String()
			number := strings.Split(sender, ":")[0] // Remove device-specific suffix
			number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

			if number == h.YamlCfg.Whatsmeow.WaSu ||
				number == h.YamlCfg.Whatsmeow.WaTetty ||
				number == h.YamlCfg.Whatsmeow.WaBuLina {
				h.GetTotalTodayDataFUCC()
				return
			} else {
				var sb strings.Builder
				sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
				sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
				msgToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &msgToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed to send reply:", err)
					return
				}
				return
			}
		}

		if strings.ToLower(messageText) == "halo" {
			h.replyWithHalo(v, stanzaID, originalSenderJID)
			return
		}

		if strings.ToLower(messageText) == "contoh template request" {
			h.replyWithTemplateRequest(v, stanzaID, originalSenderJID)
			return
		}

		/*************************************************************************/
		// Prefix msg text
		text := strings.ToLower(strings.TrimSpace(messageText))
		prefix := "apakah nomor "
		suffix := " adalah nomor valid ?"

		if strings.HasPrefix(text, prefix) && strings.HasSuffix(text, suffix) {
			sender := v.Info.Sender.String()
			number := strings.Split(sender, ":")[0] // Remove device-specific suffix
			number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

			if number == h.YamlCfg.Whatsmeow.WaSu {
				// Get what's between the prefix and suffix
				start := len(prefix)
				end := len(text) - len(suffix)

				if end > start {
					phoneNumber := strings.TrimSpace(text[start:end])
					isValidPhoneNumber := h.CheckValidWhatsappPhoneNumber(phoneNumber)
					var msgToSend string
					if !isValidPhoneNumber {
						msgToSend = fmt.Sprintf("*[ERROR]* Nomor _%v_ bukan nomor Whatsapp yang valid!", phoneNumber)
					} else {
						msgToSend = fmt.Sprintf("*[INFO]* Nomor _%v_ merupakan nomor valid yang terdaftar di Whatsapp", phoneNumber)
					}

					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &msgToSend,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] Failed to send reply:", err)
						return
					}
					return
				} else {
					fmt.Println("[ERROR] No phone number found between prefix and suffix.")
					return
				}
			} else {
				var sb strings.Builder
				sb.WriteString("⛔ *UNAUTHORIZED USER* ⛔\n")
				sb.WriteString("Anda tidak memiliki otoritas untuk menjalankan perintah ini ‼")
				msgToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &msgToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed to send reply:", err)
					return
				}
				return
			}
		}

		prefix = "wo number:"

		if strings.HasPrefix(text, prefix) {
			woValue := strings.TrimSpace(strings.TrimPrefix(text, prefix))
			h.ProcessSingleWoNumber(v, stanzaID, originalSenderJID, woValue)
		}
		/*************************************************************************/

		lines := strings.Split(messageText, "\n")

		if !strings.HasPrefix(lines[0], "[REQUEST ") {
			// fmt.Printf("[ERROR] Invalid format. Must start with '[REQUEST ...]', from request; %v", lines)
			return
		} else {
			h.replyRequestTemplate(v, stanzaID, originalSenderJID, lines)
			return
		}
	}
}
