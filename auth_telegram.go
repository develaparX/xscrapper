package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

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