package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

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
	
	// Look for token object first (nested structure) - This is the correct structure
	if tokenObj, exists := tgInfo["token"]; exists {
		if tokenMap, ok := tokenObj.(map[string]interface{}); ok {
			log.Printf("Found token object in tgInfo: %+v", tokenMap)
			
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
	
	// Fallback: Look for direct access_token at root level
	if accessToken, exists := tgInfo["access_token"]; exists {
		if tokenStr, ok := accessToken.(string); ok && len(tokenStr) > 20 {
			log.Printf("Found direct access_token in tgInfo: %s", tokenStr)
			
			// Also look for refresh_token at root level
			if refreshToken, exists := tgInfo["refresh_token"]; exists {
				if refreshStr, ok := refreshToken.(string); ok && len(refreshStr) > 20 {
					am.refreshToken = refreshStr
					log.Printf("Found direct refresh_token: %s", refreshStr)
				}
			}
			
			return tokenStr
		}
	}
	
	// Fallback: Look for accessToken (camelCase)
	if accessToken, exists := tgInfo["accessToken"]; exists {
		if tokenStr, ok := accessToken.(string); ok && len(tokenStr) > 20 {
			log.Printf("Found accessToken in localStorage: %s", tokenStr)
			return tokenStr
		}
	}
	
	log.Printf("No valid token found in localStorage data")
	log.Printf("Available keys in tgInfo: %v", getKeys(tgInfo))
	return ""
}

// Helper function to get keys from map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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

// isTokenExpiringSoon checks if token will expire within the next 10 minutes
func (am *AuthManager) isTokenExpiringSoon() bool {
	if am.tokenExpiry.IsZero() {
		// If we don't have expiry info, assume it's not expiring soon
		return false
	}
	
	// Check if token expires within 10 minutes
	return time.Until(am.tokenExpiry) < 10*time.Minute
}

// ValidateTokenWithAPI validates token by making a test API call
func (am *AuthManager) ValidateTokenWithAPI() error {
	req, err := http.NewRequest("GET", "https://gmgn.ai/api/v1/user/profile", nil)
	if err != nil {
		return fmt.Errorf("failed to create validation request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", am.config.BearerToken))
	req.Header.Set("Cookie", am.config.Cookies)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("token is invalid or expired")
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("validation failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Println("Token validation successful")
	return nil
}