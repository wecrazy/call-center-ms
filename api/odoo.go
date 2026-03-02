package api

import (
	"bytes"
	"call_center_app/config"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

func GetSessionODOO(config *config.YamlConfig) ([]*http.Cookie, error) {
	db := config.ApiODOO.Db
	login := config.ApiODOO.Login
	password := config.ApiODOO.Password
	urlSession := config.ApiODOO.UrlSession
	jsonRPC := config.ApiODOO.JSONRPC

	requestJSON := `{
		"jsonrpc": %v,
		"params": {
			"db": "%s",
			"login": "%s",
			"password": "%s"
		}
	}`
	rawJSON := fmt.Sprintf(requestJSON, jsonRPC, db, login, password)

	maxRetriesStr := config.ApiODOO.MaxRetry
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		log.Printf("Invalid ODOO_MAX_RETRY value: %v", err)
		return nil, err
	}

	retryDelayStr := config.ApiODOO.RetryDelay
	retryDelay, err := strconv.ParseInt(retryDelayStr, 0, 64)
	if err != nil {
		log.Printf("Invalid ODOO_RETRY_DELAY value: %v", err)
		return nil, err
	}

	reqTimeout, err := time.ParseDuration(config.ApiODOO.SessionTimeout)
	if err != nil {
		log.Printf("Invalid ODOO_SESSION_TIMEOUT value: %v", err)
		return nil, err
	}

	var response *http.Response

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlSession, bytes.NewBufferString(rawJSON))
		if err != nil {
			log.Printf("Error creating request: %v", err)
			return nil, err
		}

		request.Header.Set("Content-Type", "application/json")

		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Timeout: reqTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}

		// Send the request
		response, err = client.Do(request)
		if err != nil {
			log.Printf("Error making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			log.Printf("Bad response, status code: %d (attempt %d/%d)", response.StatusCode, attempts, maxRetries)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		log.Printf("POST request failed with status code: %v", response.StatusCode)
		return nil, err
	}

	_, err = ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	// Store and return the cookies
	cookieODOO := response.Cookies()
	// log.Print("ODOO session obtained successfully.")
	return cookieODOO, nil
}

func ODOOAPI(config *config.YamlConfig, APIReq string, domain interface{}, model string, fields []string, order string) (interface{}, error) {
	fieldsJSON, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("error marshaling fields: %v", err)
	}

	domainJSON, err := json.Marshal(domain)
	if err != nil {
		return nil, fmt.Errorf("error marshaling domain: %v", err)
	}

	requestJSON := `{
		"jsonrpc": "2.0", 
		"params": {
			"model": "%s",  
			"fields": %s,
			"domain": %s,
			"order": "%s"
		}
	}`

	rawJSON := fmt.Sprintf(requestJSON, model, string(fieldsJSON), string(domainJSON), order)

	switch APIReq {
	case "GetData":
		return ODOOGetData(config, rawJSON)
	default:
		return nil, fmt.Errorf("unknown API request type: %s", APIReq)
	}
}

func ODOOGetData(config *config.YamlConfig, req string) (interface{}, error) {
	urlGetData := config.ApiODOO.UrlGetData

	maxRetriesStr := config.ApiODOO.MaxRetry
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		log.Printf("Invalid ODOO_MAX_RETRY value: %v", err)
		return nil, err
	}

	retryDelayStr := config.ApiODOO.RetryDelay
	retryDelay, err := strconv.ParseInt(retryDelayStr, 0, 64)
	if err != nil {
		log.Printf("Invalid ODOO_RETRY_DELAY value: %v", err)
		return nil, err
	}

	var response *http.Response
	cookieODOO, err := GetSessionODOO(config)
	if err != nil {
		log.Printf("Got error while trying to get session ODOO: %v", err)
		return nil, err
	}
	// log.Printf("Cookies: %v", cookieODOO)

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlGetData, bytes.NewBufferString(req))
		if err != nil {
			log.Printf("Error creating request: %v", err)
			return nil, err
		}

		request.Header.Set("Content-Type", "application/json")

		for _, cookie := range cookieODOO {
			request.AddCookie(cookie)
		}

		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}

		// Send the request
		response, err = client.Do(request)
		if err != nil {
			log.Printf("Error making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			log.Printf("Bad response, status code: %d (attempt %d/%d)", response.StatusCode, attempts, maxRetries)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		log.Printf("POST request failed with status code: %v", response.StatusCode)
		return nil, err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	// log.Print("Response Body:", string(body))

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		log.Printf("Error parsing JSON Response: %v", err)
		return nil, err
	}

	// Check for error response from Odoo
	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			log.Printf("Error code: %v, message: %v", errorResponse["code"], errorMessage)
			return nil, fmt.Errorf("error code: %v, message: %v", errorResponse["code"], errorMessage)
		}
	}

	// Check for the result in JSON response
	if result, ok := jsonResponse["result"].(map[string]interface{}); ok {
		// Log the message and success status if they exist
		if message, ok := result["message"].(string); ok {
			success, successOk := result["success"]
			log.Printf("ODOO Result, message: %v, status: %v", message, successOk && success == true)
		}
	}

	// Check for the existence and validity of the "result" field
	result, resultExists := jsonResponse["result"]
	if !resultExists {
		log.Print("Result field missing in the response!")
		log.Printf("Error with params: %v", bytes.NewBufferString(req))
		return nil, nil
	}

	// Check if the result is an array and ensure it's not empty
	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) == 0 {
		log.Print("Unexpected result format or empty result!")
		log.Printf("Error with params: %v", bytes.NewBufferString(req))
		return nil, nil
	}

	return result, nil
}

func ODOOUpdateData(config *config.YamlConfig, req string) (interface{}, error) {
	urlUpdateData := config.ApiODOO.UrlUpdateData

	maxRetriesStr := config.ApiODOO.MaxRetry
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		log.Printf("Invalid ODOO_MAX_RETRY value: %v", err)
		return nil, err
	}

	retryDelayStr := config.ApiODOO.RetryDelay
	retryDelay, err := strconv.ParseInt(retryDelayStr, 0, 64)
	if err != nil {
		log.Printf("Invalid ODOO_RETRY_DELAY value: %v", err)
		return nil, err
	}

	var response *http.Response
	cookieODOO, err := GetSessionODOO(config)
	if err != nil {
		log.Printf("Got error while trying to get session ODOO: %v", err)
		return nil, err
	}

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlUpdateData, bytes.NewBufferString(req))
		if err != nil {
			log.Printf("Error creating request: %v", err)
			return nil, err
		}

		request.Header.Set("Content-Type", "application/json")

		for _, cookie := range cookieODOO {
			request.AddCookie(cookie)
		}

		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}

		response, err = client.Do(request)
		if err != nil {
			log.Printf("Error making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}

		if response.StatusCode == http.StatusOK {
			break
		} else {
			log.Printf("Bad response, status code: %d (attempt %d/%d)", response.StatusCode, attempts, maxRetries)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		log.Printf("POST request failed with status code: %v", response.StatusCode)
		return nil, err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	// log.Print("Response Body:", string(body))

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		log.Printf("Error parsing JSON Response: %v", err)
		return nil, err
	}

	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			log.Printf("Error code: %v, message: %v", errorResponse["code"], errorMessage)
			return nil, fmt.Errorf("error code: %v, message: %v", errorResponse["code"], errorMessage)
		}
	}

	if result, exists := jsonResponse["result"]; exists && result != nil {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if status, ok := resultMap["status"].(float64); ok {
				if int(status) == 200 {
					if resultMap["success"] == true && resultMap["response"] == true && resultMap["message"] == "Success" {
						// log.Print("ODOO data update successful.")
						return "Success update ODOO data", nil
					} else {
						errorMsg := fmt.Sprintf("odoo error: [status]%v; [success: %v]; [response: %v]; [message: %v]",
							status, resultMap["success"], resultMap["response"], resultMap["message"])
						log.Print(errorMsg)
						return nil, errors.New(errorMsg)
					}
				} else {
					errorMsg := fmt.Sprintf("odoo error: [%v] %v, from json request: %v",
						status, resultMap["message"], req)
					log.Print(errorMsg)
					return nil, errors.New(errorMsg)
				}
			} else {
				errorMsg := fmt.Sprintf("Expected status to be float64, but got: %v", resultMap["status"])
				log.Print(errorMsg)
				return nil, errors.New(errorMsg)
			}
		} else {
			log.Printf("Result exists but is not a valid map: %v", result)
			return nil, fmt.Errorf("invalid result format: %v", result)
		}
	} else {
		log.Print("Missing 'result' key or it is nil in the response")
		return nil, fmt.Errorf("missing or empty 'result' key in response")
	}
}
