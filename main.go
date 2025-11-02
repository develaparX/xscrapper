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
	autoRefresh := flag.Bool("refresh", false, "Auto refresh every 30 seconds")
	flag.Parse()

	// Load config from environment or config file
	config := LoadConfig()
	if config.BearerToken == "" {
		log.Fatal("Bearer token is required. Set GMGN_BEARER_TOKEN environment variable or edit config.json")
	}

	// Create scraper
	scraper := NewGMGNScraper(config)

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

		// Output results
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
		} else {
			fmt.Println(string(jsonData))
		}

		if !*autoRefresh {
			break
		}

		log.Println("Waiting 30 seconds before next refresh...")
		time.Sleep(30 * time.Second)
	}
}
