package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/andybalholm/brotli"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type GMGNScraper struct {
	config     *Config
	httpClient *http.Client
}

func NewGMGNScraper(config *Config) *GMGNScraper {
	return &GMGNScraper{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// GetTwitterMessages fetches Twitter messages from GMGN API
func (s *GMGNScraper) GetTwitterMessages() (*TwitterResponse, error) {
	params := url.Values{}
	params.Add("has_token", "false")
	params.Add("mine", "1")
	params.Add("device_id", s.config.DeviceID)
	params.Add("fp_did", s.config.FingerprintID)
	params.Add("client_id", s.config.ClientID)
	params.Add("from_app", "gmgn")
	params.Add("app_ver", s.config.AppVersion)
	params.Add("tz_name", "Asia/Jakarta")
	params.Add("tz_offset", "25200")
	params.Add("app_lang", "id")
	params.Add("os", "web")
	params.Add("worker", "0")

	// Add user tags
	for _, tag := range s.config.UserTags {
		params.Add("user_tags", tag)
	}

	// Add tweet types
	for _, twType := range s.config.TweetTypes {
		params.Add("tw_types", twType)
	}

	apiURL := fmt.Sprintf("%s?%s", s.config.TwitterAPIURL, params.Encode())

	var response TwitterResponse
	err := s.makeRequest(apiURL, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetFollowingWallets fetches following wallets from GMGN API
func (s *GMGNScraper) GetFollowingWallets() (*WalletsResponse, error) {
	params := url.Values{}
	params.Add("device_id", s.config.DeviceID)
	params.Add("fp_did", s.config.FingerprintID)
	params.Add("client_id", s.config.ClientID)
	params.Add("from_app", "gmgn")
	params.Add("app_ver", s.config.AppVersion)
	params.Add("tz_name", "Asia/Jakarta")
	params.Add("tz_offset", "25200")
	params.Add("app_lang", "id")
	params.Add("os", "web")
	params.Add("worker", "0")
	params.Add("network", s.config.Network)
	params.Add("limit", "100")
	params.Add("order_by", "created_at")
	params.Add("direction", "desc")
	params.Add("chain", s.config.Chain)

	apiURL := fmt.Sprintf("%s?%s", s.config.WalletsAPIURL, params.Encode())

	var response WalletsResponse
	err := s.makeRequest(apiURL, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// makeRequest makes an authenticated HTTP request to GMGN API
func (s *GMGNScraper) makeRequest(url string, target interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	s.setHeaders(req)

	// Make request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Handle compressed response
	var reader io.Reader = resp.Body
	encoding := resp.Header.Get("Content-Encoding")
	
	switch encoding {
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "gzip":
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("error creating gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read response body
	body, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}



	// Check for authentication error
	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized: please check your bearer token and cookies")
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	err = json.Unmarshal(body, target)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %w", err)
	}

	// Check for API error response
	if apiResp, ok := target.(*TwitterResponse); ok {
		if apiResp.Code != 0 {
			return fmt.Errorf("API error: code=%d, reason=%s, message=%s",
				apiResp.Code, apiResp.Reason, apiResp.Message)
		}
	}

	if apiResp, ok := target.(*WalletsResponse); ok {
		if apiResp.Code != 0 {
			return fmt.Errorf("API error: code=%d, reason=%s, message=%s",
				apiResp.Code, apiResp.Reason, apiResp.Message)
		}
	}

	return nil
}

// setHeaders sets all required headers for GMGN API
func (s *GMGNScraper) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.BearerToken))
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://gmgn.ai/portfolio?transferType=Distribute&chain=bsc")
	req.Header.Set("Origin", "https://gmgn.ai")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("DNT", "1")
	req.Header.Set("Sec-GPC", "1")

	// Set cookies
	if s.config.Cookies != "" {
		req.Header.Set("Cookie", s.config.Cookies)
	}

	// Set baggage if configured
	if s.config.Baggage != "" {
		req.Header.Set("Baggage", s.config.Baggage)
	}

	// Set sentry trace if configured
	if s.config.SentryTrace != "" {
		req.Header.Set("Sentry-Trace", s.config.SentryTrace)
	}
}
