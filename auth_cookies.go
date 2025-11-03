package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

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