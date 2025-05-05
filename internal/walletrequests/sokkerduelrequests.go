package walletrequests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"time"

	"github.com/Lavizord/checkers-server/internal/models"
)

func SokkerDuelGetWallet(baseUrl string, token string) (*models.WalletResponse, error) {
	// Parse the base URL
	baseUrlParsed, err := url.Parse(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse base URL: %v", err)
	}
	baseUrlParsed.Path = path.Join(baseUrlParsed.Path, "wallet")

	req, err := http.NewRequest("POST", baseUrlParsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create request: %v", err)
	}
	req.Header.Set("x-access-token", token)
	req.Header.Set("Content-Type", "application/json")

	// Print request (updated to show all headers)
	//fmt.Println("===== API REQUEST =====")
	//log.Printf("URL: %s\n", baseUrlParsed.String())
	//log.Printf("Headers: %v\n", req.Header)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 10 * time.Second, // Added timeout for reliability.
		Jar:     jar,              // Enable cookies // TODO: Review this.

	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response body: %v", err)
	}

	// Print response
	//fmt.Println("===== API RESPONSE =====")
	//log.Printf("Status: %s\n", resp.Status)
	//log.Printf("Body: %s\n", string(body))

	// Try to unmarshal as an error first
	var apiError models.SokkerDuelErrorResponse
	if err := json.Unmarshal(body, &apiError); err == nil && apiError.Status == "error" {
		return nil, fmt.Errorf(apiError.Resp)
	}
	// If no error, proceed with normal response
	var walletResponse models.WalletResponse
	if err := json.Unmarshal(body, &walletResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}
	return &walletResponse, nil
}

func SokkerDuelPostBet(session models.Session, betData models.SokkerDuelBet) (*models.SokkerDuelBetResponse, error) {
	baseUrl, err := url.Parse(session.OperatorBaseUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %v", err)
	}
	baseUrl.Path = path.Join(baseUrl.Path, "bet")
	jsonData, err := json.Marshal(betData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize bet data: %v", err)
	}
	req, err := http.NewRequest("POST", baseUrl.String(), bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create bet request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", session.Token)

	// Print request
	//log.Println("===== API REQUEST =====")
	//log.Printf("URL: %s\n", baseUrl.String())
	//log.Printf("Headers: x-access-token=%s\n", session.Token)
	//log.Printf("Body: %s\n", string(jsonData))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send bet request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Print response
	//log.Println("===== API RESPONSE =====")
	//log.Printf("Status: %s\n", resp.Status)
	//log.Printf("Body: %s\n", string(body))

	// Try to unmarshal as an error first
	var apiError models.SokkerDuelErrorResponse
	if err := json.Unmarshal(body, &apiError); err == nil && apiError.Status == "error" {
		return nil, fmt.Errorf("api error: %s", apiError.Resp)
	}
	// If no error, proceed with normal response
	var walletResponse models.SokkerDuelBetResponse
	if err := json.Unmarshal(body, &walletResponse); err != nil {
		return nil, fmt.Errorf("failed to parse bet response: %v. With err: %v", walletResponse, err)
	}
	return &walletResponse, nil
}

func SokkerDuelPostWin(session models.Session, winData models.SokkerDuelWin) (*models.SokkerDuelWinResponse, error) {
	baseUrl, err := url.Parse(session.OperatorBaseUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %v", err)
	}
	baseUrl.Path = path.Join(baseUrl.Path, "win")

	jsonData, err := json.Marshal(winData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize win data: %v", err)
	}
	req, err := http.NewRequest("POST", baseUrl.String(), bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create win request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", session.Token)

	// Print request
	//log.Println("===== API REQUEST =====")
	//log.Printf("URL: %s\n", baseUrl.String())
	//log.Printf("Headers: x-access-token=%s\n", session.Token)
	//log.Printf("Body: %s\n", string(jsonData))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send win request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read win response body: %v", err)
	}
	// Print response
	//log.Println("===== API RESPONSE =====")
	//log.Printf("Status: %s\n", resp.Status)
	//log.Printf("Body: %s\n", string(body))
	// Try to unmarshal as an error first
	var apiError models.SokkerDuelErrorResponse
	if err := json.Unmarshal(body, &apiError); err == nil && apiError.Status == "error" {
		return nil, fmt.Errorf("api error: %s", apiError.Resp)
	}
	var walletResponse models.SokkerDuelWinResponse
	if err := json.Unmarshal(body, &walletResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v. With err: %v", walletResponse, err)
	}
	return &walletResponse, nil
}
