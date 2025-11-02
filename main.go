package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	// Command line flags
	apiChoice := flag.String("api", "twitter", "API to call: 'twitter' or 'wallets'")
	outputFile := flag.String("output", "", "Output file path (optional, default: stdout)")
	autoRefresh := flag.Bool("refresh", false, "Auto refresh every 10 seconds")
	sendToTelegram := flag.Bool("telegram", false, "Send results to Telegram")
	flag.Parse()

	// Load config from environment or config file
	config := LoadConfig()
	if config.BearerToken == "" {
		log.Fatal("Bearer token is required. Set GMGN_BEARER_TOKEN environment variable or edit config.json")
	}

	// Create scraper
	scraper := NewGMGNScraper(config)

	// Initialize Telegram bot if needed
	var telegramBot *TelegramBot
	if *sendToTelegram {
		var err error
		telegramBot, err = NewTelegramBot(config.TelegramBotToken, config.TelegramChatID)
		if err != nil {
			log.Fatalf("Failed to initialize Telegram bot: %v", err)
		}
		log.Println("Telegram bot initialized successfully")
	}

	// Run scraper
	for {
		var data interface{}
		var err error

		switch *apiChoice {
		case "twitter":
			data, err = scraper.GetTwitterMessages()
		case "wallets":
			data, err = scraper.GetFollowingWallets()
		default:
			log.Fatal("Invalid API choice. Use 'twitter' or 'wallets'")
		}

		if err != nil {
			log.Printf("Error fetching data: %v\n", err)
			if !*autoRefresh {
				os.Exit(1)
			}
			time.Sleep(30 * time.Second)
			continue
		}

		// Send to Telegram if enabled
		if *sendToTelegram && telegramBot != nil {
			switch *apiChoice {
			case "twitter":
				if twitterData, ok := data.(*TwitterResponse); ok {
					if err := telegramBot.SendTwitterMessages(twitterData); err != nil {
						log.Printf("Error sending to Telegram: %v\n", err)
					} else {
						log.Println("Data sent to Telegram successfully")
					}
				}
			case "wallets":
				if walletData, ok := data.(*WalletsResponse); ok {
					if err := telegramBot.SendWalletData(walletData); err != nil {
						log.Printf("Error sending to Telegram: %v\n", err)
					} else {
						log.Println("Data sent to Telegram successfully")
					}
				}
			}
		}

		// Output results to console/file (unless only sending to Telegram)
		if !*sendToTelegram || *outputFile != "" {
			jsonData, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				log.Printf("Error marshaling JSON: %v\n", err)
				continue
			}

			if *outputFile != "" {
				err = os.WriteFile(*outputFile, jsonData, 0644)
				if err != nil {
					log.Printf("Error writing to file: %v\n", err)
				} else {
					log.Printf("Data written to %s\n", *outputFile)
				}
			} else if !*sendToTelegram {
				fmt.Println(string(jsonData))
			}
		}

		if !*autoRefresh {
			break
		}

		log.Println("Waiting 10 seconds before next refresh...")
		time.Sleep(10 * time.Second)
	}
}
