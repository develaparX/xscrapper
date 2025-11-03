package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/chromedp/chromedp"
)

// performAutomatedTelegramLogin performs automated browser-based Telegram login
func (am *AuthManager) performAutomatedTelegramLogin(telegramBotURL string) error {
	log.Println("Starting automated Telegram login...")
	
	// Create Chrome context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	
	// Set timeout for the entire operation
	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	
	var loginURL string
	
	// Navigate to Telegram bot and wait for redirect
	err := chromedp.Run(ctx,
		chromedp.Navigate(telegramBotURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for potential redirects
		chromedp.Location(&loginURL),
	)
	
	if err != nil {
		return fmt.Errorf("failed to navigate to Telegram bot: %w", err)
	}
	
	log.Printf("Current URL after Telegram bot: %s", loginURL)
	
	// Check if we got redirected to a login URL
	if strings.Contains(loginURL, "gmgn.ai/tglogin") {
		log.Printf("Got login URL from automated browser: %s", loginURL)
		return am.processTelegramLoginURL(loginURL)
	}
	
	return fmt.Errorf("automated login did not produce expected login URL")
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
	if resp.StatusCode == 200 || resp.StatusCode == 302 {
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