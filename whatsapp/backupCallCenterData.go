package whatsapp

import (
	"log"
	"sync"
	"time"
)

var dumpCCDataFUMutex sync.Mutex

func (h *WhatsmeowHandler) DumpDataCCForFU() {
	if !dumpCCDataFUMutex.TryLock() {
		log.Println("DumpDataCCForFU is already running, skipping execution.")
		return
	}
	defer dumpCCDataFUMutex.Unlock()

	taskDoing := "dump CC data for follow up"

	log.Printf("Running task %v @%v", taskDoing, time.Now())

	// soon do something here !!

	log.Printf("Task %v successfully executed @%v", taskDoing, time.Now())
}
