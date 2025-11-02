package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	apisolvecaptcha "github.com/solvercaptcha/solvecaptcha-go"
)

type AuthManager struct {
	config     *Config
	httpClient *http.Client
	lastRefresh time.Time
	sessionCookies map[string]string // Store session cookies
}

// Multi-step login structures
type LoginStep1Request struct {
	Account           string `json:"account"`
	EnablePasskey     bool   `json:"enable_passkey"`
	KeepLoginState    bool   `json:"keep_login_state"`
	Lang              string `json:"lang"`
	PasskeySupport    bool   `json:"passkey_support"`
	SrpA              string `json:"srp_A"`
	Version           string `json:"version"`
}

type LoginStep1Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Data struct {
			TraditionalLogin bool `json:"traditional_login"`
		} `json:"data"`
		Done      bool   `json:"done"`
		SessionID string `json:"session_id"`
		Step      int    `json:"step"`
	} `json:"data"`
}

type LoginStep2Request struct {
	CaptchaToken string `json:"captcha_token"`
	SessionID    string `json:"session_id"`
}

type LoginStep2Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Data struct {
			CaptchaData string `json:"captcha_data"`
			KeyType     string `json:"key_type"`
			SiteKey     string `json:"site_key"`
		} `json:"data"`
		Done      bool     `json:"done"`
		Require   []string `json:"require"`
		SessionID string   `json:"session_id"`
		Step      int      `json:"step"`
	} `json:"data"`
}

type LoginStep3Request struct {
	ClientM   string `json:"client_M"`
	SessionID string `json:"session_id"`
}

type LoginStep3Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Data struct {
			EmailCodeID                  string `json:"email_code_id"`
			MaskAccounts                 []struct {
				AccountType  string `json:"account_type"`
				AccountValue string `json:"account_value"`
			} `json:"maskAccounts"`
			ServerH                      string `json:"server_H"`
			VerificationCodeTokenData    struct {
				ExpireAt int64  `json:"expire_at"`
				Token    string `json:"token"`
			} `json:"verification_code_token_data"`
		} `json:"data"`
		Done      bool     `json:"done"`
		Require   []string `json:"require"`
		SessionID string   `json:"session_id"`
		Step      int      `json:"step"`
	} `json:"data"`
}

type LoginStep4Request struct {
	EmailCode   string `json:"email_code"`
	EmailCodeID string `json:"email_code_id"`
	SessionID   string `json:"session_id"`
}

type LoginStep4Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Data struct {
			AccessToken struct {
				ExpireAt int64  `json:"expire_at"`
				Token    string `json:"token"`
			} `json:"access_token"`
			App          string `json:"app"`
			RefreshToken struct {
				ExpireAt int64  `json:"expire_at"`
				Token    string `json:"token"`
			} `json:"refresh_token"`
		} `json:"data"`
		Done      bool   `json:"done"`
		SessionID string `json:"session_id"`
		Step      int    `json:"step"`
	} `json:"data"`
}

// Legacy structures for backward compatibility
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	} `json:"data"`
}

func NewAuthManager(config *Config) *AuthManager {
	return &AuthManager{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		sessionCookies: make(map[string]string),
	}
}

// RefreshTokenIfNeeded checks if token needs refresh and refreshes it
func (am *AuthManager) RefreshTokenIfNeeded() error {
	log.Println("Attempting to refresh authentication...")
	
	// Always try to refresh using stored credentials when called
	if am.config.Email != "" && am.config.Password != "" {
		log.Println("Refreshing token using stored credentials...")
		return am.refreshWithCredentials()
	}

	// Try to refresh using browser session
	return am.refreshFromBrowser()
}

// refreshWithCredentials refreshes token using login flow
func (am *AuthManager) refreshWithCredentials() error {
	log.Println("Starting login process...")
	
	// Try legacy login first (simpler approach)
	if err := am.performLegacyLogin(); err != nil {
		log.Printf("Legacy login failed: %v, trying multi-step login...", err)
		return am.performMultiStepLogin()
	}
	
	return nil
}

// performMultiStepLogin implements the new OTP-based login flow
func (am *AuthManager) performMultiStepLogin() error {
	deviceID := "7754041a-7df7-4454-80aa-2160aec01882"
	fpDid := "8de143c466b06cd9949cb90da027a288"
	clientID := "gmgn_web_20251101-6461-0986672"
	
	// Initialize with common browser cookies if not already set
	am.initializeBrowserCookies()
	
	// Step 0: Get public key first
	pubkeyResp, err := am.getPubkey(deviceID, fpDid, clientID)
	if err != nil {
		return fmt.Errorf("pubkey step failed: %w", err)
	}
	
	log.Printf("=== PUBKEY RESPONSE ===")
	log.Printf("Pubkey Response: %+v", pubkeyResp)
	log.Printf("Available cookies after pubkey: %+v", am.sessionCookies)
	log.Printf("=== END PUBKEY RESPONSE ===")
	
	baseURL := fmt.Sprintf("https://gmgn.ai/account/login_v3?device_id=%s&fp_did=%s&client_id=%s&from_app=gmgn&app_ver=20251101-6461-0986672&tz_name=Asia%%2FJakarta&tz_offset=25200&app_lang=id&os=web&worker=0", 
		deviceID, fpDid, clientID)
	
	// Step 1: Submit email
	step1Resp, err := am.loginStep1(baseURL)
	if err != nil {
		return fmt.Errorf("step 1 failed: %w", err)
	}
	
	// DEBUG: Print detailed step 1 response
	log.Printf("=== STEP 1 DETAILED RESPONSE ===")
	log.Printf("Code: %d", step1Resp.Code)
	log.Printf("Message: %s", step1Resp.Message)
	log.Printf("Data.Done: %t", step1Resp.Data.Done)
	log.Printf("Data.SessionID: %s", step1Resp.Data.SessionID)
	log.Printf("Data.Step: %d", step1Resp.Data.Step)
	log.Printf("Data.Data.TraditionalLogin: %t", step1Resp.Data.Data.TraditionalLogin)
	log.Printf("Available cookies after step 1: %+v", am.sessionCookies)
	log.Printf("Config cookies: %s", am.config.Cookies)
	log.Printf("=== END STEP 1 RESPONSE ===")
	
	// Continue with multi-step login flow regardless of traditional_login flag
	log.Println("Continuing with multi-step login flow...")
	
	// Step 2: Submit password as captcha_token
	step2Resp, err := am.loginStep2(baseURL, step1Resp.Data.SessionID)
	if err != nil {
		return fmt.Errorf("step 2 failed: %w", err)
	}
	
	log.Printf("Step 2 Response: Code=%d, Done=%t, Require=%v", 
		step2Resp.Code, step2Resp.Data.Done, step2Resp.Data.Require)
	
	if step2Resp.Data.Done {
		log.Println("Login completed after step 2")
		return am.extractTokensFromCookies()
	}
	
	// Check if captcha verification is required
	if contains(step2Resp.Data.Require, "verify_recaptcha") {
		log.Printf("Captcha required: site_key=%s, key_type=%s", 
			step2Resp.Data.Data.SiteKey, step2Resp.Data.Data.KeyType)
		
		// Solve captcha manually or with service
		captchaToken, err := am.solveCaptcha(step2Resp.Data.Data.SiteKey, step2Resp.Data.Data.KeyType)
		if err != nil {
			return fmt.Errorf("captcha solving failed: %w", err)
		}
		
		// Submit captcha solution
		step2CaptchaResp, err := am.submitCaptchaSolution(baseURL, step2Resp.Data.SessionID, captchaToken)
		if err != nil {
			return fmt.Errorf("captcha submission failed: %w", err)
		}
		
		log.Printf("Captcha Step Response: Code=%d, Done=%t, Require=%v", 
			step2CaptchaResp.Code, step2CaptchaResp.Data.Done, step2CaptchaResp.Data.Require)
		
		// Update step2Resp with captcha response
		step2Resp = step2CaptchaResp
	}
	
	// Step 3: Submit client_M
	step3Resp, err := am.loginStep3(baseURL, step2Resp.Data.SessionID)
	if err != nil {
		return fmt.Errorf("step 3 failed: %w", err)
	}
	
	log.Printf("Step 3 Response: Code=%d, Done=%t, Require=%v", 
		step3Resp.Code, step3Resp.Data.Done, step3Resp.Data.Require)
	
	if step3Resp.Data.Done {
		log.Println("Login completed after step 3")
		return am.extractTokensFromCookies()
	}
	
	// Step 4: Handle OTP if required
	if contains(step3Resp.Data.Require, "email_code") {
		log.Printf("Email OTP required for: %s", step3Resp.Data.Data.MaskAccounts[0].AccountValue)
		return am.loginStep4(baseURL, step3Resp)
	}
	
	return fmt.Errorf("login flow completed but not done")
}

// loginStep1 submits email address with SRP_A
func (am *AuthManager) loginStep1(baseURL string) (*LoginStep1Response, error) {
	// Generate SRP_A value (this should be properly generated using SRP protocol)
	// For now, using the example value from your trace
	srpA := "61f0587b38c3549bc3c755097019caa30a94f2848c1517856452a981f167e204e5b04c6c33cd447cfdcfee7821eabc9627467b482ae0fa2433426051002c03d16ff3a8099d397b35cfcf7a473d3a47a24eef1a712c9c118733bbc0adc449edc679c5298ebf95d5e1b7c5f74ee4a76af2b0cfaa63623ec894ee41a1e90731946a84f78d94b28989e65f9beb27d0bee75e48786b0769eb4b80b746bbb4053b1efd54c55451c0d877dfb24de393a39c0f0858b1b0f754d9872e1dbc0b1b1a975b54decd25b2d07cbf36876052214c5c944683430fa5f9e92cd238bf9abbd6b5a79e6f70e56ee7efaea0166946c8a6b80c14639a6652b2ef46bc6d8253439fb1ce6d"
	
	req := LoginStep1Request{
		Account:           am.config.Email,
		EnablePasskey:     false, // Changed to false as per your trace
		KeepLoginState:    false,
		Lang:              "id",
		PasskeySupport:    false, // Changed to false as per your trace
		SrpA:              srpA,
		Version:           "2.0",
	}
	
	_, body, err := am.makeLoginRequest(baseURL, req)
	if err != nil {
		return nil, err
	}
	
	log.Printf("Step1 Response: %s", string(body))
	
	var result LoginStep1Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse step1 response: %w", err)
	}
	
	return &result, nil
}

// loginStep2 submits password with captcha token
func (am *AuthManager) loginStep2(baseURL, sessionID string) (*LoginStep2Response, error) {
	log.Printf("=== STEP 2 REQUEST ===")
	log.Printf("BaseURL: %s", baseURL)
	log.Printf("SessionID: %s", sessionID)
	log.Printf("Password length: %d", len(am.config.Password))
	
	req := LoginStep2Request{
		CaptchaToken: am.config.Password, // Using password as captcha_token
		SessionID:    sessionID,
	}
	
	log.Printf("Request payload: %+v", req)
	log.Printf("=== END STEP 2 REQUEST ===")
	
	_, body, err := am.makeLoginRequest(baseURL, req)
	if err != nil {
		return nil, err
	}
	
	log.Printf("Step2 Response: %s", string(body))
	
	var result LoginStep2Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse step2 response: %w", err)
	}
	
	return &result, nil
}

// loginStep3 submits client_M after captcha verification
func (am *AuthManager) loginStep3(baseURL, sessionID string) (*LoginStep3Response, error) {
	// This client_M would normally be generated from SRP protocol
	// Using a placeholder for now
	req := LoginStep3Request{
		ClientM:   "5a7422b0be158fee81229ab62c8befc6030072acca621e94bacf8c2f3bc925a4",
		SessionID: sessionID,
	}
	
	_, body, err := am.makeLoginRequest(baseURL, req)
	if err != nil {
		return nil, err
	}
	
	var result LoginStep3Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse step3 response: %w", err)
	}
	
	return &result, nil
}

// loginStep4 submits email OTP code
func (am *AuthManager) loginStep4(baseURL string, step3Resp *LoginStep3Response) error {
	log.Printf("Email OTP required. Code will be sent to: %s", 
		step3Resp.Data.Data.MaskAccounts[0].AccountValue)
	
	// Prompt user for OTP
	fmt.Print("Enter email OTP code: ")
	reader := bufio.NewReader(os.Stdin)
	otpCode, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read OTP code: %w", err)
	}
	otpCode = strings.TrimSpace(otpCode)
	
	req := LoginStep4Request{
		EmailCode:   otpCode,
		EmailCodeID: step3Resp.Data.Data.EmailCodeID,
		SessionID:   step3Resp.Data.SessionID,
	}
	
	_, body, err := am.makeLoginRequest(baseURL, req)
	if err != nil {
		return fmt.Errorf("OTP verification failed: %w", err)
	}
	
	var step4Resp LoginStep4Response
	if err := json.Unmarshal(body, &step4Resp); err != nil {
		return fmt.Errorf("failed to parse step4 response: %w", err)
	}
	
	if !step4Resp.Data.Done {
		return fmt.Errorf("login not completed after OTP verification")
	}
	
	// Update config with new tokens
	am.config.BearerToken = step4Resp.Data.Data.AccessToken.Token
	am.lastRefresh = time.Now()
	
	// Save tokens to .env file
	if err := am.config.SaveTokensToEnv(); err != nil {
		log.Printf("Warning: Failed to save tokens to .env: %v", err)
	}
	
	log.Println("Successfully completed multi-step login with OTP")
	return nil
}

// makeLoginRequest is a helper for making login API requests
func (am *AuthManager) makeLoginRequest(url string, payload interface{}) (*http.Response, []byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://gmgn.ai")
	req.Header.Set("Referer", "https://gmgn.ai/login")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("sec-ch-ua", `"Chromium";v="130", "Google Chrome";v="130", "Not?A_Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Linux"`)
	
	// Add existing session cookies to request
	am.addCookiesToRequest(req)
	
	resp, err := am.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	// Extract and store cookies from response
	am.extractCookiesFromResponse(resp)
	
	// Handle compressed responses
	var reader io.Reader = resp.Body
	encoding := resp.Header.Get("Content-Encoding")
	
	switch encoding {
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}
	
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != 200 {
		return resp, body, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	return resp, body, nil
}

// addCookiesToRequest adds stored session cookies to the request
func (am *AuthManager) addCookiesToRequest(req *http.Request) {
	var cookieParts []string
	
	// Add stored session cookies
	for name, value := range am.sessionCookies {
		cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", name, value))
	}
	
	// Add any existing cookies from config
	if am.config.Cookies != "" {
		cookieParts = append(cookieParts, am.config.Cookies)
	}
	
	if len(cookieParts) > 0 {
		req.Header.Set("Cookie", strings.Join(cookieParts, "; "))
	}
}

// extractCookiesFromResponse extracts cookies from response and stores them
func (am *AuthManager) extractCookiesFromResponse(resp *http.Response) {
	for _, cookie := range resp.Cookies() {
		am.sessionCookies[cookie.Name] = cookie.Value
		log.Printf("Stored cookie: %s=%s", cookie.Name, cookie.Value)
	}
	
	// Update config cookies with all current cookies
	var cookieParts []string
	for name, value := range am.sessionCookies {
		cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", name, value))
	}
	am.config.Cookies = strings.Join(cookieParts, "; ")
}

// performLegacyLogin implements the old simple login flow as fallback
func (am *AuthManager) performLegacyLogin() error {
	loginReq := LoginRequest{
		Email:    am.config.Email,
		Password: am.config.Password,
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://gmgn.ai/account/login", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://gmgn.ai")
	req.Header.Set("Referer", "https://gmgn.ai/login")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make login request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	if loginResp.Code != 0 {
		return fmt.Errorf("login failed: %s", loginResp.Message)
	}

	// Update config with new token
	am.config.BearerToken = loginResp.Data.Token
	am.lastRefresh = time.Now()

	// Extract cookies from response
	if cookies := resp.Header.Get("Set-Cookie"); cookies != "" {
		am.config.Cookies = cookies
	}

	// Save tokens to .env file
	if err := am.config.SaveTokensToEnv(); err != nil {
		log.Printf("Warning: Failed to save tokens to .env: %v", err)
	}

	log.Println("Successfully refreshed authentication token using legacy login")
	return nil
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// handleTraditionalLogin handles the traditional login flow after step 1
func (am *AuthManager) handleTraditionalLogin(baseURL, sessionID string) error {
	log.Println("Handling traditional login flow...")
	
	// Traditional login means we can directly authenticate with password
	// Step 2: Submit password as captcha_token
	req := LoginStep2Request{
		CaptchaToken: am.config.Password,
		SessionID:    sessionID,
	}
	
	_, body, err := am.makeLoginRequest(baseURL, req)
	if err != nil {
		return fmt.Errorf("traditional login step 2 failed: %w", err)
	}
	
	log.Printf("Traditional Login Step 2 Response: %s", string(body))
	
	// Try to parse as step2 response
	var step2Resp LoginStep2Response
	if err := json.Unmarshal(body, &step2Resp); err == nil {
		if step2Resp.Data.Done {
			// Login completed, check for tokens in cookies
			if sid, exists := am.sessionCookies["sid"]; exists && sid != "" {
				log.Printf("Found session cookie after step 2: %s", sid)
				am.config.BearerToken = sid
				am.lastRefresh = time.Now()
				
				// Save tokens to .env file
				if err := am.config.SaveTokensToEnv(); err != nil {
					log.Printf("Warning: Failed to save tokens to .env: %v", err)
				}
				
				log.Println("Successfully completed traditional login")
				return nil
			}
		} else {
			// Continue with step 3 if needed
			log.Printf("Step 2 not done, continuing with step 3...")
			return am.continueTraditionalLogin(baseURL, step2Resp.Data.SessionID)
		}
	}
	
	// Try to parse as final response with tokens
	var finalResp LoginStep4Response
	if err := json.Unmarshal(body, &finalResp); err == nil && finalResp.Data.Done {
		// Success - extract tokens
		am.config.BearerToken = finalResp.Data.Data.AccessToken.Token
		am.lastRefresh = time.Now()
		
		// Save tokens to .env file
		if err := am.config.SaveTokensToEnv(); err != nil {
			log.Printf("Warning: Failed to save tokens to .env: %v", err)
		}
		
		log.Println("Successfully completed traditional login with access token")
		return nil
	}
	
	return fmt.Errorf("traditional login completed but no valid token found")
}

// continueTraditionalLogin continues the traditional login flow with step 3
func (am *AuthManager) continueTraditionalLogin(baseURL, sessionID string) error {
	log.Println("Continuing traditional login with step 3...")
	
	// Step 3: Submit client_M
	req := LoginStep3Request{
		ClientM:   "5a7422b0be158fee81229ab62c8befc6030072acca621e94bacf8c2f3bc925a4",
		SessionID: sessionID,
	}
	
	_, body, err := am.makeLoginRequest(baseURL, req)
	if err != nil {
		return fmt.Errorf("traditional login step 3 failed: %w", err)
	}
	
	log.Printf("Traditional Login Step 3 Response: %s", string(body))
	
	// Try to parse as step3 response
	var step3Resp LoginStep3Response
	if err := json.Unmarshal(body, &step3Resp); err == nil {
		if step3Resp.Data.Done {
			// Login completed
			if sid, exists := am.sessionCookies["sid"]; exists && sid != "" {
				log.Printf("Found session cookie after step 3: %s", sid)
				am.config.BearerToken = sid
				am.lastRefresh = time.Now()
				
				// Save tokens to .env file
				if err := am.config.SaveTokensToEnv(); err != nil {
					log.Printf("Warning: Failed to save tokens to .env: %v", err)
				}
				
				log.Println("Successfully completed traditional login")
				return nil
			}
		} else if contains(step3Resp.Data.Require, "email_code") {
			// Need OTP - this shouldn't happen in traditional login
			log.Println("Traditional login requires OTP - this is unexpected")
			return fmt.Errorf("traditional login unexpectedly requires OTP")
		}
	}
	
	// Try to parse as final response with tokens
	var finalResp LoginStep4Response
	if err := json.Unmarshal(body, &finalResp); err == nil && finalResp.Data.Done {
		// Success - extract tokens
		am.config.BearerToken = finalResp.Data.Data.AccessToken.Token
		am.lastRefresh = time.Now()
		
		// Save tokens to .env file
		if err := am.config.SaveTokensToEnv(); err != nil {
			log.Printf("Warning: Failed to save tokens to .env: %v", err)
		}
		
		log.Println("Successfully completed traditional login with access token")
		return nil
	}
	
	return fmt.Errorf("traditional login step 3 completed but no valid token found")
}

// getPubkey gets the public key before login
func (am *AuthManager) getPubkey(deviceID, fpDid, clientID string) (interface{}, error) {
	pubkeyURL := fmt.Sprintf("https://gmgn.ai/account/pubkey?device_id=%s&fp_did=%s&client_id=%s&from_app=gmgn&app_ver=20251101-6461-0986672&tz_name=Asia%%2FJakarta&tz_offset=25200&app_lang=id&os=web&worker=0&fp_dfp=unknown", 
		deviceID, fpDid, clientID)
	
	// Use GET request for pubkey endpoint
	req, err := http.NewRequest("GET", pubkeyURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubkey request: %w", err)
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("sec-ch-ua", `"Chromium";v="130", "Google Chrome";v="130", "Not?A_Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Linux"`)
	req.Header.Set("Origin", "https://gmgn.ai")
	req.Header.Set("Referer", "https://gmgn.ai/login")
	
	// Add existing session cookies to request
	am.addCookiesToRequest(req)
	
	resp, err := am.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make pubkey request: %w", err)
	}
	defer resp.Body.Close()
	
	// Extract and store cookies from response
	am.extractCookiesFromResponse(resp)
	
	// Handle compressed responses
	var reader io.Reader = resp.Body
	encoding := resp.Header.Get("Content-Encoding")
	
	switch encoding {
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}
	
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read pubkey response: %w", err)
	}
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("pubkey request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	log.Printf("Pubkey Response: %s", string(body))
	
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse pubkey response: %w", err)
	}
	
	return result, nil
}

// initializeBrowserCookies sets up common browser cookies if not already present
func (am *AuthManager) initializeBrowserCookies() {
	// Initialize common browser cookies based on the screenshot
	if _, exists := am.sessionCookies["cf_bm"]; !exists {
		// Cloudflare bot management cookie (example value)
		am.sessionCookies["cf_bm"] = "FehRcXTzNJPouUyJnqQ_Gm"
	}
	
	if _, exists := am.sessionCookies["ga_UGRMV420"]; !exists {
		// Google Analytics cookie (example value)
		am.sessionCookies["ga_UGRMV420"] = "GS1.2.1762102308851803.3ef346561624f6216a3bbea"
	}
	
	if _, exists := am.sessionCookies["cf_clearance"]; !exists {
		// Cloudflare clearance cookie (example value)
		am.sessionCookies["cf_clearance"] = "BDReCJeqs_JnFPT9ghBKuI.fG"
	}
	
	// Session ID will be set during login flow
	log.Println("Initialized browser cookies for GMGN login")
}

// refreshFromBrowser attempts to get fresh session from browser automation
func (am *AuthManager) refreshFromBrowser() error {
	log.Println("Browser-based refresh not implemented yet")
	return fmt.Errorf("browser refresh not available, please update credentials manually")
}

// IsTokenExpired checks if current token is expired by making a test request
func (am *AuthManager) IsTokenExpired() bool {
	// If no token, consider it expired
	if am.config.BearerToken == "" {
		return true
	}
	
	req, err := http.NewRequest("GET", "https://gmgn.ai/api/v1/user/profile", nil)
	if err != nil {
		return true
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", am.config.BearerToken))
	req.Header.Set("Cookie", am.config.Cookies)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		log.Printf("Token validation request failed: %v", err)
		return true
	}
	defer resp.Body.Close()

	// Read response to check for specific error messages
	body, _ := io.ReadAll(resp.Body)
	log.Printf("Token validation response (status %d): %s", resp.StatusCode, string(body))

	return resp.StatusCode == 401
}

// GetValidToken returns a valid token, refreshing if necessary
func (am *AuthManager) GetValidToken() (string, error) {
	if am.IsTokenExpired() {
		if err := am.RefreshTokenIfNeeded(); err != nil {
			return "", err
		}
	}
	return am.config.BearerToken, nil
}

// GetValidCookies returns valid cookies, refreshing if necessary
func (am *AuthManager) GetValidCookies() (string, error) {
	if am.IsTokenExpired() {
		if err := am.RefreshTokenIfNeeded(); err != nil {
			return "", err
		}
	}
	return am.config.Cookies, nil
}

// extractTokensFromCookies extracts authentication tokens from session cookies
func (am *AuthManager) extractTokensFromCookies() error {
	log.Println("Extracting tokens from cookies...")
	
	// Look for common authentication cookies
	if sid, exists := am.sessionCookies["sid"]; exists && sid != "" {
		log.Printf("Found session cookie: %s", sid)
		am.config.BearerToken = sid
		am.lastRefresh = time.Now()
		
		// Save tokens to .env file
		if err := am.config.SaveTokensToEnv(); err != nil {
			log.Printf("Warning: Failed to save tokens to .env: %v", err)
		}
		
		log.Println("Successfully extracted token from session cookies")
		return nil
	}
	
	// Look for access_token cookie
	if accessToken, exists := am.sessionCookies["access_token"]; exists && accessToken != "" {
		log.Printf("Found access_token cookie: %s", accessToken)
		am.config.BearerToken = accessToken
		am.lastRefresh = time.Now()
		
		// Save tokens to .env file
		if err := am.config.SaveTokensToEnv(); err != nil {
			log.Printf("Warning: Failed to save tokens to .env: %v", err)
		}
		
		log.Println("Successfully extracted access token from cookies")
		return nil
	}
	
	log.Printf("Available cookies: %+v", am.sessionCookies)
	return fmt.Errorf("no authentication token found in cookies")
}

// solveCaptcha solves the captcha challenge
func (am *AuthManager) solveCaptcha(siteKey, keyType string) (string, error) {
	log.Printf("Captcha solving required - Site Key: %s, Type: %s", siteKey, keyType)
	
	// Check if we have a captcha solver API key
	captchaAPIKey := os.Getenv("CAPTCHA_API_KEY")
	if captchaAPIKey == "" {
		log.Println("No CAPTCHA_API_KEY found, falling back to manual solving")
		return am.solveCaptchaManually(siteKey, keyType)
	}
	
	// Use automatic captcha solving service
	return am.solveCaptchaAutomatically(siteKey, keyType, captchaAPIKey)
}

// solveCaptchaManually prompts user to manually solve captcha
func (am *AuthManager) solveCaptchaManually(siteKey, keyType string) (string, error) {
	fmt.Printf("Please solve the captcha manually:\n")
	fmt.Printf("Site Key: %s\n", siteKey)
	fmt.Printf("Key Type: %s\n", keyType)
	fmt.Printf("Visit: https://gmgn.ai/login and solve the captcha, then paste the captcha token here.\n")
	fmt.Print("Enter captcha token: ")
	
	reader := bufio.NewReader(os.Stdin)
	captchaToken, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read captcha token: %w", err)
	}
	
	captchaToken = strings.TrimSpace(captchaToken)
	if captchaToken == "" {
		return "", fmt.Errorf("empty captcha token provided")
	}
	
	log.Printf("Received captcha token (length: %d)", len(captchaToken))
	return captchaToken, nil
}

// solveCaptchaAutomatically uses captcha solving service
func (am *AuthManager) solveCaptchaAutomatically(siteKey, keyType, apiKey string) (string, error) {
	log.Printf("Using automatic captcha solving service...")
	
	// Create captcha solver client
	client := apisolvecaptcha.NewClient(apiKey)
	client.DefaultTimeout = 120
	client.RecaptchaTimeout = 600
	client.PollingInterval = 10
	
	var recaptcha apisolvecaptcha.ReCaptcha
	
	// Determine captcha type and configure accordingly
	switch keyType {
	case "score":
		// reCAPTCHA v3
		log.Printf("Solving reCAPTCHA v3 with site key: %s", siteKey)
		recaptcha = apisolvecaptcha.ReCaptcha{
			SiteKey: siteKey,
			Url:     "https://gmgn.ai/login",
			Version: "v3",
			Action:  "login",
			Score:   0.3,
		}
	case "widget":
		// reCAPTCHA v2
		log.Printf("Solving reCAPTCHA v2 with site key: %s", siteKey)
		recaptcha = apisolvecaptcha.ReCaptcha{
			SiteKey: siteKey,
			Url:     "https://gmgn.ai/login",
			Version: "v2",
		}
	default:
		// Default to reCAPTCHA v2
		log.Printf("Unknown key type '%s', defaulting to reCAPTCHA v2", keyType)
		recaptcha = apisolvecaptcha.ReCaptcha{
			SiteKey: siteKey,
			Url:     "https://gmgn.ai/login",
			Version: "v2",
		}
	}
	
	// Solve the captcha
	captchaToken, taskId, err := client.Solve(recaptcha.ToRequest())
	if err != nil {
		log.Printf("Automatic captcha solving failed: %v", err)
		log.Println("Falling back to manual solving...")
		return am.solveCaptchaManually(siteKey, keyType)
	}
	
	log.Printf("Successfully solved captcha automatically (task ID: %s, token length: %d)", taskId, len(captchaToken))
	return captchaToken, nil
}

// submitCaptchaSolution submits the captcha solution
func (am *AuthManager) submitCaptchaSolution(baseURL, sessionID, captchaToken string) (*LoginStep2Response, error) {
	log.Printf("Submitting captcha solution...")
	
	req := LoginStep2Request{
		CaptchaToken: captchaToken,
		SessionID:    sessionID,
	}
	
	_, body, err := am.makeLoginRequest(baseURL, req)
	if err != nil {
		return nil, err
	}
	
	log.Printf("Captcha Solution Response: %s", string(body))
	
	var result LoginStep2Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse captcha solution response: %w", err)
	}
	
	return &result, nil
}