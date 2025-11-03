package main

import (
	"log"
	"time"
)

// MonitorTokenHealth continuously monitors token health and rotates when needed
func (am *AuthManager) MonitorTokenHealth() {
	log.Println("Token health monitoring started")
	
	// Start persistent browser if not already running
	if !am.browserReady {
		log.Println("Starting persistent browser for token monitoring...")
		if err := am.StartPersistentBrowser(); err != nil {
			log.Printf("Failed to start persistent browser: %v", err)
			log.Println("Continuing without browser automation...")
		}
	}
	
	ticker := time.NewTicker(2 * time.Minute) // Check every 2 minutes for more responsive monitoring
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check if token is expired or will expire soon
			if am.IsTokenExpired() || am.isTokenExpiringSoon() {
				log.Println("Token expired, attempting refresh...")
				
				// Try browser-based refresh first if available
				if am.browserReady {
					log.Println("Attempting browser-based token refresh...")
					if err := am.RefreshTokenFromBrowser(); err == nil {
						log.Println("Token refreshed successfully from browser")
						
						// Send success notification
						successMsg := "🔄 *Token Auto-Refreshed from Browser*\n\n" +
							"Access token renewed automatically from persistent browser session."
						am.sendTelegramMessage(successMsg)
						continue
					} else {
						log.Printf("Browser refresh failed: %v", err)
					}
				}
				
				// Try refresh token if available
				if am.refreshToken != "" {
					if err := am.RefreshTokenUsingRefreshToken(); err != nil {
						log.Printf("Refresh token failed: %v", err)
						
						// Send notification about token refresh
						message := "⚠️ *Token Refresh Required*\n\n" +
							"Access token has expired and automatic refresh failed. " +
							"Please complete login in the browser window or via Telegram."
						am.sendTelegramMessage(message)
						
						// Attempt fallback refresh
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
					log.Println("No refresh token available, using fallback methods...")
					
					// Send notification about token refresh
					message := "⚠️ *Token Refresh Required*\n\n" +
						"Access token has expired. Please complete login in the browser window."
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
				
				// Periodically check browser for updated tokens even when current token is valid
				if am.browserReady {
					if token, err := am.GetTokenFromBrowser(); err == nil && token != "" && token != am.config.BearerToken {
						log.Printf("Found updated token in browser: %s", token)
						am.config.BearerToken = token
						am.lastRefresh = time.Now()
						
						// Save to .env file
						if err := am.config.SaveTokensToEnv(); err != nil {
							log.Printf("Warning: Failed to save tokens to .env: %v", err)
						}
						
						// Send notification
						updateMsg := "🔄 *Token Updated from Browser*\n\n" +
							"Found newer token in browser session and updated automatically."
						am.sendTelegramMessage(updateMsg)
					}
				}
			}
			
			// Update cookies from any recent requests
			am.updateCookiesFromRecentRequests()
		}
	}
}

// StartTwitterMonitoring starts continuous Twitter monitoring with token management
func (am *AuthManager) StartTwitterMonitoring() {
	log.Println("Starting Twitter monitoring with robust token management...")
	
	// Start persistent browser for automatic token management
	log.Println("Initializing persistent browser for seamless token management...")
	if err := am.StartPersistentBrowser(); err != nil {
		log.Printf("Failed to start persistent browser: %v", err)
		log.Println("Continuing with manual token management...")
	} else {
		log.Println("✅ Persistent browser started successfully")
		
		// Send notification about browser startup
		browserMsg := "🌐 *Browser Session Started*\n\n" +
			"Persistent browser session is now running for automatic token management. " +
			"The browser will handle token refresh automatically when needed."
		am.sendTelegramMessage(browserMsg)
	}
	
	// Start token health monitoring in a separate goroutine
	go am.MonitorTokenHealth()
	
	// Ensure we have a valid token before starting monitoring
	if am.IsTokenExpired() {
		log.Println("Initial token validation...")
		if err := am.RefreshTokenIfNeeded(); err != nil {
			log.Printf("Failed to get initial valid token: %v", err)
			
			// Send alert about initial token issue
			alertMsg := "⚠️ *Initial Token Issue*\n\n" +
				"Failed to obtain valid token for monitoring startup. " +
				"Please complete login in the browser window."
			am.sendTelegramMessage(alertMsg)
		}
	}
	
	// Main monitoring loop
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()
	
	// Cleanup browser on exit
	defer func() {
		log.Println("Shutting down monitoring...")
		am.StopPersistentBrowser()
	}()
	
	for {
		select {
		case <-ticker.C:
			log.Println("Checking for new Twitter messages...")
			
			// Ensure token is valid before making API calls
			if am.IsTokenExpired() {
				log.Println("Token expired during monitoring, refreshing...")
				if err := am.RefreshTokenIfNeeded(); err != nil {
					log.Printf("Failed to refresh token during monitoring: %v", err)
					
					// Send alert to Telegram
					alertMsg := "🚨 *Monitoring Alert*\n\n" +
						"Token refresh failed during Twitter monitoring. " +
						"Please check the browser window or complete login manually."
					am.sendTelegramMessage(alertMsg)
					
					// Continue monitoring even if token refresh fails
					continue
				}
			}
			
			// Here you would call the actual Twitter scraping logic
			// This is a placeholder for the integration
			log.Println("Token is valid, monitoring continues...")
		}
	}
}