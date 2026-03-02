package whatsapp

import (
	"encoding/json"
	"log"
	"os"
)

// GetGroupInfo retrieves group info and saves it to a file
func (h *WhatsmeowHandler) GetGroupInfo() {
	data, err := h.Client.GetJoinedGroups()
	if err != nil {
		log.Print(err)
		return
	}

	// // Log group info (Uncomment if needed)
	// for _, group := range data {
	// 	log.Printf("\n📌 Group: %s (ID: %s)\n", group.Name, group.JID.String())
	// 	log.Printf("   🔹 Creator: %s \n", group.OwnerJID.User)

	// 	log.Println("   👥 Participants:")
	// 	for _, participant := range group.Participants {
	// 		role := "member"
	// 		if participant.IsAdmin {
	// 			role = "admin"
	// 		}
	// 		if participant.IsSuperAdmin {
	// 			role = "superadmin"
	// 		}
	// 		log.Printf("      - %s (%s)\n", participant.JID.String(), role)
	// 	}
	// }

	// Convert to JSON
	jsonData, _ := json.MarshalIndent(data, "", "  ")

	// Save to file
	err = os.WriteFile(h.YamlCfg.Whatsmeow.WaGroup, jsonData, 0644)
	if err != nil {
		log.Printf("Error writing to file: %v", err)
		return
	}

	log.Printf("✅ Group data saved to %v", h.YamlCfg.Whatsmeow.WaGroup)
	// return
}
