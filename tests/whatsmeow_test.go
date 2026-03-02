package tests

// go test -v -timeout 60m ./tests/whatsmeow_test.go

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"github.com/stretchr/testify/assert"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	// waProto "go.mau.fi/whatsmeow/binary/proto"
)

var client *whatsmeow.Client

// func eventHandler(evt interface{}) {
// 	switch v := evt.(type) {
// 	case *events.Message:
// 		if !v.Info.IsFromMe {
// 			if v.Message.GetConversation() != "" {
// 				fmt.Println("PESAN DITERIMA:", v.Message.GetConversation())

// 				// Convert sender JID properly
// 				senderJID := v.Info.Sender

// 				_, err := client.SendMessage(context.Background(), senderJID, &waProto.Message{
// 					Conversation: proto.String("Pesan ini otomatis. Anda mengirim pesan: " + v.Message.GetConversation()),
// 				})
// 				if err != nil {
// 					fmt.Println("Failed to send auto-reply:", err)
// 				}
// 			}
// 		}
// 	}

// func getGroupInfoFromMyWhatsapp() {
// 	data, err := client.GetJoinedGroups()
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// // Step 2: Pretty-print group details
// 	// for _, group := range data {
// 	// 	fmt.Printf("\n📌 Group: %s (ID: %s)\n", group.Name, group.JID.String())
// 	// 	fmt.Printf("   🔹 Creator: %s \n", group.OwnerJID.User)

// 	// 	fmt.Println("   👥 Participants:")
// 	// 	for _, participant := range group.Participants {
// 	// 		role := "member"
// 	// 		if participant.IsAdmin {
// 	// 			role = "admin"
// 	// 		}
// 	// 		if participant.IsSuperAdmin {
// 	// 			role = "superadmin"
// 	// 		}
// 	// 		fmt.Printf("      - %s (%s)\n", participant.JID.String(), role)
// 	// 	}
// 	// }

// 	// Optional: JSON Pretty Print
// 	jsonData, _ := json.MarshalIndent(data, "", "  ")
// 	// fmt.Println("\nJSON Format:\n", string(jsonData))

// 	err = os.WriteFile("group_data.json", jsonData, 0644)
// 	if err != nil {
// 		fmt.Printf("Error writing to file: %v", err)
// 	}

// 	fmt.Println("✅ Data saved to group_data.json")
// }

func TestWhatsAppClient(t *testing.T) {
	client = startWhatsAppClient(t)
	assert.NotNil(t, client, "WhatsApp client should be initialized")

	// Listen for incoming messages
	// client.AddEventHandler(eventHandler)

	// getGroupInfoFromMyWhatsapp()

	// ######################################################################################################
	/*
		Send Message Server
		- To Group: g.us
		- To Contact: s.whatsapp.net
	*/
	// defMessage := "Halo, ini contoh teks message yang dikirim menggunakan Whatsmeow 😺 "
	// client.SendMessage(context.Background(),
	// 	// types.NewJID("120363201154381780", "g.us"),
	// 	types.NewJID("628979980882", "s.whatsapp.net"),
	// 	// types.NewJID("62899334756@s", "s.whatsapp.net"),
	// 	&waProto.Message{
	// 		Conversation: proto.String(defMessage),
	// 	})
	// ######################################################################################################

	targetGroupJID := types.NewJID("120363201154381780", "g.us")
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			// Extract correct participant JID (remove the device-specific suffix)
			originalSenderJID := strings.Split(v.Info.Sender.String(), ":")[0] + "@s.whatsapp.net"

			// Extract the original message ID (stanzaID)
			stanzaID := v.Info.ID

			if v.Info.IsGroup && v.Info.Chat == targetGroupJID {
				// Only process messages from the specific group
				fmt.Printf("Message from Group %s -> Sender: %s: %s\n",
					v.Info.Chat.String(), v.Info.Sender.String(), v.Message.GetConversation(),
				)

				var messageText string
				if v.Message.Conversation != nil {
					messageText = *v.Message.Conversation
				} else if v.Message.ExtendedTextMessage != nil {
					messageText = *v.Message.ExtendedTextMessage.Text
				} else {
					messageText = "[Non-text message]"
				}

				if messageText == "Ping" {
					replyMessage := "Pong"

					if v.Info.Sender.String() == "6285173207755:83@s.whatsapp.net" {
						replyMessage = "PONG!"
					}

					// Debugging logs
					fmt.Println("-------------------------------------------------------")
					fmt.Printf("[INFO] Received 'Ping' from: %s in group: %s\n", v.Info.Sender.String(), v.Info.Chat.String())
					fmt.Printf("[INFO] Original message ID (stanzaID): %s\n", stanzaID)

					// Construct the quoted message properly
					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,          // Message ID of the "Ping" message
						Participant:   &originalSenderJID, // Correct JID format
						QuotedMessage: v.Message,          // Include the original message
					}

					// Debug output to verify construction
					fmt.Printf("[INFO] Constructed quoted message context: %+v\n", quotedMsg)

					// Send the reply as a quoted message
					_, err := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &replyMessage, // "Pong"
							ContextInfo: quotedMsg,     // Attach the quoted message
						},
					})

					if err != nil {
						fmt.Println("[ERROR] Failed to send Pong reply in group:", err)
					} else {
						fmt.Printf("[INFO] Successfully replied with 'Pong' quoting message ID '%s' from user %s in group %s\n", stanzaID, originalSenderJID, v.Info.Chat.String())
					}
					fmt.Println("-------------------------------------------------------")
				}

				if messageText == "Contoh template request dapur" {
					var contohTemplateReqDapur strings.Builder
					contohTemplateReqDapur.WriteString("[REQUEST DAPUR]\n")
					contohTemplateReqDapur.WriteString("Jenis Kunjungan: *Withdrawal*\n")
					contohTemplateReqDapur.WriteString("Merchant: *Rawamangun*\n")
					contohTemplateReqDapur.WriteString("Alamat Merchant: Jalan *daksinapati timur dekat penjual nasi goreng, iya, ini adalah contoh kalimat di dalam kalimat yang mengandung contoh kalimat.*\n")
					contohTemplateReqDapur.WriteString("PIC: *Nugi Metland*\n")
					contohTemplateReqDapur.WriteString("No/Telp. PIC: *08555555*\n")
					contohTemplateReqDapur.WriteString("SN EDC: *12345*\n")
					contohTemplateReqDapur.WriteString("MID: *000222*\n")
					contohTemplateReqDapur.WriteString("TID: *222000*\n")
					templateReqDapur := contohTemplateReqDapur.String()

					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,          // Message ID of the "Ping" message
						Participant:   &originalSenderJID, // Correct JID format
						QuotedMessage: v.Message,          // Include the original message
					}

					// 🔹 STEP 1: Send the template response (quoting the original message)
					sentMsg, err := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &templateReqDapur,
							ContextInfo: quotedMsg, // Attach the quoted message from the original sender
						},
					})

					if err != nil {
						fmt.Println("[ERROR] Failed to send template request dapur in group:", err)
					} else {
						fmt.Printf("[INFO] Successfully replied quoting %s message ID '%s' from user %s in group %s\n",
							messageText,
							stanzaID,
							originalSenderJID,
							v.Info.Chat.String())

						followUpMessage := "⚠ Pastikan Anda Mengisi Data Dengan Benar! *dibagian yang di*BOLD* ⚠"

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
						_, err = client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
							ExtendedTextMessage: &waProto.ExtendedTextMessage{
								Text:        &followUpMessage,
								ContextInfo: followUpQuotedMsg, // Attach the quoted message
							},
						})

						if err != nil {
							fmt.Println("[ERROR] Failed to send follow-up reply for contoh template request dapur:", err)
						} else {
							fmt.Println("[INFO] Follow-up reply sent successfully to request dapur template sent!")
						}
					}

				}

				lines := strings.Split(messageText, "\n")

				if !strings.HasPrefix(lines[0], "[REQUEST ") {
					fmt.Println("[ERROR] Invalid format. Must start with '[REQUEST ...]'.")
					return
				}
				if v.Info.Sender.String() == "6285173207755:83@s.whatsapp.net" {
					// fmt.Print("[INFO] REQUEST from Admin not processed!")
					// return
				}
				headerParts := strings.Fields(lines[0])
				requestType := strings.Join(headerParts[1:], " ")
				requestType = strings.Replace(requestType, "]", "", -1)
				dataMap := make(map[string]string)
				dataMap["RequestType"] = requestType

				for _, line := range lines[1:] {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}

					// Handle key-value separation (using ":" or tab "\t")
					parts := strings.SplitN(line, ":", 2)

					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(parts[1])
						dataMap[key] = value // Store in map
					} else {
						fmt.Printf("[WARNING] Skipping invalid line: %v on request: %v", line, requestType)
					}
				}

				// ***** Map Keys
				// RequestType
				// Merchant
				// Alamat Merchant
				// PIC
				// No/Telp. PIC
				// SN EDC
				// MID
				// TID

				// here u send the data is being processing to the server
				// here u process the data

				// // Example: Accessing parsed data
				// fmt.Println("\n✅ Extracted Data:")
				// fmt.Println("RequestType:", dataMap["RequestType"])
				// fmt.Println("Merchant:", dataMap["Merchant"])
				// fmt.Println("Alamat Merchant:", dataMap["Alamat Merchant"])
				// fmt.Println("PIC:", dataMap["PIC"])
				// fmt.Println("No. Telp. PIC:", dataMap["No. Telp. PIC"])
				// fmt.Println("SN EDC:", dataMap["SN EDC"])
				// fmt.Println("MID:", dataMap["MID"])
				// fmt.Println("TID:", dataMap["TID"])

				// originalSenderJID := strings.Split(v.Info.Sender.String(), ":")[0] + "@s.whatsapp.net"
				// stanzaID := v.Info.ID

				// quotedMsg := &waProto.ContextInfo{
				// 	StanzaID:      &stanzaID,          // Message ID of the "Ping" message
				// 	Participant:   &originalSenderJID, // Correct JID format
				// 	QuotedMessage: v.Message,          // Include the original message
				// }

				// replyRequest := "Request anda adalah : " + messageText
				// _, err := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
				// 	ExtendedTextMessage: &waProto.ExtendedTextMessage{
				// 		Text:        &replyRequest,
				// 		ContextInfo: quotedMsg,
				// 	},
				// })

				// if err != nil {
				// 	fmt.Println("[ERROR] Failed to send request reply in group:", err)
				// } else {
				// 	fmt.Printf("[INFO] Successfully replied with 'Pong' quoting message ID '%s' from user %s in group %s\n", stanzaID, originalSenderJID, v.Info.Chat.String())

				// }

				// fmt.Println("This is a request:", messageText)

				// fmt.Println("[ERROR] Unknown command received:", messageText)
				// return

			}
			if !v.Info.IsGroup {
				senderJID := v.Info.Sender
				senderPhone := senderJID.User // Extract phone number
				// messageText := v.Message.GetConversation() // Get text message
				var messageText string
				if v.Message.Conversation != nil {
					messageText = *v.Message.Conversation
				} else if v.Message.ExtendedTextMessage != nil {
					messageText = *v.Message.ExtendedTextMessage.Text
				} else {
					messageText = "[Non-text message]"
				}

				fmt.Printf("Private Message Received!\n")
				fmt.Printf("Sender JID: %s\n", senderJID.String())
				fmt.Printf("Sender Phone: +%s\n", senderPhone)
				fmt.Printf("Message: %s\n", messageText)
			}
		}
	})

	select {}

	// -----------------------
	// ######################################################################################################
	// specificGroupJID := "120363201154381780@g.us"

	// client.AddEventHandler(eventHandler)

	// // Listen to new messages
	// client.AddEventHandler(func(event interface{}) {
	// 	switch v := evt.(type) {
	// 	case *events.Message:
	// 		if !v.Info.IsFromMe {
	// 			if v.Message.GetConversation() != "" {
	// 				fmt.Println("PESAN DITERIMA:", v.Message.GetConversation())

	// 				// Convert sender JID properly
	// 				senderJID := v.Info.Sender

	// 				_, err := client.SendMessage(context.Background(), senderJID, &waProto.Message{
	// 					Conversation: proto.String("Pesan ini otomatis. Anda mengirim pesan: " + v.Message.GetConversation()),
	// 				})
	// 				if err != nil {
	// 					fmt.Println("Failed to send auto-reply:", err)
	// 				}
	// 			}
	// 		}
	// 	}
	// })

	// // Start client and listen for events
	// client.Start()
	// ######################################################################################################

	// tesAPIGroupJID := types.NewJID("120363201154381780@g.us", "s.whatsapp.net")
	// _, err := client.SendMessage(context.Background(), tesAPIGroupJID, &waProto.Message{
	// 	Conversation: proto.String("Hello Group!"),
	// })
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// groupJID := "120363201154381780@g.us" // Replace with your group's JID
	// recipientJID := types.NewJID(groupJID, "s.whatsapp.net")

	// for i := 0; i < 3; i++ {
	// 	_, err := client.SendMessage(context.Background(), recipientJID, &waProto.Message{
	// 		Conversation: proto.String("Hello Group!"),
	// 	})
	// 	if err == nil {
	// 		fmt.Println("Message sent successfully!")
	// 		break
	// 	}
	// 	fmt.Println("Retrying to send message...")
	// 	time.Sleep(2 * time.Second) // Wait before retrying
	// }

	// fmt.Println("Message sent to the group:", groupJID)

	// groupStr := "120363201154381780@g.us"
	// groupJID := types.NewJID(groupStr, "s.whatsapp.net")
	// _, err := client.SendMessage(context.Background(), groupJID, &waProto.Message{
	// 	Conversation: proto.String("TEST aja"),
	// })
	// assert.Nil(t, err, "Failed to send test message")

	// ==========================================================================
	// recipientStr := "120363201154381780@g.us"
	// recipientJID := types.NewJID(recipientStr, "s.whatsapp.net")

	// message := "Test pakai whatsmeow!"
	// _, err := client.SendMessage(context.Background(), recipientJID, &waProto.Message{
	// 	Conversation: proto.String(message),
	// })
	// assert.Nil(t, err, "Failed to send test message")

	// // Test sending a message
	// recipientStr := "6287883507445"
	// recipientJID := types.NewJID(recipientStr, "s.whatsapp.net")

	// message := "Test pakai whatsmeow!"
	// _, err := client.SendMessage(context.Background(), recipientJID, &waProto.Message{
	// 	Conversation: proto.String(message),
	// })
	// assert.Nil(t, err, "Failed to send test message")

}

func startWhatsAppClient(t *testing.T) *whatsmeow.Client {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	container, err := sqlstore.New("sqlite3", "file:whatsmeow.db?_foreign_keys=on", dbLog)
	assert.Nil(t, err, "Failed to initialize database")

	deviceStore, err := container.GetFirstDevice()
	assert.Nil(t, err, "Failed to get WhatsApp device")

	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		assert.Nil(t, err, "Failed to connect WhatsApp client")

		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("Scan this QR code to login:")

				// Generate and display QR Code
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		err = client.Connect()
		assert.Nil(t, err, "Failed to reconnect WhatsApp client")
	}

	// Handle graceful shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		client.Disconnect()
	}()

	return client
}
