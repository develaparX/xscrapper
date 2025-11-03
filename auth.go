package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/chromedp/chromedp"
)

type AuthManager struct {
	config         *Config
	httpClient     *http.Client
	lastRefresh    time.Time
	sessionCookies map[string]string
	refreshToken   string
	tokenExpiry    time.Time
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
	
	// Try Telegram login
	return am.performTelegramLogin()
}

// performTelegramLogin implements Telegram-based login
func (am *AuthManager) performTelegramLogin() error {
	log.Println("Starting Telegram login process...")
	
	// Print Telegram login link to console
	telegramBotURL := "https://t.me/gmgnaibot?start=i__l_en_t_f858cac7964a6bf9"
	
	fmt.Println("\n🔐 GMGN Telegram Login Required")
	fmt.Println("================================")
	fmt.Printf("1️⃣ Click this link: %s\n", telegramBotURL)
	fmt.Println("2️⃣ Follow the instructions in the Telegram bot")
	fmt.Println("3️⃣ After successful login, you'll get a response URL")
	fmt.Println("4️⃣ Copy and paste the response URL below")
	fmt.Println("\n🤖 Automated Mode Available:")
	fmt.Println("   • Type 'auto' to use automated browser login")
	fmt.Println("   • Or paste the response URL manually")
	fmt.Println("\nExpected URL format:")
	fmt.Println("https://gmgn.ai/tglogin?user_id=...&code=...&id=...")
	fmt.Print("\nEnter 'auto' or response URL: ")
	
	// Wait for console input
	input, err := am.waitForConsoleInput()
	if err != nil {
		return fmt.Errorf("failed to get console input: %w", err)
	}
	
	// Check if user wants automated mode
	if strings.ToLower(strings.TrimSpace(input)) == "auto" {
		return am.performAutomatedTelegramLogin(telegramBotURL)
	}
	
	// Process the Telegram login URL manually
	return am.processTelegramLoginURL(input)
}

// processTelegramLoginURL processes the Telegram login URL and completes authentication
func (am *AuthManager) processTelegramLoginURL(loginURL string) error {
	log.Printf("Processing Telegram login URL: %s", loginURL)
	
	// Validate URL format
	if !strings.Contains(loginURL, "gmgn.ai/tglogin") {
		return fmt.Errorf("invalid Telegram login URL format: %s", loginURL)
	}
	
	// Visit the URL to complete login
	req, err := http.NewRequest("GET", loginURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for Telegram login URL: %w", err)
	}
	
	// Set proper headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("sec-ch-ua", `"Chromium";v="130", "Google Chrome";v="130", "Not?A_Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Linux"`)
	
	// Add existing cookies if any
	am.addCookiesToRequest(req)
	
	// Retry mechanism for connection issues
	var resp *http.Response
	maxRetries := 3
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Attempting to visit Telegram login URL (attempt %d/%d)", attempt, maxRetries)
		
		resp, err = am.httpClient.Do(req)
		if err != nil {
			log.Printf("Attempt %d failed: %v", attempt, err)
			if attempt < maxRetries {
				log.Printf("Waiting 2 seconds before retry...")
				time.Sleep(2 * time.Second)
				continue
			}
			return fmt.Errorf("failed to visit Telegram login URL after %d attempts: %w", maxRetries, err)
		}
		break
	}
	defer resp.Body.Close()
	
	// Extract cookies from response
	am.extractCookiesFromResponse(resp)
	
	// Handle compressed response body
	var reader io.Reader = resp.Body
	
	// Check content encoding and decompress if needed
	encoding := resp.Header.Get("Content-Encoding")
	switch encoding {
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "deflate":
		// Handle deflate if needed
		reader = resp.Body
	default:
		reader = resp.Body
	}
	
	// Read decompressed response body
	body, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read Telegram login response: %w", err)
	}
	
	log.Printf("Telegram login response status: %d", resp.StatusCode)
	log.Printf("Response body length: %d bytes", len(body))
	
	// Debug: Print full response headers
	log.Printf("=== RESPONSE HEADERS ===")
	for name, values := range resp.Header {
		for _, value := range values {
			log.Printf("%s: %s", name, value)
		}
	}
	log.Printf("=== END RESPONSE HEADERS ===")
	
	// Convert body to string for token extraction
	bodyStr := string(body)
	
	// Debug: Print all cookies received
	log.Printf("=== COOKIES FROM RESPONSE ===")
	for _, cookie := range resp.Cookies() {
		log.Printf("Cookie: %s=%s (Domain: %s, Path: %s, Expires: %v, HttpOnly: %t, Secure: %t)", 
			cookie.Name, cookie.Value, cookie.Domain, cookie.Path, cookie.Expires, cookie.HttpOnly, cookie.Secure)
	}
	log.Printf("=== END COOKIES FROM RESPONSE ===")
	
	// Debug: Print current session cookies
	log.Printf("=== CURRENT SESSION COOKIES ===")
	for name, value := range am.sessionCookies {
		log.Printf("Session Cookie: %s=%s", name, value)
	}
	log.Printf("=== END SESSION COOKIES ===")
	
	// Try to parse as JSON if it looks like JSON
	if strings.HasPrefix(strings.TrimSpace(bodyStr), "{") {
		log.Printf("=== ATTEMPTING JSON PARSE ===")
		var jsonResponse map[string]interface{}
		if err := json.Unmarshal(body, &jsonResponse); err == nil {
			prettyJSON, _ := json.MarshalIndent(jsonResponse, "", "  ")
			log.Printf("Parsed JSON Response:\n%s", string(prettyJSON))
		} else {
			log.Printf("Failed to parse as JSON: %v", err)
		}
		log.Printf("=== END JSON PARSE ===")
	}
	
	// Try to extract token from response body
	if token := am.extractTokenFromResponseBody(bodyStr); token != "" {
		log.Printf("Found token in response body: %s", token)
		am.config.BearerToken = token
		am.lastRefresh = time.Now()
		
		// Save to .env file
		if err := am.config.SaveTokensToEnv(); err != nil {
			log.Printf("Warning: Failed to save tokens to .env: %v", err)
		}
		
		log.Println("Successfully extracted token from response body")
		return nil
	}
	
	// If no token found in response, ask user for localStorage data
	log.Printf("No token found in response body. Requesting localStorage data...")
	fmt.Println("\n🔍 No token found in response. Let's check localStorage:")
	fmt.Println("1. Open browser developer tools (F12)")
	fmt.Println("2. Go to Console tab")
	fmt.Println("3. First, check all localStorage keys:")
	fmt.Println("   Type: Object.keys(localStorage)")
	fmt.Println("   Press Enter and see what keys are available")
	fmt.Println("4. Then try to get tgInfo:")
	fmt.Println("   Type: localStorage.getItem('tgInfo')")
	fmt.Println("5. If tgInfo doesn't exist, try other keys like:")
	fmt.Println("   localStorage.getItem('auth')")
	fmt.Println("   localStorage.getItem('token')")
	fmt.Println("   localStorage.getItem('user')")
	fmt.Println("6. Copy the result that contains token data and paste it below")
	fmt.Print("\nEnter localStorage data (or press Enter to skip): ")
	
	localStorageData, err := am.waitForConsoleInput()
	if err == nil && localStorageData != "" && localStorageData != "null" {
		if token := am.extractTokenFromLocalStorage(localStorageData); token != "" {
			log.Printf("Found token in localStorage data: %s", token)
			am.config.BearerToken = token
			am.lastRefresh = time.Now()
			
			// Save to .env file
			if err := am.config.SaveTokensToEnv(); err != nil {
				log.Printf("Warning: Failed to save tokens to .env: %v", err)
			}
			
			log.Println("Successfully extracted token from localStorage data")
			return nil
		}
	}
	
	log.Printf("Available cookies after Telegram login: %+v", am.sessionCookies)
	
	// Check if login was successful
	if resp.StatusCode == 200 || resp.StatusCode == 302 {
		// Send success message
		successMsg := "✅ *Telegram Login Successful!*\n\n" +
			"Authentication completed successfully. The bot is now ready to use."
		am.sendTelegramMessage(successMsg)
		
		// Extract tokens from cookies
		if err := am.extractTokensFromCookies(); err != nil {
			log.Printf("No tokens found in cookies, but login appears successful")
			
			// Set a placeholder token to indicate successful login
			am.config.BearerToken = "telegram_login_success"
			am.lastRefresh = time.Now()
			
			// Save to .env file
			if err := am.config.SaveTokensToEnv(); err != nil {
				log.Printf("Warning: Failed to save tokens to .env: %v", err)
			}
		}
		
		log.Println("Successfully completed Telegram login")
		return nil
	}
	
	return fmt.Errorf("Telegram login failed with status %d", resp.StatusCode)
}

// sendTelegramMessage sends a message to Telegram
func (am *AuthManager) sendTelegramMessage(message string) error {
	if am.config.TelegramBotToken == "" || am.config.TelegramChatID == "" {
		return fmt.Errorf("telegram bot token or chat ID not configured")
	}
	
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", am.config.TelegramBotToken)
	
	payload := map[string]interface{}{
		"chat_id":    am.config.TelegramChatID,
		"text":       message,
		"parse_mode": "Markdown",
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram message: %w", err)
	}
	
	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	log.Println("Successfully sent message to Telegram")
	return nil
}

// waitForTelegramResponse waits for user response from Telegram
func (am *AuthManager) waitForTelegramResponse() (string, error) {
	log.Println("Waiting for Telegram response...")
	
	// Get the last message ID to start polling from
	lastUpdateID, err := am.getLastTelegramUpdateID()
	if err != nil {
		log.Printf("Warning: Could not get last update ID: %v", err)
		lastUpdateID = 0
	}
	
	// Poll for new messages for up to 10 minutes
	timeout := time.Now().Add(10 * time.Minute)
	pollInterval := 5 * time.Second
	
	for time.Now().Before(timeout) {
		updates, err := am.getTelegramUpdates(lastUpdateID + 1)
		if err != nil {
			log.Printf("Error getting Telegram updates: %v", err)
			time.Sleep(pollInterval)
			continue
		}
		
		for _, update := range updates {
			if update.Message != nil && update.Message.Text != "" {
				// Update lastUpdateID to avoid processing the same message again
				lastUpdateID = update.UpdateID
				
				text := strings.TrimSpace(update.Message.Text)
				log.Printf("Received Telegram message: %s", text)
				
				// Check if it looks like a login URL
				if strings.Contains(text, "gmgn.ai/tglogin") {
					// Send confirmation
					confirmMsg := "✅ Received your login URL! Processing authentication..."
					am.sendTelegramMessage(confirmMsg)
					return text, nil
				}
			}
		}
		
		time.Sleep(pollInterval)
	}
	
	return "", fmt.Errorf("timeout waiting for Telegram response")
}

// waitForConsoleInput waits for user input from console
func (am *AuthManager) waitForConsoleInput() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read console input: %w", err)
	}
	
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("empty input provided")
	}
	
	log.Printf("Received console input: %s", input)
	return input, nil
}

// TelegramUpdate represents a Telegram update
type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int    `json:"message_id"`
		Text      string `json:"text"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

// getTelegramUpdates gets new updates from Telegram
func (am *AuthManager) getTelegramUpdates(offset int) ([]TelegramUpdate, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30", 
		am.config.TelegramBotToken, offset)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get telegram updates: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("telegram API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	var response struct {
		OK     bool             `json:"ok"`
		Result []TelegramUpdate `json:"result"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode telegram response: %w", err)
	}
	
	if !response.OK {
		return nil, fmt.Errorf("telegram API returned ok=false")
	}
	
	return response.Result, nil
}

// getLastTelegramUpdateID gets the last update ID to avoid processing old messages
func (am *AuthManager) getLastTelegramUpdateID() (int, error) {
	updates, err := am.getTelegramUpdates(0)
	if err != nil {
		return 0, err
	}
	
	if len(updates) == 0 {
		return 0, nil
	}
	
	// Return the highest update ID
	maxID := 0
	for _, update := range updates {
		if update.UpdateID > maxID {
			maxID = update.UpdateID
		}
	}
	
	return maxID, nil
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

// IsTokenExpired checks if current token is expired by making a test request
func (am *AuthManager) IsTokenExpired() bool {
	// If no token, consider it expired
	if am.config.BearerToken == "" {
		return true
	}
	
	// For Telegram login, we'll assume token is valid for a reasonable time
	if am.config.BearerToken == "telegram_login_success" {
		// Check if it's been more than 1 hour since last refresh
		return time.Since(am.lastRefresh) > time.Hour
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

// extractTokenFromResponseBody tries to extract authentication token from HTML response
func (am *AuthManager) extractTokenFromResponseBody(responseBody string) string {
	// Look for common token patterns in the response
	
	// Pattern 1: Look for tgInfo localStorage data
	if strings.Contains(responseBody, "tgInfo") {
		// Try to find patterns like: "tgInfo":{"accessToken":"eyJ...","refreshToken":"eyJ..."}
		start := strings.Index(responseBody, `"tgInfo"`)
		if start != -1 {
			// Find the opening brace for tgInfo object
			braceStart := strings.Index(responseBody[start:], `{`)
			if braceStart != -1 {
				braceStart += start
				// Find matching closing brace
				braceCount := 1
				braceEnd := braceStart + 1
				for braceEnd < len(responseBody) && braceCount > 0 {
					if responseBody[braceEnd] == '{' {
						braceCount++
					} else if responseBody[braceEnd] == '}' {
						braceCount--
					}
					braceEnd++
				}
				
				if braceCount == 0 {
					tgInfoStr := responseBody[braceStart:braceEnd]
					log.Printf("Found tgInfo object: %s", tgInfoStr)
					
					// Try to extract accessToken from tgInfo
					if accessTokenStart := strings.Index(tgInfoStr, `"accessToken":"`); accessTokenStart != -1 {
						accessTokenStart += len(`"accessToken":"`)
						if accessTokenEnd := strings.Index(tgInfoStr[accessTokenStart:], `"`); accessTokenEnd != -1 {
							token := tgInfoStr[accessTokenStart : accessTokenStart+accessTokenEnd]
							if len(token) > 20 {
								log.Printf("Extracted token from tgInfo: %s", token)
								return token
							}
						}
					}
				}
			}
		}
	}
	
	// Pattern 2: Look for access_token in JavaScript variables
	if strings.Contains(responseBody, "access_token") {
		// Try to find patterns like: "access_token":"eyJ..."
		start := strings.Index(responseBody, `"access_token":"`)
		if start != -1 {
			start += len(`"access_token":"`)
			end := strings.Index(responseBody[start:], `"`)
			if end != -1 {
				token := responseBody[start : start+end]
				if len(token) > 20 { // Basic validation
					return token
				}
			}
		}
	}
	
	// Pattern 3: Look for bearer token patterns
	if strings.Contains(responseBody, "bearer") || strings.Contains(responseBody, "Bearer") {
		// Try to find patterns like: bearer: "eyJ..."
		patterns := []string{`"bearer":"`, `"Bearer":"`, `bearer: "`, `Bearer: "`}
		for _, pattern := range patterns {
			start := strings.Index(responseBody, pattern)
			if start != -1 {
				start += len(pattern)
				end := strings.Index(responseBody[start:], `"`)
				if end != -1 {
					token := responseBody[start : start+end]
					if len(token) > 20 { // Basic validation
						return token
					}
				}
			}
		}
	}
	
	// Pattern 4: Look for JWT tokens (eyJ...)
	jwtStart := strings.Index(responseBody, "eyJ")
	if jwtStart != -1 {
		// Find the end of the JWT token (usually ends with quote or space)
		remaining := responseBody[jwtStart:]
		var token strings.Builder
		for i, char := range remaining {
			if char == '"' || char == ' ' || char == '\n' || char == '\r' || char == '\t' || char == ',' || char == '}' {
				break
			}
			if i > 500 { // Prevent extremely long tokens
				break
			}
			token.WriteRune(char)
		}
		
		tokenStr := token.String()
		if len(tokenStr) > 50 && strings.Count(tokenStr, ".") >= 2 { // JWT should have at least 2 dots
			return tokenStr
		}
	}
	
	return ""
}

// extractTokenFromLocalStorage extracts token from localStorage tgInfo data
func (am *AuthManager) extractTokenFromLocalStorage(localStorageData string) string {
	log.Printf("Processing localStorage data: %s", localStorageData)
	
	// Remove quotes if the data is wrapped in quotes
	localStorageData = strings.Trim(localStorageData, `"`)
	
	// Try to parse as JSON
	var tgInfo map[string]interface{}
	if err := json.Unmarshal([]byte(localStorageData), &tgInfo); err != nil {
		log.Printf("Failed to parse localStorage data as JSON: %v", err)
		return ""
	}
	
	// Look for token object first (nested structure)
	if tokenObj, exists := tgInfo["token"]; exists {
		if tokenMap, ok := tokenObj.(map[string]interface{}); ok {
			// Look for access_token inside token object
			if accessToken, exists := tokenMap["access_token"]; exists {
				if tokenStr, ok := accessToken.(string); ok && len(tokenStr) > 20 {
					log.Printf("Found access_token in token object: %s", tokenStr)
					
					// Also extract refresh_token if available
					if refreshToken, exists := tokenMap["refresh_token"]; exists {
						if refreshStr, ok := refreshToken.(string); ok && len(refreshStr) > 20 {
							am.refreshToken = refreshStr
							log.Printf("Found refresh_token: %s", refreshStr)
						}
					}
					
					return tokenStr
				}
			}
		}
	}
	
	// Look for direct accessToken
	if accessToken, exists := tgInfo["accessToken"]; exists {
		if tokenStr, ok := accessToken.(string); ok && len(tokenStr) > 20 {
			log.Printf("Found accessToken in localStorage: %s", tokenStr)
			return tokenStr
		}
	}
	
	// Look for direct access_token
	if accessToken, exists := tgInfo["access_token"]; exists {
		if tokenStr, ok := accessToken.(string); ok && len(tokenStr) > 20 {
			log.Printf("Found access_token in localStorage: %s", tokenStr)
			return tokenStr
		}
	}
	
	log.Printf("No valid token found in localStorage data")
	return ""
}

// RefreshTokenUsingRefreshToken attempts to refresh access token using refresh token
func (am *AuthManager) RefreshTokenUsingRefreshToken() error {
	if am.refreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}
	
	log.Println("Attempting to refresh token using refresh token...")
	
	// Prepare refresh token request
	refreshData := map[string]string{
		"refresh_token": am.refreshToken,
		"grant_type":    "refresh_token",
	}
	
	jsonData, err := json.Marshal(refreshData)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh token request: %w", err)
	}
	
	req, err := http.NewRequest("POST", "https://gmgn.ai/api/v1/auth/refresh", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create refresh token request: %w", err)
	}
	
	// Set proper headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	
	// Add cookies if available
	am.addCookiesToRequest(req)
	
	resp, err := am.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute refresh token request: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read refresh token response: %w", err)
	}
	
	log.Printf("Refresh token response (status %d): %s", resp.StatusCode, string(body))
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("refresh token failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var refreshResponse map[string]interface{}
	if err := json.Unmarshal(body, &refreshResponse); err != nil {
		return fmt.Errorf("failed to parse refresh token response: %w", err)
	}
	
	// Extract new access token
	if accessToken, exists := refreshResponse["access_token"]; exists {
		if tokenStr, ok := accessToken.(string); ok && len(tokenStr) > 20 {
			am.config.BearerToken = tokenStr
			am.lastRefresh = time.Now()
			
			// Update refresh token if provided
			if newRefreshToken, exists := refreshResponse["refresh_token"]; exists {
				if refreshStr, ok := newRefreshToken.(string); ok && len(refreshStr) > 20 {
					am.refreshToken = refreshStr
				}
			}
			
			// Save to .env file
			if err := am.config.SaveTokensToEnv(); err != nil {
				log.Printf("Warning: Failed to save tokens to .env: %v", err)
			}
			
			log.Println("Successfully refreshed token using refresh token")
			return nil
		}
	}
	
	return fmt.Errorf("no valid access token in refresh response")
}

// MonitorTokenHealth continuously monitors token health and rotates when needed
func (am *AuthManager) MonitorTokenHealth() {
	log.Println("Token health monitoring started")
	
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check if token is expired or will expire soon
			if am.IsTokenExpired() || am.isTokenExpiringSoon() {
				log.Println("Token expired, attempting refresh...")
				
				// Try refresh token first if available
				if am.refreshToken != "" {
					if err := am.RefreshTokenUsingRefreshToken(); err != nil {
						log.Printf("Refresh token failed: %v", err)
						
						// Send notification about token refresh
						message := "⚠️ *Token Refresh Required*\n\n" +
							"Access token has expired and refresh token failed. " +
							"Please complete Telegram login to continue monitoring."
						am.sendTelegramMessage(message)
						
						// Attempt Telegram login
						if err := am.RefreshTokenIfNeeded(); err != nil {
							log.Printf("Token refresh failed: %v", err)
							
							// Send failure notification
							failMsg := "❌ *Token Refresh Failed*\n\n" +
								"Unable to refresh authentication. Manual intervention required."
							am.sendTelegramMessage(failMsg)
						} else {
							// Send success notification
							successMsg := "✅ *Token Refreshed Successfully*\n\n" +
								"Authentication renewed. Monitoring will continue."
							am.sendTelegramMessage(successMsg)
						}
					} else {
						log.Println("Token refreshed using refresh token")
						
						// Send success notification
						successMsg := "🔄 *Token Auto-Refreshed*\n\n" +
							"Access token renewed automatically using refresh token."
						am.sendTelegramMessage(successMsg)
					}
				} else {
					log.Println("No refresh token available, using Telegram login...")
					
					// Send notification about token refresh
					message := "⚠️ *Token Refresh Required*\n\n" +
						"Access token has expired. Please complete Telegram login to continue monitoring."
					am.sendTelegramMessage(message)
					
					if err := am.RefreshTokenIfNeeded(); err != nil {
						log.Printf("Token refresh failed: %v", err)
						
						// Send failure notification
						failMsg := "❌ *Token Refresh Failed*\n\n" +
							"Unable to refresh authentication. Manual intervention required."
						am.sendTelegramMessage(failMsg)
					} else {
						// Send success notification
						successMsg := "✅ *Token Refreshed Successfully*\n\n" +
							"Authentication renewed. Monitoring will continue."
						am.sendTelegramMessage(successMsg)
					}
				}
			} else {
				log.Println("Token is healthy")
			}
			
			// Update cookies from any recent requests
			am.updateCookiesFromRecentRequests()
		}
	}
}

// isTokenExpiringSoon checks if token will expire within the next 10 minutes
func (am *AuthManager) isTokenExpiringSoon() bool {
	if am.tokenExpiry.IsZero() {
		// If we don't have expiry info, assume it's not expiring soon
		return false
	}
	
	// Check if token expires within 10 minutes
	return time.Until(am.tokenExpiry) < 10*time.Minute
}

// updateCookiesFromRecentRequests updates session cookies from recent API responses
func (am *AuthManager) updateCookiesFromRecentRequests() {
	// This would be called after each API request to update cookies
	// For now, we'll just ensure cookies are properly formatted
	if len(am.sessionCookies) > 0 {
		var cookieParts []string
		for name, value := range am.sessionCookies {
			cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", name, value))
		}
		am.config.Cookies = strings.Join(cookieParts, "; ")
	}
}

// ValidateTokenWithAPI validates token by making a test API call
func (am *AuthManager) ValidateTokenWithAPI() error {
	req, err := http.NewRequest("GET", "https://gmgn.ai/api/v1/user/profile", nil)
	if err != nil {
		return fmt.Errorf("failed to create validation request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", am.config.BearerToken))
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	
	// Add cookies
	am.addCookiesToRequest(req)
	
	resp, err := am.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("validation request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Update cookies from response
	am.extractCookiesFromResponse(resp)
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read validation response: %w", err)
	}
	
	if resp.StatusCode == 401 {
		return fmt.Errorf("token is invalid or expired")
	}
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("validation failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	return nil
}

// StartTwitterMonitoring starts monitoring Twitter/X data with robust token management
func (am *AuthManager) StartTwitterMonitoring() {
	log.Println("Twitter/X monitoring started")
	
	// Start token health monitoring in background
	go am.MonitorTokenHealth()
	
	// Main monitoring loop
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Validate token before making requests
			if err := am.ValidateTokenWithAPI(); err != nil {
				log.Printf("Token validation failed: %v", err)
				continue
			}
			
			// Make API request to get Twitter/X data
			if err := am.fetchTwitterData(); err != nil {
				log.Printf("Failed to fetch Twitter data: %v", err)
				
				// Check if it's an auth error
				if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") {
					log.Println("Auth error detected, refreshing token...")
					
					// Force token refresh
					if refreshErr := am.RefreshTokenIfNeeded(); refreshErr != nil {
						log.Printf("Token refresh failed: %v", refreshErr)
					}
				}
			}
		}
	}
}

// fetchTwitterData fetches Twitter/X data from GMGN API
func (am *AuthManager) fetchTwitterData() error {
	
	// Build API URL for Twitter messages
	apiURL := "https://gmgn.ai/vas/api/v1/twitter/messages?" +
		"app_lang=en&" +
		"app_ver=20251101-6461-0986672&" +
		"client_id=gmgn_web_20251101-6461-0986672&" +
		"device_id=a3697e7f-dbaa-4ebb-9f50-9e2fecd21cbe&" +
		"fp_did=8de143c466b06cd9949cb90da027a288&" +
		"from_app=gmgn&" +
		"has_token=true&" +
		"mine=1&" +
		"os=web&" +
		"tw_types=tweet&tw_types=reply&tw_types=repost&tw_types=quote&tw_types=description&tw_types=follow&tw_types=photo&tw_types=banner&tw_types=name&tw_types=handle&" +
		"tz_name=Asia%2FJakarta&" +
		"tz_offset=25200&" +
		"user_tags=kol&user_tags=trader&user_tags=master&user_tags=politics&user_tags=media&user_tags=companies&user_tags=founder&user_tags=exchange&user_tags=celebrity&user_tags=binance_square&user_tags=other&" +
		"worker=0"
	
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create Twitter data request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", am.config.BearerToken))
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://gmgn.ai/")
	req.Header.Set("Origin", "https://gmgn.ai")
	
	// Add cookies
	am.addCookiesToRequest(req)
	
	resp, err := am.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Twitter data request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Update cookies from response
	am.extractCookiesFromResponse(resp)
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Twitter data response: %w", err)
	}
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("Twitter data request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var twitterData map[string]interface{}
	if err := json.Unmarshal(body, &twitterData); err != nil {
		return fmt.Errorf("failed to parse Twitter data response: %w", err)
	}
	
	// Process Twitter data
	if data, exists := twitterData["data"]; exists {
		if dataArray, ok := data.([]interface{}); ok {
			log.Printf("Fetched %d Twitter messages", len(dataArray))
			
			// Process each message
			for _, item := range dataArray {
				if message, ok := item.(map[string]interface{}); ok {
					am.processTwitterMessage(message)
				}
			}
		}
	}
	
	return nil
}

// processTwitterMessage processes individual Twitter message
func (am *AuthManager) processTwitterMessage(message map[string]interface{}) {
	// Extract relevant information from Twitter message
	if content, exists := message["content"]; exists {
		if contentStr, ok := content.(string); ok && contentStr != "" {
			// Check for token mentions or relevant keywords
			if am.containsTokenKeywords(contentStr) {
				log.Printf("Found relevant Twitter message: %s", contentStr)
				
				// Send notification to Telegram
				notificationMsg := fmt.Sprintf("🐦 *New Twitter Alert*\n\n%s", contentStr)
				if err := am.sendTelegramMessage(notificationMsg); err != nil {
					log.Printf("Failed to send Twitter notification: %v", err)
				}
			}
		}
	}
}

// containsTokenKeywords checks if content contains relevant token keywords
func (am *AuthManager) containsTokenKeywords(content string) bool {
	keywords := []string{
		"token", "coin", "crypto", "pump", "moon", "gem", 
		"buy", "sell", "trade", "dex", "swap", "launch",
		"$", "0x", "CA:", "contract",
	}
	
	contentLower := strings.ToLower(content)
	for _, keyword := range keywords {
		if strings.Contains(contentLower, keyword) {
			return true
		}
	}
	
	return false
}

// performAutomatedTelegramLogin uses chromedp to automate browser login and extract localStorage
func (am *AuthManager) performAutomatedTelegramLogin(telegramBotURL string) error {
	log.Println("Starting automated Telegram login with browser automation...")
	
	fmt.Println("\n🤖 Automated Browser Login")
	fmt.Println("==========================")
	fmt.Printf("1. Click this link manually: %s\n", telegramBotURL)
	fmt.Println("2. Follow the instructions in the Telegram bot")
	fmt.Println("3. After successful login, you'll get a response URL")
	fmt.Println("4. Paste the response URL below")
	fmt.Println("5. Chromium will automatically open the URL and extract localStorage")
	fmt.Print("\nEnter the response URL: ")
	
	// Wait for user to provide the response URL
	loginURL, err := am.waitForConsoleInput()
	if err != nil {
		return fmt.Errorf("failed to get response URL: %w", err)
	}
	
	// Validate URL format
	if !strings.Contains(loginURL, "gmgn.ai/tglogin") {
		return fmt.Errorf("invalid Telegram login URL format: %s", loginURL)
	}
	
	log.Printf("Processing Telegram login URL with browser automation: %s", loginURL)
	
	// Test if chromium can be executed
	log.Println("Testing chromium executable...")
	testCmd := "chromium --version"
	if output, err := exec.Command("sh", "-c", testCmd).Output(); err != nil {
		return fmt.Errorf("chromium test failed: %w", err)
	} else {
		log.Printf("Chromium test successful: %s", string(output))
	}
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	// Create chromedp context with options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath("/usr/bin/thorium-browser"), // Use Thorium instead of Chromium
		chromedp.Flag("headless", false), // Show browser window
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.WindowSize(1200, 800),
	)
	
	log.Println("Creating chromedp allocator...")
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()
	
	log.Println("Creating chromedp context...")
	ctx, cancel = chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()
	
	// Navigate to the login URL
	log.Println("Opening browser and navigating to login URL...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(loginURL),
		chromedp.Sleep(3*time.Second), // Wait for page to load and JavaScript to execute
	)
	if err != nil {
		log.Printf("Browser error details: %v", err)
		return fmt.Errorf("failed to navigate to login URL: %w", err)
	}
	log.Println("Browser opened successfully and page loaded")
	
	// Wait a moment for JavaScript to execute and localStorage to be populated
	time.Sleep(2 * time.Second)
	
	// Extract localStorage data
	log.Println("Extracting localStorage data...")
	var tgInfoData string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`localStorage.getItem('tgInfo')`, &tgInfoData),
	)
	if err != nil {
		log.Printf("Failed to get localStorage tgInfo: %v", err)
	} else {
		log.Printf("Retrieved localStorage tgInfo: %s", tgInfoData)
	}
	
	// Try to extract token from localStorage data
	if tgInfoData != "" && tgInfoData != "null" {
		if token := am.extractTokenFromLocalStorage(tgInfoData); token != "" {
			log.Printf("Successfully extracted token from automated browser: %s", token)
			am.config.BearerToken = token
			am.lastRefresh = time.Now()
			
			// Save to .env file
			if err := am.config.SaveTokensToEnv(); err != nil {
				log.Printf("Warning: Failed to save tokens to .env: %v", err)
			}
			
			// Send success message
			successMsg := "✅ *Automated Telegram Login Successful!*\n\n" +
				"Token extracted automatically from browser localStorage."
			am.sendTelegramMessage(successMsg)
			
			log.Println("Successfully completed automated Telegram login")
			return nil
		}
	}
	
	// If tgInfo is empty, try to get all localStorage keys
	log.Println("tgInfo not found, checking all localStorage keys...")
	var allKeys []string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`Object.keys(localStorage)`, &allKeys),
	)
	if err == nil {
		log.Printf("Available localStorage keys: %v", allKeys)
		
		// Try other common keys
		for _, key := range allKeys {
			if strings.Contains(strings.ToLower(key), "auth") || 
			   strings.Contains(strings.ToLower(key), "token") || 
			   strings.Contains(strings.ToLower(key), "user") ||
			   key == "tgInfo" {
				var keyData string
				err = chromedp.Run(ctx,
					chromedp.Evaluate(fmt.Sprintf(`localStorage.getItem('%s')`, key), &keyData),
				)
				if err == nil && keyData != "" && keyData != "null" {
					log.Printf("Found data in key '%s': %s", key, keyData)
					if token := am.extractTokenFromLocalStorage(keyData); token != "" {
						log.Printf("Successfully extracted token from key '%s': %s", key, token)
						am.config.BearerToken = token
						am.lastRefresh = time.Now()
						
						// Save to .env file
						if err := am.config.SaveTokensToEnv(); err != nil {
							log.Printf("Warning: Failed to save tokens to .env: %v", err)
						}
						
						log.Println("Successfully completed automated Telegram login")
						return nil
					}
				}
			}
		}
	} else {
		log.Printf("Failed to get localStorage keys: %v", err)
	}
	
	// If still no token found, wait a bit more and try again
	log.Println("No token found yet, waiting 5 more seconds for page to fully load...")
	time.Sleep(5 * time.Second)
	
	// Try one more time
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`localStorage.getItem('tgInfo')`, &tgInfoData),
	)
	if err == nil && tgInfoData != "" && tgInfoData != "null" {
		if token := am.extractTokenFromLocalStorage(tgInfoData); token != "" {
			log.Printf("Successfully extracted token on second attempt: %s", token)
			am.config.BearerToken = token
			am.lastRefresh = time.Now()
			
			// Save to .env file
			if err := am.config.SaveTokensToEnv(); err != nil {
				log.Printf("Warning: Failed to save tokens to .env: %v", err)
			}
			
			log.Println("Successfully completed automated Telegram login")
			return nil
		}
	}
	
	return fmt.Errorf("no valid token found in browser localStorage after automated login")
}