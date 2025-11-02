package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Authentication
	BearerToken string `json:"bearer_token"`
	Cookies     string `json:"cookies"`

	// Device & Client Info
	DeviceID      string `json:"device_id"`
	FingerprintID string `json:"fingerprint_id"`
	ClientID      string `json:"client_id"`
	AppVersion    string `json:"app_version"`

	// API URLs
	TwitterAPIURL string `json:"twitter_api_url"`
	WalletsAPIURL string `json:"wallets_api_url"`

	// Filters
	UserTags   []string `json:"user_tags"`
	TweetTypes []string `json:"tweet_types"`
	Network    string   `json:"network"`
	Chain      string   `json:"chain"`

	// Optional headers
	Baggage     string `json:"baggage"`
	SentryTrace string `json:"sentry_trace"`

	// HTTP Client settings
	Timeout time.Duration `json:"-"`
}

// LoadConfig loads configuration from .env, environment variables, or config.json
func LoadConfig() *Config {
	// Load .env file if exists (silent fail if not found)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables or config.json")
	}

	config := &Config{
		// Default values
		DeviceID:      "7754041a-7df7-4454-80aa-2160aec01882",
		FingerprintID: "8de143c466b06cd9949cb90da027a288",
		ClientID:      "gmgn_web_20251101-6461-0986672",
		AppVersion:    "20251101-6461-0986672",
		TwitterAPIURL: "https://gmgn.ai/vas/api/v1/twitter/messages",
		WalletsAPIURL: "https://gmgn.ai/api/v1/follow/following_wallets_v2",
		Network:       "bsc",
		Chain:         "bsc",
		Timeout:       30 * time.Second,

		UserTags: []string{
			"kol", "trader", "master", "politics", "media",
			"companies", "founder", "exchange", "celebrity",
			"binance_square", "other",
		},

		TweetTypes: []string{
			"tweet", "reply", "repost", "quote", "description",
			"follow", "photo", "banner", "name", "handle",
		},
	}

	// Try to load from config.json
	if data, err := os.ReadFile("config.json"); err == nil {
		if err := json.Unmarshal(data, config); err != nil {
			log.Printf("Warning: Failed to parse config.json: %v\n", err)
		} else {
			log.Println("Loaded configuration from config.json")
		}
	}

	// Override with environment variables (from .env or system)
	if token := os.Getenv("GMGN_BEARER_TOKEN"); token != "" {
		config.BearerToken = token
		log.Println("Bearer token loaded from environment")
	}
	if cookies := os.Getenv("GMGN_COOKIES"); cookies != "" {
		config.Cookies = cookies
		log.Println("Cookies loaded from environment")
	}
	if deviceID := os.Getenv("GMGN_DEVICE_ID"); deviceID != "" {
		config.DeviceID = deviceID
	}
	if chain := os.Getenv("GMGN_CHAIN"); chain != "" {
		config.Chain = chain
		config.Network = chain
	}

	return config
}

// SaveConfig saves the current config to config.json
func (c *Config) SaveConfig() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("config.json", data, 0644)
}
