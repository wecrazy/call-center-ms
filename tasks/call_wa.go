package tasks

import (
	"bytes"
	"call_center_app/config"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func CallWA(config *config.YamlConfig, csIP, purpose, kontak, phone, tid, url string) {
	type RequestBody struct {
		Purpose string `json:"purpose"`
		Kontak  string `json:"kontak"`
		NoHP    string `json:"no_hp"`
		Tid     string `json:"tid"`
		URL     string `json:"url"`
	}

	requestBody := RequestBody{
		Purpose: purpose,
		Kontak:  kontak,
		URL:     url,
		Tid:     tid,
		NoHP:    phone,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Printf("error encoding JSON: %v", err)
		return
	}

	resp, err := http.Post(fmt.Sprintf("http://%s:%v/wa", csIP, config.ApiWA.Port), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("error making POST req: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v", err)
		return
	}

	fmt.Printf("IP: %v, Call To: %v, Response Status: %v, Response Body: %v", csIP, requestBody.Kontak, resp.Status, string(body))
}

func ChatWA(config *config.YamlConfig, csIP, purpose, kontak, phone, chat, tid, url string) {
	type RequestBody struct {
		Purpose string `json:"purpose"`
		Kontak  string `json:"kontak"`
		NoHP    string `json:"no_hp"`
		Tid     string `json:"tid"`
		URL     string `json:"url"`
		Chat    string `json:"chat"`
	}

	requestBody := RequestBody{
		Purpose: purpose,
		Kontak:  kontak,
		URL:     url,
		Tid:     tid,
		NoHP:    phone,
		Chat:    chat,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Printf("error encoding JSON: %v", err)
		return
	}

	resp, err := http.Post(fmt.Sprintf("http://%s:%v/chat", csIP, config.ApiWA.Port), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("error making POST req: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v", err)
		return
	}

	fmt.Printf("IP: %v, Chat To: %v, Response Status: %v, Response Body: %v", csIP, requestBody.Kontak, resp.Status, string(body))
}
