package whatsapp

import (
	"call_center_app/models"
	"call_center_app/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

// replyRequestTemplate replies with a request template processed message
func (h *WhatsmeowHandler) replyRequestTemplate(v *events.Message, stanzaID, originalSenderJID string, lines []string) {
	if len(lines) < 13 {
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		textToSend := "Data request invalid! Mohon lengkapi data request sesuai template yang tersedia. Untuk contoh template, Anda bisa ketik _Contoh Template Request_"

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &textToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Println("[ERROR] Failed format data request:", err)
		}

		return
	}

	headerParts := strings.Fields(lines[0])
	requestType := strings.Join(headerParts[1:], " ")
	requestType = strings.Replace(requestType, "]", "", -1)
	dataMap := make(map[string]string)
	dataMap["RequestType"] = requestType

	// Make map data for request template
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
			value = strings.ReplaceAll(value, "*", "")
			dataMap[key] = value // Store in map
		} else {
			log.Printf("[WARNING] Skipping invalid line: %v on request: %v", line, requestType)
		}
	}

	// Check if all fields are empty
	allEmpty := true
	keys := []string{
		// "RequestType",
		"Jenis Kunjungan",
		"Merchant",
		"PIC",
		"No/Telp",
		"SN EDC",
		"MID",
		"TID",
		"RC",
		"Alamat Merchant",
		"Order",
		"Request to CC",
		"Plan Schedule",
	}

	for _, key := range keys {
		if dataMap[key] != "" { // If any value is NOT empty, set allEmpty to false
			allEmpty = false
			break
		}
	}

	if allEmpty {
		// log.Println("[WARNING] All fields are empty!")
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		textToSend := "Data yang Anda request kosong! Tolong dilengkapi!"

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &textToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Println("[ERROR] Failed format data request coz empty:", err)
		}

		return
	} else {
		// ───────────────────────────────
		// ✅ Check Data Validity
		// ───────────────────────────────

		// 🔹 Order Validity
		if dataMap["Order"] != "" {
			dataValid := false
			for _, orderWish := range allowedOrderinWhatsapp {
				if strings.EqualFold(orderWish, dataMap["Order"]) {
					dataValid = true
					break
				}
			}

			if !dataValid {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("⚠ Maaf, jenis order: *%v* yang Anda masukkan tidak sesuai dengan yang ada di list template request. Silahkan masukkan sesuai yang ada di template!\n", dataMap["Order"]))
				sb.WriteString("\n*Order* yang diperbolehkan:\n")
				for _, order := range allowedOrderinWhatsapp {
					sb.WriteString("- " + order + "\n")
				}
				textToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &textToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed format data request coz invalid order:", err)
				}

				return
			}
		} else {
			var sb strings.Builder
			sb.WriteString("⚠ Maaf, *Order* wajib diisi, tidak boleh kosong!\n")
			sb.WriteString("\nOrder yang diperbolehkan:\n")
			for _, item := range allowedOrderinWhatsapp {
				sb.WriteString("- " + item + "\n")
			}
			errMsg := sb.String()

			quotedMsg := &waProto.ContextInfo{
				StanzaID:      &stanzaID,
				Participant:   &originalSenderJID,
				QuotedMessage: v.Message,
			}

			_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text:        &errMsg,
					ContextInfo: quotedMsg,
				},
			})

			if err != nil {
				log.Println("[ERROR] Failed while get valid order in db:", err)
			}

			return
		}

		// 🔹 Jenis Kunjungan Validity
		if dataMap["Jenis Kunjungan"] != "" {
			valid := false
			for _, jenis := range allowedJenisKunjungan {
				if strings.EqualFold(jenis, dataMap["Jenis Kunjungan"]) { // Case-insensitive check
					valid = true
					break
				}
			}

			if !valid {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("⚠ Maaf, jenis kunjungan: *%v* yang Anda masukkan tidak sesuai dengan yang ada di list template request. Silahkan masukkan sesuai yang ada di template!\n", dataMap["Jenis Kunjungan"]))
				sb.WriteString("\nJenis kunjungan yang diperbolehkan:\n")
				for _, jenis := range allowedJenisKunjungan {
					sb.WriteString("- " + jenis + "\n")
				}
				textToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &textToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed format data request coz invalid jenis kunjungan:", err)
				}

				return
			}
		}

		// 🔹 RC (Response Code) Validity
		if dataMap["RC"] != "" {
			if isValidRC(dataMap["RC"], ReasonCodeAllowed) {
				// if len(dataMap["RC"]) > 3 {
				if len(dataMap["RC"]) != 3 {
					var dataRC models.OdooReasonCode

					if err := h.Database.Table(h.YamlCfg.Db.TbOdooRC).
						First(&dataRC, "x_name LIKE ?", "%"+dataMap["RC"]+"%").
						Error; err != nil {

						var sb strings.Builder
						sb.WriteString(fmt.Sprintf("Maaf Reason Code: *%v* yang Anda masukkan tidak ditemukan di database!\n", dataMap["RC"]))
						sb.WriteString(fmt.Sprintf("⚠ Detail error: _%v_\n", err))
						sb.WriteString("\nReason Code yang diperbolehkan:\n")
						for _, rc := range ReasonCodeAllowed {
							sb.WriteString("- " + rc + "\n")
						}
						errMsg := sb.String()

						quotedMsg := &waProto.ContextInfo{
							StanzaID:      &stanzaID,
							Participant:   &originalSenderJID,
							QuotedMessage: v.Message,
						}

						_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
							ExtendedTextMessage: &waProto.ExtendedTextMessage{
								Text:        &errMsg,
								ContextInfo: quotedMsg,
							},
						})

						if err != nil {
							log.Println("[ERROR] Failed while get valid rc in db:", err)
						}

						return
					}

					dataMap["RC"] = dataRC.ReasonCode
				}
			} else {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("⚠ Maaf, Reason Code: *%v* yang Anda masukkan tidak sesuai dengan yang ada di template!\n", dataMap["RC"]))
				sb.WriteString("\nReason Code yang diperbolehkan:\n")
				for _, rc := range ReasonCodeAllowed {
					sb.WriteString("- " + rc + "\n")
				}
				errMsg := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &errMsg,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] invalid template RC:", err)
				}

				return
			}
		}

		// 🔹 No/Telp (Phone Number) Validity
		if dataMap["No/Telp"] != "" {
			if len(dataMap["No/Telp"]) > digitNoTelp {
				validPhoneNumber, err := utils.SanitizePhoneNumber(dataMap["No/Telp"])
				if err != nil {
					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("Maaf, nomor telepon: *%v* yang Anda masukkan tidak valid!\n", dataMap["No/Telp"]))
					sb.WriteString(fmt.Sprintf("⚠ Error: _%v_\n", err))
					errMsg := sb.String()

					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &errMsg,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] invalid phone number:", err)
					}

					return
				} else {
					dataMap["No/Telp"] = validPhoneNumber
				}

				// Check phone number if its registered on Whatsapp
				result, err := h.Client.IsOnWhatsApp([]string{dataMap["No/Telp"] + "@s.whatsapp.net"})
				if err != nil {
					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("Maaf, nomor telepon: *%v* yang Anda masukkan mengalami kesalahan saat proses pengecekan valid Whatsapp!\n", dataMap["No/Telp"]))
					sb.WriteString(fmt.Sprintf("⚠ Error: _%v_\n", err))
					errMsg := sb.String()

					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &errMsg,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] invalid check wa phone number:", err)
					}

					return
				} else {
					if len(result) > 0 {
						contact := result[0]
						// if contact.IsIn == false {
						if !contact.IsIn {
							var sb strings.Builder
							sb.WriteString(fmt.Sprintf("⚠ Maaf, nomor telepon: *%v* yang Anda masukkan belum terdaftar di Whatsapp! Coba masukkan nomor yang valid dan terdaftar di Whatsapp.\n", "0"+dataMap["No/Telp"]))
							errMsg := sb.String()

							quotedMsg := &waProto.ContextInfo{
								StanzaID:      &stanzaID,
								Participant:   &originalSenderJID,
								QuotedMessage: v.Message,
							}

							_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
								ExtendedTextMessage: &waProto.ExtendedTextMessage{
									Text:        &errMsg,
									ContextInfo: quotedMsg,
								},
							})

							if err != nil {
								log.Println("[ERROR] invalid check wa phone number:", err)
							}

							return
						}
					} else {
						var sb strings.Builder
						sb.WriteString(fmt.Sprintf("Maaf, nomor telepon: *%v* yang Anda masukkan mengalami kesalahan saat proses pengecekan valid Whatsapp!\n", dataMap["No/Telp"]))
						sb.WriteString("⚠ Error: _No result returned from WhatsApp check._\n")
						errMsg := sb.String()

						quotedMsg := &waProto.ContextInfo{
							StanzaID:      &stanzaID,
							Participant:   &originalSenderJID,
							QuotedMessage: v.Message,
						}

						_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
							ExtendedTextMessage: &waProto.ExtendedTextMessage{
								Text:        &errMsg,
								ContextInfo: quotedMsg,
							},
						})

						if err != nil {
							log.Println("[ERROR] invalid check wa phone number:", err)
						}

						return
					}
				}

			} else {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Maaf, nomor telepon: *%v* yang Anda masukkan tidak valid!\n", dataMap["No/Telp"]))
				sb.WriteString(fmt.Sprintf("⚠ Error: _Nomor telepon harus lebih dari %d digit!_\n", digitNoTelp))
				errMsg := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &errMsg,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] invalid phone number:", err)
				}

				return
			}
		}

		// 🔹 Plan Schedule Validity
		var dateTimeFormatPlanSchedule *time.Time
		value, exists := dataMap["Plan Schedule"]
		if exists && value != "" {
			_, err := time.Parse(layoutPlanSchedule, value)
			if err == nil {
				parsedTime, err := time.Parse(layoutPlanSchedule, value)
				if err == nil {
					dateTimeFormatPlanSchedule = &parsedTime
				} else {
					dateTimeFormatPlanSchedule = nil
				}
			} else {
				var sb strings.Builder
				sb.WriteString("⚠ Maaf, format tanggal *Plan Schedule* yang Anda masukkan tidak sesuai dengan yang ada di list template request. Silahkan masukkan sesuai yang ada di template!\n")
				sb.WriteString(fmt.Sprintf("\nFormat *Plan Schedule* yang diperbolehkan: %v (DD/MM/YYYY), contoh: 28/02/2025 ➡ ini mengindikasikan bahwa tanggal *28* bulan *February* tahun *2025*.", layoutPlanSchedule))
				textToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &textToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed format data request coz invalid date format plan schedule:", err)
				}

				return
			}
		}

		// 🔹 Request to CC (Call Center) Validity
		if dataMap["Request to CC"] == "" {
			var sb strings.Builder
			sb.WriteString("⚠ Maaf, *Request to CC* wajib diisi! Tidak boleh dikosongkan.\n")
			textToSend := sb.String()

			quotedMsg := &waProto.ContextInfo{
				StanzaID:      &stanzaID,
				Participant:   &originalSenderJID,
				QuotedMessage: v.Message,
			}

			_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text:        &textToSend,
					ContextInfo: quotedMsg,
				},
			})

			if err != nil {
				log.Println("[ERROR] Failed format data request to cc coz invalid:", err)
			}

			return
		} else {
			if len(dataMap["Request to CC"]) > 300 {
				var sb strings.Builder
				sb.WriteString("⚠ Maaf, *Request to CC* tidak boleh melebihi 300 karakter.\n")
				textToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &textToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed format data request to cc coz invalid len:", err)
				}

				return
			}
		}

		/* if needed */
		// 🔹 Merchant Validity
		// 🔹 PIC (Person in Charge) Validity
		// 🔹 SN EDC (Serial Number EDC) Validity
		// 🔹 Alamat Merchant (Merchant Address) Validity

		// 🔹 MID (Merchant ID) & TID (Terminal ID) Validity
		if dataMap["MID"] == "" || dataMap["TID"] == "" {
			var sb strings.Builder
			sb.WriteString("⚠ Maaf, data request Anda tidak dapat diproses, karena *MID & TID* wajib diisi! Tidak boleh dikosongkan.\n")
			errMsg := sb.String()

			quotedMsg := &waProto.ContextInfo{
				StanzaID:      &stanzaID,
				Participant:   &originalSenderJID,
				QuotedMessage: v.Message,
			}

			_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text:        &errMsg,
					ContextInfo: quotedMsg,
				},
			})

			if err != nil {
				log.Println("[ERROR] Failed MID & TID validation:", err)
			}

			return
		} else {
			switch strings.ToUpper(dataMap["Order"]) {
			case "RE-CONFIRM":
				// check mid tid jenis kunjungan pic and phone number keisi!!
				if dataMap["Jenis Kunjungan"] == "" ||
					dataMap["PIC"] == "" ||
					dataMap["Merchant"] == "" ||
					dataMap["No/Telp"] == "" {

					var sb strings.Builder
					sb.WriteString("⚠ Maaf, pastikan *Jenis Kunjungan, Merchant, PIC & No/Telp* terisi! Tidak boleh dikosongkan.\n")
					textToSend := sb.String()

					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &textToSend,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] Failed format data request coz invalid order:", err)
					}

					return
				}

				newDataforFUbyCC := models.WaRequest{
					Counter:             0,
					RequestType:         dataMap["RequestType"],
					MerchantName:        dataMap["Merchant"],
					PicMerchant:         dataMap["PIC"],
					PicPhone:            dataMap["No/Telp"],
					MerchantAddress:     dataMap["Alamat Merchant"],
					Mid:                 dataMap["MID"],
					Tid:                 dataMap["TID"],
					IsOnCalling:         false,
					IsDone:              false,
					IsOnCallingDatetime: nil,
					IsDoneDatetime:      nil,
					OrderWish:           dataMap["Order"],
					TaskType:            dataMap["Jenis Kunjungan"],
					UpdatedToOdoo:       false,
					RequestToCC:         dataMap["Request to CC"],
					SnEdc:               dataMap["SN EDC"],
					TargetScheduleDate:  dateTimeFormatPlanSchedule,
					ReasonCodeName:      dataMap["RC"],
					LastUpdateBy:        "System",
					IsFinal:             false,
					/* Whatsapp Info */
					GroupWaJid:        v.Info.Chat.String(),
					StanzaId:          stanzaID,
					OriginalSenderJid: originalSenderJID,
				}

				if err := h.Database.Table(h.YamlCfg.Db.TbWaReq).Create(&newDataforFUbyCC).Error; err != nil {
					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("Maaf, request _%v_ untuk Follow Up ke *%v* [%v] tidak dapat difollow up oleh tim Call Center, karena terdapat kesalahan di system.\n",
						dataMap["RequestType"],
						dataMap["PIC"],
						dataMap["Merchant"],
					))
					sb.WriteString(fmt.Sprintf("⚠ Detail error: _%v_\n", err))
					errMsg := sb.String()

					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &errMsg,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] Failed while try to save re-config new data in db:", err)
					}
					return
				}

				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("📌 *[INFO]* New ID: %d\n Request *%v* ke PIC: _%v_ [%v], selanjutnya akan di-follow up oleh tim *Call Center*.",
					newDataforFUbyCC.ID,
					dataMap["RequestType"],
					dataMap["PIC"],
					dataMap["Merchant"],
				))
				textToSend := sb.String()

				quotedMsg := &waProto.ContextInfo{
					StanzaID:      &stanzaID,
					Participant:   &originalSenderJID,
					QuotedMessage: v.Message,
				}

				_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text:        &textToSend,
						ContextInfo: quotedMsg,
					},
				})

				if err != nil {
					log.Println("[ERROR] Failed to fu req re-confirm by cc:", err)
				}

				return
			default:
				// Construct payload to get data in ODOO
				odooModel := "project.task"
				odooOrder := "id desc"
				companyAllowed := h.YamlCfg.ApiODOO.CompanyAllowed
				stageExcluded := []string{
					// "Done",
					// "Verified",
					"To Do",
					"Cancel",
				}
				odooFields := []string{
					"id",
					"x_merchant",
					"x_pic_merchant",
					"x_pic_phone",
					"partner_street",
					"description",
					"x_sla_deadline",
					"create_date",
					"x_task_type",
					"company_id",
					"stage_id",
					"helpdesk_ticket_id",
					"x_cimb_master_tid",
					"x_cimb_master_mid",
					"x_source",
					"x_message_call",
					"x_no_task",
					"x_status_merchant",
					"x_studio_edc",
					"x_product",
					"x_wo_remark",
					"x_latitude",
					"x_longitude",
					"technician_id",
					"x_received_datetime_spk",
					"planned_date_begin",
					"x_reason_code_id",
					"timesheet_timer_last_stop",
				}

				domain := []interface{}{
					[]interface{}{"active", "=", true},
					[]interface{}{"company_id", "=", companyAllowed},
					[]interface{}{"stage_id", "!=", stageExcluded},
					// []interface{}{"timesheet_timer_last_stop", "=", false},
				}

				fieldMapping := map[string]string{
					"Merchant":        "x_merchant",
					"RC":              "x_reason_code_id",
					"TID":             "x_cimb_master_tid",
					"MID":             "x_cimb_master_mid",
					"Jenis Kunjungan": "x_task_type",
					"SN EDC":          "x_studio_edc",
				}

				// Iterate over dataMap and append conditions if not empty
				for key, odooField := range fieldMapping {
					if value, exists := dataMap[key]; exists && strings.TrimSpace(value) != "" {
						domain = append(domain, []interface{}{odooField, "like", value})
					}
				}

				odooParams := map[string]interface{}{
					"domain": domain,
					"model":  odooModel,
					"fields": odooFields,
					"order":  odooOrder,
				}

				payload := map[string]interface{}{
					"jsonrpc": h.YamlCfg.ApiODOO.JSONRPC,
					"params":  odooParams,
				}

				payloadBytes, err := json.Marshal(payload)
				if err != nil {
					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("Maaf, request *%v* Anda tidak dapat diproses karena mengalami kendala.\n", dataMap["RequestType"]))
					sb.WriteString(fmt.Sprintf("\n⚠ Error: _failed to marshal JSON payload, %v_", err))
					textToSend := sb.String()

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &textToSend,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] Failed marshal json payload result:", err)
					}

					return
				}

				result, err := getODOOData(h.YamlCfg, string(payloadBytes))
				if err != nil {
					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("Maaf, request *%v* Anda tidak dapat diproses karena mengalami kendala.\n", dataMap["RequestType"]))
					sb.WriteString(fmt.Sprintf("\n⚠ Error: _%v_", err))
					textToSend := sb.String()

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &textToSend,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] Failed format odoo data result:", err)
					}

					return
				}

				resultArray, ok := result.([]interface{})
				if !ok {
					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("Maaf, request *%v* Anda tidak dapat diproses karena mengalami kendala.\n", dataMap["RequestType"]))
					sb.WriteString("\n⚠ ")

					textToSend := sb.String()

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &textToSend,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] Failed format odoo data result array :", err)
					}

					return
				}

				var woFUbyCC []string
				var woExisting []string
				var gotError []string

				for _, record := range resultArray {
					recordMap, ok := record.(map[string]interface{})
					if !ok {
						gotError = append(gotError, "Invalid record format in resultArray")
						continue
					}

					var odooData OdooTaskDataRequestItem
					jsonData, err := json.Marshal(recordMap)
					if err != nil {
						gotError = append(gotError, fmt.Sprintf("Failed to marshal recordMap: %v", err))
						continue
					}

					err = json.Unmarshal(jsonData, &odooData)
					if err != nil {
						gotError = append(gotError, fmt.Sprintf("Failed to unmarshal into odooData struct: %v", err))
						continue
					}

					var validMerchantAddress string
					if addr, ok := dataMap["Alamat Merchant"]; ok && addr != "" {
						validMerchantAddress = addr
					} else if odooData.MerchantAddress.String != "" {
						validMerchantAddress = odooData.MerchantAddress.String
					} else {
						validMerchantAddress = "" // Default to empty string
					}

					var validPIC string
					if pic, ok := dataMap["PIC"]; ok && pic != "" {
						validPIC = pic
					} else if odooData.PicMerchant.String != "" {
						validPIC = odooData.PicMerchant.String
					} else {
						validPIC = "" // Default to empty string
					}

					var validPicPhoneNumber string
					if dataMap["No/Telp"] != "" {
						validPicPhoneNumber = dataMap["No/Telp"]
					} else {
						checkValidOdooPhoneNumber, err := utils.SanitizePhoneNumber(odooData.PicPhone.String)
						if err != nil {
							gotError = append(gotError, fmt.Sprintf("Cannot processing WO Number: %v, cause not have a valid phone number!", odooData.WoNumber))
							continue
						}
						validPicPhoneNumber = checkValidOdooPhoneNumber
					}

					companyId, companyName, err := parseJSONIDDataCombined(odooData.CompanyId)
					if err != nil {
						gotError = append(gotError, fmt.Sprintf("Cannot processing WO Number: %v, cause not have a valid company ID!", odooData.WoNumber))
						continue
					}

					stageId, stageName, err := parseJSONIDDataCombined(odooData.StageId)
					if err != nil {
						gotError = append(gotError, fmt.Sprintf("Cannot processing WO Number: %v, cause not have a valid stage ID!", odooData.WoNumber))
						continue
					}

					var cleanedTicketNumber string
					ticketID, ticketNumber, err := parseJSONIDDataCombined(odooData.HelpdeskTicketId)
					if err != nil {
						gotError = append(gotError, fmt.Sprintf("Cannot processing WO Number: %v, cause not have a valid ticket ID!", odooData.WoNumber))
						continue
					} else {
						re := regexp.MustCompile(`\s*\(.*?\)`)
						cleanedTicketNumber = re.ReplaceAllString(ticketNumber, "")
					}

					snEdcId, snEdcName, err := parseJSONIDDataCombined(odooData.SnEdc)
					if err != nil {
						gotError = append(gotError, fmt.Sprintf("Cannot processing WO Number: %v, cause not have a valid SnEdc ID!", odooData.WoNumber))
						continue
					}

					edcTypeId, edcTypeName, err := parseJSONIDDataCombined(odooData.EdcType)
					if err != nil {
						gotError = append(gotError, fmt.Sprintf("Cannot processing WO Number: %v, cause not have a valid EdcType ID!", odooData.WoNumber))
						continue
					}

					technicianId, technicianName, err := parseJSONIDDataCombined(odooData.TechnicianId)
					if err != nil {
						gotError = append(gotError, fmt.Sprintf("Cannot processing WO Number: %v, cause not have a valid Technician ID!", odooData.WoNumber))
						continue
					}

					reasonCodeId, reasonCodeName, err := parseJSONIDDataCombined(odooData.ReasonCodeId)
					if err != nil {
						gotError = append(gotError, fmt.Sprintf("Cannot processing WO Number: %v, cause not have a valid ReasonCode ID!", odooData.WoNumber))
						continue
					}

					var slaDeadline, createDate, receivedDatetimeSpk, planDate, timesheetLastStop *time.Time
					if !odooData.SlaDeadline.Time.IsZero() {
						slaDeadline = &odooData.SlaDeadline.Time
					}
					if !odooData.CreateDate.Time.IsZero() {
						createDate = &odooData.CreateDate.Time
					}
					if !odooData.ReceivedDatetimeSpk.Time.IsZero() {
						receivedDatetimeSpk = &odooData.ReceivedDatetimeSpk.Time
					}
					if !odooData.PlanDate.Time.IsZero() {
						planDate = &odooData.PlanDate.Time
					}
					if !odooData.TimesheetLastStop.Time.IsZero() {
						timesheetLastStop = &odooData.TimesheetLastStop.Time
					}

					/* Get helpdesk.ticket data */
					odooModel := "helpdesk.ticket"
					odooFields := []string{
						"id",
						"x_job_id",
						"stage_id",
					}
					domain := []interface{}{
						[]interface{}{"active", "=", true},
						[]interface{}{"company_id", "=", companyAllowed},
						[]interface{}{"id", "=", ticketID},
					}

					odooParams := map[string]interface{}{
						"domain": domain,
						"model":  odooModel,
						"fields": odooFields,
						"order":  odooOrder,
					}

					payload := map[string]interface{}{
						"jsonrpc": h.YamlCfg.ApiODOO.JSONRPC,
						"params":  odooParams,
					}

					payloadBytes, err := json.Marshal(payload)
					if err != nil {
						quotedMsg := &waProto.ContextInfo{
							StanzaID:      &stanzaID,
							Participant:   &originalSenderJID,
							QuotedMessage: v.Message,
						}

						var sb strings.Builder
						sb.WriteString(fmt.Sprintf("Maaf, request *%v* Anda tidak dapat diproses karena mengalami kendala.\n", dataMap["RequestType"]))
						sb.WriteString(fmt.Sprintf("\n⚠ Error: _failed to marshal JSON payload, %v_ from SPK Number: %v", err, ticketNumber))
						textToSend := sb.String()

						_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
							ExtendedTextMessage: &waProto.ExtendedTextMessage{
								Text:        &textToSend,
								ContextInfo: quotedMsg,
							},
						})

						if err != nil {
							log.Println("[ERROR] Failed marshal json payload ticket result:", err)
						}

						return
					}

					ticketResult, err := getODOOData(h.YamlCfg, string(payloadBytes))
					if err != nil {
						quotedMsg := &waProto.ContextInfo{
							StanzaID:      &stanzaID,
							Participant:   &originalSenderJID,
							QuotedMessage: v.Message,
						}

						var sb strings.Builder
						sb.WriteString(fmt.Sprintf("Maaf, request *%v* Anda tidak dapat diproses karena mengalami kendala.\n", dataMap["RequestType"]))
						sb.WriteString(fmt.Sprintf("\n⚠ Error: _%v_ from SPK Number: %v", err, ticketNumber))
						textToSend := sb.String()

						_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
							ExtendedTextMessage: &waProto.ExtendedTextMessage{
								Text:        &textToSend,
								ContextInfo: quotedMsg,
							},
						})

						if err != nil {
							log.Println("[ERROR] Failed format odoo data ticket result:", err)
						}

						return
					}

					ticketResultArray, ok := ticketResult.([]interface{})
					if !ok {
						quotedMsg := &waProto.ContextInfo{
							StanzaID:      &stanzaID,
							Participant:   &originalSenderJID,
							QuotedMessage: v.Message,
						}

						var sb strings.Builder
						sb.WriteString(fmt.Sprintf("Maaf, request *%v* Anda tidak dapat diproses karena mengalami kendala.\n", dataMap["RequestType"]))
						sb.WriteString(fmt.Sprintf("\n⚠ Error: _failed to assert results as []interface{}_ from SPK Number: %v", ticketNumber))

						textToSend := sb.String()

						_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
							ExtendedTextMessage: &waProto.ExtendedTextMessage{
								Text:        &textToSend,
								ContextInfo: quotedMsg,
							},
						})

						if err != nil {
							log.Println("[ERROR] Failed format odoo data ticket result array :", err)
						}

						return
					}

					var bankVendor string
					odooTaskSource := strings.ToUpper(odooData.Source.String)

					if strings.Contains(odooTaskSource, "BMRI") {
						bankVendor = strings.ReplaceAll(odooTaskSource, "BMRI", "MANDIRI")
					} else {
						bankVendor = odooTaskSource
					}

					var ticketJobId string
					var ticketStageId int
					var ticketStageName string

					for _, ticketRecord := range ticketResultArray {
						ticketRecordMap, ok := ticketRecord.(map[string]interface{})
						if !ok {
							gotError = append(gotError, fmt.Sprintf("Invalid record format in resultArray from SPK Number: %v", ticketNumber))
							continue
						}

						var odooTicketData OdooTicketDataRequestItem
						jsonDataTicket, err := json.Marshal(ticketRecordMap)
						if err != nil {
							gotError = append(gotError, fmt.Sprintf("Failed to marshal recordMap: %v, from SPK Number: %v", err, ticketNumber))
							continue
						}

						err = json.Unmarshal(jsonDataTicket, &odooTicketData)
						if err != nil {
							gotError = append(gotError, fmt.Sprintf("Failed to unmarshal into odooData struct: %v, from SPK Number: %v", err, ticketNumber))
							continue
						}

						ticketJobId = odooTicketData.JobId.String
						ticketStageId, ticketStageName, err = parseJSONIDDataCombined(odooTicketData.StageId)
						if err != nil {
							gotError = append(gotError, fmt.Sprintf("Cannot processing SPK Number: %v, cause not have a valid stage ID!", ticketNumber))
							continue
						}
					}

					requestWAData := models.WaRequest{
						ID:                  uint(odooData.ID),
						Counter:             0,
						RequestType:         dataMap["RequestType"],
						MerchantName:        odooData.MerchantName.String,
						PicMerchant:         validPIC,
						PicPhone:            validPicPhoneNumber,
						MerchantAddress:     validMerchantAddress,
						Description:         odooData.Description.String,
						SlaDeadline:         slaDeadline,
						CreateDate:          createDate,
						ReceivedDatetimeSpk: receivedDatetimeSpk,
						PlanDate:            planDate,
						TimesheetLastStop:   timesheetLastStop,
						TaskType:            odooData.TaskType.String,
						CompanyId:           companyId,
						CompanyName:         companyName,
						StageId:             stageId,
						StageName:           stageName,
						HelpdeskTicketId:    ticketID,
						HelpdeskTicketName:  cleanedTicketNumber,
						Mid:                 odooData.Mid.String,
						Tid:                 odooData.Tid.String,
						Source:              bankVendor,
						MessageCC:           odooData.MessageCC.String,
						WoNumber:            odooData.WoNumber,
						StatusMerchant:      odooData.StatusMerchant.String,
						SnEdcId:             snEdcId,
						SnEdc:               snEdcName,
						EdcTypeId:           edcTypeId,
						EdcType:             edcTypeName,
						WoRemarkTiket:       odooData.WoRemarkTiket.String,
						Longitude:           odooData.Longitude.String,
						Latitude:            odooData.Latitude.String,
						TechnicianId:        technicianId,
						TechnicianName:      technicianName,
						ReasonCodeId:        reasonCodeId,
						ReasonCodeName:      reasonCodeName,
						IsOnCalling:         false,
						IsDone:              false,
						TempCS:              0,
						UpdatedToOdoo:       false,
						OrderWish:           dataMap["Order"],
						TargetScheduleDate:  dateTimeFormatPlanSchedule,
						Keterangan:          "",
						/* Additional fields on v3 */
						IsFinal:             false,
						LastUpdateBy:        "System",
						RequestToCC:         dataMap["Request to CC"],
						JobId:               ticketJobId,
						TicketStageId:       ticketStageId,
						TicketStageName:     ticketStageName,
						NextFollowUpTo:      "",
						IsOnCallingDatetime: nil,
						IsDoneDatetime:      nil,
						/* Whatsapp Info */
						GroupWaJid:        v.Info.Chat.String(),
						StanzaId:          stanzaID,
						OriginalSenderJid: originalSenderJID,
						/* All Teams */
						MarkDoneByOperational: false,
						MarkDoneByInventory:   false,
						MarkDoneByPmo:         false,
						MarkDoneByMonitoring:  false,

						RemarkByOperational: "",
						RemarkByInventory:   "",
						RemarkByPmo:         "",
						RemarkByMonitoring:  "",

						AttachmentByOperational: "",
						AttachmentByInventory:   "",
						AttachmentByPmo:         "",
						AttachmentByMonitoring:  "",
					}

					var existingRequest models.WaRequest
					err = h.Database.Table(h.YamlCfg.Db.TbWaReq).
						// Where("id = ?", requestWAData.ID).
						Where("helpdesk_ticket_id = ? AND x_no_task = ?",
							ticketID,
							odooData.WoNumber,
						).
						First(&existingRequest).Error

					if err != nil {
						if errors.Is(err, gorm.ErrRecordNotFound) {
							// Record does not exist, create a new one
							if err := h.Database.Table(h.YamlCfg.Db.TbWaReq).
								Create(&requestWAData).Error; err != nil {
								gotError = append(gotError, fmt.Sprintf("Failed to create record for WO Number: %v, got error: %v", odooData.WoNumber, err))
							} else {
								woFUbyCC = append(woFUbyCC, fmt.Sprintf("Request dengan WO Number: *%v*, SPK Number: *%v* berhasil disimpan kedalam database!",
									odooData.WoNumber,
									cleanedTicketNumber,
								))
							}
						} else {
							// Some other database error occurred
							gotError = append(gotError, fmt.Sprintf("Failed to check existing record for WO Number: %v, got error: %v", odooData.WoNumber, err))
						}
					} else {
						// Record exists, update it
						// woExisting = append(woExisting, fmt.Sprintf("Tidak dapat memproses request dengan WO Number: *%v*, karena sebelumnya sudah ada di database. *%v* ini sudah diminta untuk FU sejak %v",
						// 	existingRequest.WoNumber,
						// 	existingRequest.WoNumber,
						// 	existingRequest.CreatedAt.Format("02-01-2006 15:04:05")))
						formattedTime := existingRequest.CreatedAt.Format("Monday, 02 January 2006 @15:04:05")
						woExisting = append(woExisting, fmt.Sprintf("Request dengan WO Number: *%v*, SPK Number: *%v* sebelumnya sudah ada di database. Request ini sudah diminta untuk FU sejak %v",
							existingRequest.WoNumber,
							existingRequest.HelpdeskTicketName,
							formattedTime,
						))

						if err := h.Database.Table(h.YamlCfg.Db.TbWaReq).
							// Where("id = ?", requestWAData.ID).
							Where("id = ? AND is_on_calling = ? AND is_done = ?",
								existingRequest.ID,
								false,
								false,
							).
							Updates(map[string]interface{}{
								"counter":                 requestWAData.Counter,
								"request_type":            requestWAData.RequestType,
								"x_merchant":              requestWAData.MerchantName,
								"x_pic_merchant":          requestWAData.PicMerchant,
								"x_pic_phone":             requestWAData.PicPhone,
								"partner_street":          requestWAData.MerchantAddress,
								"description":             requestWAData.Description,
								"x_sla_deadline":          slaDeadline,
								"create_date":             createDate,
								"x_received_datetime_spk": receivedDatetimeSpk,
								"planned_date_begin":      planDate,
								"timesheet_last_stop":     timesheetLastStop,
								"x_task_type":             requestWAData.TaskType,
								"company_id":              requestWAData.CompanyId,
								"company_name":            requestWAData.CompanyName,
								"stage_id":                requestWAData.StageId,
								"stage_name":              requestWAData.StageName,
								"helpdesk_ticket_id":      requestWAData.HelpdeskTicketId,
								"helpdesk_ticket_name":    requestWAData.HelpdeskTicketName,
								"x_cimb_master_mid":       requestWAData.Mid,
								"x_cimb_master_tid":       requestWAData.Tid,
								"x_source":                requestWAData.Source,
								"x_message_call":          requestWAData.MessageCC,
								"x_no_task":               requestWAData.WoNumber,
								"x_status_merchant":       requestWAData.StatusMerchant,
								"x_studio_edc_id":         requestWAData.SnEdcId,
								"x_studio_edc":            requestWAData.SnEdc,
								"x_product_id":            requestWAData.EdcTypeId,
								"x_product":               requestWAData.EdcType,
								"x_wo_remark":             requestWAData.WoRemarkTiket,
								"x_longitude":             requestWAData.Longitude,
								"x_latitude":              requestWAData.Latitude,
								"technician_id":           requestWAData.TechnicianId,
								"technician_name":         requestWAData.TechnicianName,
								"reason_code_id":          requestWAData.ReasonCodeId,
								"reason_code_name":        requestWAData.ReasonCodeName,
								"is_on_calling":           requestWAData.IsOnCalling,
								"is_done":                 requestWAData.IsDone,
								"temp_cs":                 requestWAData.TempCS,
								"updated_to_odoo":         requestWAData.UpdatedToOdoo,
								"order_wish":              dataMap["Order"],
								"target_schedule_date":    dateTimeFormatPlanSchedule,
								"keterangan":              "",
								/* Additional fields in v3 */
								"is_final":               requestWAData.IsFinal,
								"x_job_id":               requestWAData.JobId,
								"last_update_by":         requestWAData.LastUpdateBy,
								"request_to_cc":          requestWAData.RequestToCC,
								"ticket_stage_id":        requestWAData.TicketStageId,
								"ticket_stage_name":      requestWAData.TicketStageName,
								"next_follow_up_to":      requestWAData.NextFollowUpTo,
								"is_on_calling_datetime": nil,
								"is_done_datetime":       nil,
								/* Whatsapp Info */
								"group_wa_jid":        requestWAData.GroupWaJid,
								"stanza_id":           requestWAData.StanzaId,
								"original_sender_jid": requestWAData.OriginalSenderJid,
								/* All Teams */
								"mark_done_by_operational":  requestWAData.MarkDoneByOperational,
								"remark_by_operational":     requestWAData.RemarkByOperational,
								"attachment_by_operational": requestWAData.AttachmentByOperational,

								"mark_done_by_inventory":  requestWAData.MarkDoneByInventory,
								"remark_by_inventory":     requestWAData.RemarkByInventory,
								"attachment_by_inventory": requestWAData.AttachmentByInventory,

								"mark_done_by_pmo":  requestWAData.MarkDoneByPmo,
								"remark_by_pmo":     requestWAData.RemarkByPmo,
								"attachment_by_pmo": requestWAData.AttachmentByPmo,

								"mark_done_by_monitoring":  requestWAData.MarkDoneByMonitoring,
								"remark_by_monitoring":     requestWAData.RemarkByMonitoring,
								"attachment_by_monitoring": requestWAData.AttachmentByMonitoring,
							}).
							Error; err != nil {
							gotError = append(gotError, fmt.Sprintf("Failed to update record for WO Number: %v, got error: %v", odooData.WoNumber, err))
						} else {
							woFUbyCC = append(woFUbyCC, fmt.Sprintf("Request dengan WO Number: *%v*, SPK Number: *%v* berhasil diupdate kedalam database!",
								odooData.WoNumber,
								cleanedTicketNumber,
							))
						}
					}
				} // .end of for looping odoo map data

				if len(woFUbyCC) > 0 {
					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("*[INFO]* Request *%v* Anda selanjutnya akan di _Follow Up_ oleh tim _*Call Center*_.", dataMap["RequestType"]))
					sb.WriteString("\n📌 Details:")
					for _, info := range woFUbyCC {
						sb.WriteString(fmt.Sprintf("\n    - _%v_", info))
					}
					textToSend := sb.String()

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &textToSend,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] Failed to send wo fu by cc:", err)
					}
				}

				if len(woExisting) > 0 {
					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					var sb strings.Builder
					// sb.WriteString(fmt.Sprintf("*[WARNING]* Request *%v* Anda mungkin tidak dapat di _Follow Up_ oleh tim _*Call Center*_, karena ada kendala.", dataMap["RequestType"]))
					sb.WriteString(fmt.Sprintf("*[WARNING]* Request *%v* Anda mungkin sudah ada sebelumnya dan telah di _Follow Up_ oleh tim _*Call Center*_.", dataMap["RequestType"]))
					sb.WriteString("\n⚠ Details:")
					for _, info := range woExisting {
						sb.WriteString(fmt.Sprintf("\n    - _%v_", info))
					}
					textToSend := sb.String()

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &textToSend,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] Failed to send wo existing:", err)
					}
				}

				if len(gotError) > 0 {
					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("*[ERROR]* Request *%v* Anda mungkin tidak dapat di _Follow Up_ oleh tim _*Call Center*_, karena dapat masalah di system.", dataMap["RequestType"]))
					sb.WriteString("\n☠ Details:")
					for _, info := range gotError {
						sb.WriteString(fmt.Sprintf("\n    - _%v_", info))
					}
					sb.WriteString(fmt.Sprintf("\nUntuk info lebih lanjut, silahkan hubungi *IT Support +%v* terkait masalah ini!", h.YamlCfg.Whatsmeow.WaSupport))
					textToSend := sb.String()

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &textToSend,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Println("[ERROR] Failed to send got error:", err)
					}
				}

				return
			}
		} // .end of switch case order wish
	} // .end of check all data request is not empty
} // .end of func replyRequestTemplate
