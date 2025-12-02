package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// AuthManager handles authentication for GMGN API
type AuthManager struct {
	config         *Config
	httpClient     *http.Client
	lastRefresh    time.Time
	sessionCookies map[string]string
	refreshToken   string
	tokenExpiry    time.Time

	// Browser management
	browserCtx    context.Context
	browserCancel context.CancelFunc
	browserReady  bool
}

// NewAuthManager creates a new AuthManager instance
func NewAuthManager(config *Config) *AuthManager {
	return &AuthManager{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		sessionCookies: make(map[string]string),
		browserReady:   false,
	}
}

// IsBrowserReady returns true if the browser session is active
func (am *AuthManager) IsBrowserReady() bool {
	return am.browserReady
}

// NewAuthManagerWithBrowser creates a new AuthManager instance with browser initialization
func NewAuthManagerWithBrowser(config *Config) (*AuthManager, error) {
	am := NewAuthManager(config)

	// Initialize browser session
	if err := am.InitializeBrowserSession(); err != nil {
		log.Printf("Warning: Failed to initialize browser session: %v", err)
		log.Println("Continuing without browser automation...")
		return am, nil // Don't fail completely, just continue without browser
	}

	return am, nil
}

// RefreshTokenIfNeeded checks if token needs refresh and refreshes it
func (am *AuthManager) RefreshTokenIfNeeded() error {
	log.Println("Attempting to refresh authentication...")

	// Try browser-based refresh first if browser is available
	if am.browserReady {
		log.Println("Using persistent browser for token refresh...")
		if err := am.RefreshTokenFromBrowser(); err == nil {
			return nil
		} else {
			log.Printf("Browser refresh failed, falling back to manual Telegram login: %v", err)
		}
	}

	// Fallback to manual Telegram login
	return am.performTelegramLogin()
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

// StartPersistentBrowser starts a persistent browser session for token management
func (am *AuthManager) StartPersistentBrowser() error {
	if am.browserReady {
		log.Println("Browser already running")
		return nil
	}

	log.Println("Starting persistent browser session...")

	// Find available browser executable
	browserPath := am.findBrowserExecutable()
	if browserPath == "" {
		return fmt.Errorf("no supported browser found (tried: thorium-browser, google-chrome, chromium-browser, chromium)")
	}

	log.Printf("Using browser: %s", browserPath)

	// Create Chrome context with options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),   // Use detected browser
		chromedp.Flag("headless", false), // Keep browser visible
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserDataDir("./browser-data"), // Persistent user data
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"),
	)

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	am.browserCtx, am.browserCancel = chromedp.NewContext(allocCtx)

	// Navigate to GMGN.ai and wait for it to load
	err := chromedp.Run(am.browserCtx,
		chromedp.Navigate("https://gmgn.ai"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		return fmt.Errorf("failed to start persistent browser: %w", err)
	}

	am.browserReady = true
	log.Println("Persistent browser session started successfully")
	return nil
}

// StopPersistentBrowser stops the persistent browser session
func (am *AuthManager) StopPersistentBrowser() {
	if am.browserCancel != nil {
		log.Println("Stopping persistent browser session...")
		am.browserCancel()
		am.browserReady = false
	}
}

// GetTokenFromBrowser extracts token from browser localStorage
func (am *AuthManager) GetTokenFromBrowser() (string, error) {
	if !am.browserReady {
		return "", fmt.Errorf("browser not ready")
	}

	log.Println("Extracting token from browser localStorage...")

	var tgInfoStr string
	var allKeys []string

	// First, get all localStorage keys to see what's available
	err := chromedp.Run(am.browserCtx,
		chromedp.Evaluate(`Object.keys(localStorage)`, &allKeys),
	)

	if err != nil {
		return "", fmt.Errorf("failed to get localStorage keys: %w", err)
	}

	log.Printf("Available localStorage keys: %v", allKeys)

	// Try to get tgInfo first - this is the primary source
	err = chromedp.Run(am.browserCtx,
		chromedp.Evaluate(`localStorage.getItem('tgInfo')`, &tgInfoStr),
	)

	if err != nil {
		return "", fmt.Errorf("failed to get tgInfo from localStorage: %w", err)
	}

	if tgInfoStr != "" && tgInfoStr != "null" {
		log.Printf("Found tgInfo in localStorage")
		// Don't log the full token for security, just indicate we found it
		token := am.extractTokenFromLocalStorage(tgInfoStr)
		if token != "" {
			log.Printf("Successfully extracted token from tgInfo")
			return token, nil
		}
	}

	// Try to get token directly using JavaScript to access nested structure
	var directToken string
	err = chromedp.Run(am.browserCtx,
		chromedp.Evaluate(`
			try {
				const tgInfo = JSON.parse(localStorage.getItem('tgInfo') || '{}');
				if (tgInfo.token && tgInfo.token.access_token) {
					return tgInfo.token.access_token;
				}
				return null;
			} catch (e) {
				return null;
			}
		`, &directToken),
	)

	if err == nil && directToken != "" && directToken != "null" {
		log.Printf("Successfully extracted token directly from browser JavaScript")

		// Also try to get refresh token
		var refreshToken string
		chromedp.Run(am.browserCtx,
			chromedp.Evaluate(`
				try {
					const tgInfo = JSON.parse(localStorage.getItem('tgInfo') || '{}');
					if (tgInfo.token && tgInfo.token.refresh_token) {
						return tgInfo.token.refresh_token;
					}
					return null;
				} catch (e) {
					return null;
				}
			`, &refreshToken),
		)

		if refreshToken != "" && refreshToken != "null" {
			am.refreshToken = refreshToken
			log.Printf("Also extracted refresh token from browser")
		}

		return directToken, nil
	}

	// Fallback: Try other common token storage keys
	tokenKeys := []string{"auth", "token", "user", "session", "authToken", "accessToken"}

	for _, key := range tokenKeys {
		var keyValue string
		err = chromedp.Run(am.browserCtx,
			chromedp.Evaluate(fmt.Sprintf(`localStorage.getItem('%s')`, key), &keyValue),
		)

		if err != nil {
			log.Printf("Failed to get %s from localStorage: %v", key, err)
			continue
		}

		if keyValue != "" && keyValue != "null" {
			log.Printf("Found %s in localStorage", key)
			token := am.extractTokenFromLocalStorage(keyValue)
			if token != "" {
				return token, nil
			}
		}
	}

	return "", fmt.Errorf("no valid token found in browser localStorage")
}

// FetchWithBrowser fetches data using the persistent browser session
func (am *AuthManager) FetchWithBrowser(url string, headers map[string]string) ([]byte, error) {
	if !am.browserReady {
		return nil, fmt.Errorf("browser not ready")
	}

	// Reset result variable
	if err := chromedp.Run(am.browserCtx, chromedp.Evaluate(`window._fetchResult = null`, nil)); err != nil {
		return nil, fmt.Errorf("failed to reset fetch result: %w", err)
	}

	// Construct headers JS object
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal headers: %w", err)
	}

	// Script to perform fetch and save to window._fetchResult
	script := fmt.Sprintf(`
		(async () => {
			try {
				const controller = new AbortController();
				const id = setTimeout(() => controller.abort(), 30000);
				
				const response = await fetch("%s", {
					method: 'GET',
					headers: %s,
					signal: controller.signal
				});
				clearTimeout(id);
				
				const text = await response.text();
				window._fetchResult = JSON.stringify({status: response.status, body: text});
			} catch (e) {
				window._fetchResult = JSON.stringify({status: 0, body: e.toString()});
			}
		})()
	`, url, string(headersJSON))

	// Execute script (async)
	if err := chromedp.Run(am.browserCtx, chromedp.Evaluate(script, nil)); err != nil {
		return nil, fmt.Errorf("browser fetch execution failed: %w", err)
	}

	// Poll for result
	var resultJSON string
	err = chromedp.Run(am.browserCtx,
		chromedp.Poll(`window._fetchResult`, &resultJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to poll fetch result: %w", err)
	}

	// Parse result
	var result struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to parse browser fetch result: %w", err)
	}

	if result.Status != 200 {
		return nil, fmt.Errorf("browser fetch failed with status %d: %s", result.Status, result.Body)
	}

	return []byte(result.Body), nil
}

// GetCookiesFromBrowser extracts cookies from the browser session
func (am *AuthManager) GetCookiesFromBrowser() error {
	if !am.browserReady {
		return fmt.Errorf("browser not ready")
	}

	log.Println("Extracting cookies from browser...")

	var cookies []*network.Cookie
	err := chromedp.Run(am.browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)

	if err != nil {
		return fmt.Errorf("failed to get cookies from browser: %w", err)
	}

	var cookieParts []string
	for _, cookie := range cookies {
		// Store in session cookies map
		am.sessionCookies[cookie.Name] = cookie.Value
		cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))

		// Log important cookies (masked)
		if cookie.Name == "cf_clearance" {
			log.Printf("Found Cloudflare clearance cookie")
		}
	}

	if len(cookieParts) > 0 {
		am.config.Cookies = strings.Join(cookieParts, "; ")
		log.Printf("Extracted %d cookies from browser", len(cookies))
		return nil
	}

	return fmt.Errorf("no cookies found in browser")
}

// RefreshTokenFromBrowser attempts to refresh token using persistent browser
func (am *AuthManager) RefreshTokenFromBrowser() error {
	log.Println("Attempting to refresh token from persistent browser...")

	// Ensure browser is running
	if !am.browserReady {
		if err := am.StartPersistentBrowser(); err != nil {
			return fmt.Errorf("failed to start browser: %w", err)
		}
	}

	// Try to get token from current browser state
	token, err := am.GetTokenFromBrowser()
	if err == nil && token != "" {
		log.Printf("Successfully extracted token from browser: %s", token)
		am.config.BearerToken = token
		am.lastRefresh = time.Now()

		// Also get cookies
		if err := am.GetCookiesFromBrowser(); err != nil {
			log.Printf("Warning: Failed to get cookies from browser: %v", err)
		}

		// Save to .env file
		if err := am.config.SaveTokensToEnv(); err != nil {
			log.Printf("Warning: Failed to save tokens to .env: %v", err)
		}

		return nil
	}

	log.Printf("No valid token in browser, attempting Telegram login...")

	// Navigate to Telegram bot for fresh login
	telegramBotURL := "https://t.me/gmgnaibot?start=i__l_en_t_f858cac7964a6bf9"

	err = chromedp.Run(am.browserCtx,
		chromedp.Navigate(telegramBotURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(5*time.Second), // Wait for potential redirects
	)

	if err != nil {
		return fmt.Errorf("failed to navigate to Telegram bot: %w", err)
	}

	// Wait for user to complete login and check for token periodically
	log.Println("Waiting for user to complete Telegram login in browser...")
	log.Println("Please complete the login process in the browser window")

	// Poll for token every 5 seconds for up to 5 minutes
	timeout := time.Now().Add(5 * time.Minute)
	for time.Now().Before(timeout) {
		time.Sleep(5 * time.Second)

		// Check if we're back on gmgn.ai domain
		var currentURL string
		err = chromedp.Run(am.browserCtx,
			chromedp.Location(&currentURL),
		)

		if err != nil {
			log.Printf("Failed to get current URL: %v", err)
			continue
		}

		log.Printf("Current URL: %s", currentURL)

		// If we're back on gmgn.ai, try to extract token
		if strings.Contains(currentURL, "gmgn.ai") {
			token, err := am.GetTokenFromBrowser()
			if err == nil && token != "" {
				log.Printf("Successfully extracted token after login: %s", token)
				am.config.BearerToken = token
				am.lastRefresh = time.Now()

				// Also get cookies
				if err := am.GetCookiesFromBrowser(); err != nil {
					log.Printf("Warning: Failed to get cookies from browser: %v", err)
				}

				// Save to .env file
				if err := am.config.SaveTokensToEnv(); err != nil {
					log.Printf("Warning: Failed to save tokens to .env: %v", err)
				}

				// Send success notification
				successMsg := "✅ *Browser Token Refresh Successful!*\n\n" +
					"Token extracted from persistent browser session."
				am.sendTelegramMessage(successMsg)

				return nil
			}
		}
	}

	return fmt.Errorf("timeout waiting for login completion in browser")
}

// InitializeBrowserSession initializes browser session at startup
func (am *AuthManager) InitializeBrowserSession() error {
	log.Println("Initializing browser session for automatic token management...")

	if err := am.StartPersistentBrowser(); err != nil {
		return fmt.Errorf("failed to initialize browser session: %w", err)
	}

	// Try to get existing token from browser
	if token, err := am.GetTokenFromBrowser(); err == nil && token != "" {
		log.Printf("Found existing token in browser: %s", token)
		am.config.BearerToken = token
		am.lastRefresh = time.Now()

		// Also get cookies
		if err := am.GetCookiesFromBrowser(); err != nil {
			log.Printf("Warning: Failed to get cookies from browser: %v", err)
		}

		// Save to .env file
		if err := am.config.SaveTokensToEnv(); err != nil {
			log.Printf("Warning: Failed to save tokens to .env: %v", err)
		}

		log.Println("Successfully loaded token from browser session")
		return nil
	}

	log.Println("No existing token found in browser, will need login")
	return nil
}

// EnsureBrowserLogin ensures user is logged in via browser
func (am *AuthManager) EnsureBrowserLogin() error {
	if !am.browserReady {
		if err := am.StartPersistentBrowser(); err != nil {
			return fmt.Errorf("failed to start browser: %w", err)
		}
	}

	// Check if already logged in
	if token, err := am.GetTokenFromBrowser(); err == nil && token != "" {
		log.Println("Already logged in via browser")
		am.config.BearerToken = token
		am.lastRefresh = time.Now()

		// Also get cookies
		if err := am.GetCookiesFromBrowser(); err != nil {
			log.Printf("Warning: Failed to get cookies from browser: %v", err)
		}

		// Save to .env file
		if err := am.config.SaveTokensToEnv(); err != nil {
			log.Printf("Warning: Failed to save tokens to .env: %v", err)
		}

		return nil
	}

	// Navigate to login page
	log.Println("Navigating to GMGN.ai login page...")
	err := chromedp.Run(am.browserCtx,
		chromedp.Navigate("https://gmgn.ai"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)

	if err != nil {
		return fmt.Errorf("failed to navigate to login page: %w", err)
	}

	log.Println("Please complete login in the browser window")
	log.Println("The system will automatically detect when you're logged in")

	// Wait for login completion
	return am.waitForBrowserLogin()
}

// waitForBrowserLogin waits for user to complete login in browser
func (am *AuthManager) waitForBrowserLogin() error {
	log.Println("Waiting for login completion in browser...")

	timeout := time.Now().Add(10 * time.Minute) // 10 minute timeout
	checkInterval := 5 * time.Second

	for time.Now().Before(timeout) {
		time.Sleep(checkInterval)

		// Check for token in localStorage
		if token, err := am.GetTokenFromBrowser(); err == nil && token != "" {
			log.Printf("Login detected! Token found: %s", token)
			am.config.BearerToken = token
			am.lastRefresh = time.Now()

			// Also get cookies
			if err := am.GetCookiesFromBrowser(); err != nil {
				log.Printf("Warning: Failed to get cookies from browser: %v", err)
			}

			// Save to .env file
			if err := am.config.SaveTokensToEnv(); err != nil {
				log.Printf("Warning: Failed to save tokens to .env: %v", err)
			}

			// Send success notification
			successMsg := "✅ *Browser Login Successful!*\n\n" +
				"Successfully logged in via browser. Token management is now automated."
			am.sendTelegramMessage(successMsg)

			return nil
		}

		// Check current URL to see if we're on a logged-in page
		var currentURL string
		err := chromedp.Run(am.browserCtx,
			chromedp.Location(&currentURL),
		)

		if err == nil {
			log.Printf("Current URL: %s", currentURL)
		}
	}

	return fmt.Errorf("timeout waiting for browser login completion")
}

// findBrowserExecutable finds available browser executable
func (am *AuthManager) findBrowserExecutable() string {
	// List of browsers to try in order of preference
	browsers := []string{
		"thorium-browser",      // Thorium browser (your VPS)
		"google-chrome",        // Google Chrome
		"google-chrome-stable", // Google Chrome stable
		"chromium-browser",     // Chromium browser
		"chromium",             // Chromium
		"chrome",               // Generic chrome
	}

	for _, browser := range browsers {
		if path, err := exec.LookPath(browser); err == nil {
			log.Printf("Found browser executable: %s at %s", browser, path)
			return path
		}
	}

	log.Println("No supported browser found in PATH")
	return ""
}
