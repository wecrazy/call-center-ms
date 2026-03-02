package whatsapp

import (
	"call_center_app/models"
	"log"
	"sync"
)

var (
	getReasonCodeMutex sync.Mutex
	ReasonCodeAllowed  []string
)

func (h *WhatsmeowHandler) GetAllowedReasonCode() {
	getReasonCodeMutex.Lock()
	defer getReasonCodeMutex.Unlock()

	var dataODOOReasonCode []models.OdooReasonCode
	if err := h.Database.Table(h.YamlCfg.Db.TbOdooRC).Raw(`
		SELECT x_reason_code, x_name, MAX(id) as id 
		FROM odoo_reason_code 
		WHERE LENGTH(x_reason_code) = 3 
		GROUP BY x_reason_code, x_name
	`).Scan(&dataODOOReasonCode).Error; err != nil {
		log.Print(err)
		return
	}

	for _, data := range dataODOOReasonCode {
		ReasonCodeAllowed = append(ReasonCodeAllowed, data.Name, data.ReasonCode)
	}

	// Remove duplicates in one step
	ReasonCodeAllowed = uniqueSlice(ReasonCodeAllowed)
}
