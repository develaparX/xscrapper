package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramBot struct {
	bot         *tgbotapi.BotAPI
	chatID      int64
	sentTweets  map[string]bool // Track sent tweet IDs to avoid duplicates
	sentWallets map[string]bool // Track sent wallet addresses to avoid duplicates
}

// NewTelegramBot creates a new Telegram bot instance
func NewTelegramBot(token, chatID string) (*TelegramBot, error) {
	if token == "" || chatID == "" {
		return nil, fmt.Errorf("telegram bot token and chat ID are required")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	log.Printf("Telegram bot authorized on account %s", bot.Self.UserName)

	telegramBot := &TelegramBot{
		bot:         bot,
		chatID:      chatIDInt,
		sentTweets:  make(map[string]bool),
		sentWallets: make(map[string]bool),
	}

	// Start cleanup routine to prevent memory leaks
	go telegramBot.startCleanupRoutine()

	return telegramBot, nil
}

// startCleanupRoutine starts a background routine to clean up old tracking data
func (tb *TelegramBot) startCleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour) // Clean up every hour
	defer ticker.Stop()

	for range ticker.C {
		// Clear old tracking data to prevent memory leaks
		// Keep only recent entries (this is a simple approach)
		if len(tb.sentTweets) > 1000 {
			// Keep only the last 500 entries
			newMap := make(map[string]bool)
			count := 0
			for k, v := range tb.sentTweets {
				if count >= 500 {
					break
				}
				newMap[k] = v
				count++
			}
			tb.sentTweets = newMap
			log.Println("Cleaned up tweet tracking cache")
		}

		if len(tb.sentWallets) > 1000 {
			// Keep only the last 500 entries
			newMap := make(map[string]bool)
			count := 0
			for k, v := range tb.sentWallets {
				if count >= 500 {
					break
				}
				newMap[k] = v
				count++
			}
			tb.sentWallets = newMap
			log.Println("Cleaned up wallet tracking cache")
		}
	}
}

// SendTwitterMessages sends only new Twitter messages to Telegram
func (tb *TelegramBot) SendTwitterMessages(response *TwitterResponse) error {
	if len(response.Data) == 0 {
		return nil // Don't send "no messages" notification for realtime
	}

	// Filter new messages (not sent before)
	var newMessages []TwitterMessage
	for _, msg := range response.Data {
		if !tb.sentTweets[msg.ID] {
			newMessages = append(newMessages, msg)
		}
	}

	if len(newMessages) == 0 {
		return nil // No new messages to send
	}

	// Sort messages by timestamp (oldest first) for chronological order
	sort.Slice(newMessages, func(i, j int) bool {
		timestampI, _ := strconv.ParseInt(newMessages[i].Timestamp, 10, 64)
		timestampJ, _ := strconv.ParseInt(newMessages[j].Timestamp, 10, 64)
		return timestampI < timestampJ
	})

	// Send messages in chronological order
	for _, msg := range newMessages {
		// Mark as sent
		tb.sentTweets[msg.ID] = true

		messageText := tb.formatTwitterMessage(&msg)
		if err := tb.SendMessage(messageText); err != nil {
			log.Printf("Failed to send message %s: %v", msg.ID, err)
		}

		// Small delay to avoid rate limiting
		time.Sleep(200 * time.Millisecond)
	}

	log.Printf("Sent %d new Twitter messages to Telegram (chronological order)", len(newMessages))

	return nil
}

// SendWalletData sends only new wallet data to Telegram
func (tb *TelegramBot) SendWalletData(response *WalletsResponse) error {
	if len(response.Data.List) == 0 {
		return nil // Don't send "no wallets" notification for realtime
	}

	newWalletsCount := 0

	// Send only new wallets (not sent before)
	for _, wallet := range response.Data.List {
		// Check if we've already sent this wallet
		if tb.sentWallets[wallet.WalletAddress] {
			continue
		}

		// Mark as sent
		tb.sentWallets[wallet.WalletAddress] = true
		newWalletsCount++

		messageText := tb.formatWallet(&wallet)
		if err := tb.SendMessage(messageText); err != nil {
			log.Printf("Failed to send wallet %s: %v", wallet.WalletAddress, err)
		}

		// Small delay to avoid rate limiting
		time.Sleep(200 * time.Millisecond)
	}

	if newWalletsCount > 0 {
		log.Printf("Sent %d new wallets to Telegram", newWalletsCount)
	}

	return nil
}

// SendMessage sends a text message to Telegram
func (tb *TelegramBot) SendMessage(text string) error {
	msg := tgbotapi.NewMessage(tb.chatID, text)
	// Remove markdown parsing to avoid issues
	msg.DisableWebPagePreview = true

	_, err := tb.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}

	return nil
}

// formatTwitterMessage formats a Twitter message for Telegram
func (tb *TelegramBot) formatTwitterMessage(msg *TwitterMessage) string {
	var builder strings.Builder

	// Header with user info
	builder.WriteString("🐦 X Message\n\n")
	builder.WriteString(fmt.Sprintf("👤 %s (@%s)\n", 
		msg.User.Name, 
		msg.User.ScreenName))

	if msg.User.Followers > 0 {
		builder.WriteString(fmt.Sprintf("👥 Followers: %s\n", formatNumber(msg.User.Followers)))
	}

	// User tags
	if len(msg.UserTags) > 0 {
		builder.WriteString(fmt.Sprintf("🏷️ Tags: %s\n", strings.Join(msg.UserTags, ", ")))
	}

	// Tweet type and timestamp
	timestamp, _ := strconv.ParseInt(msg.Timestamp, 10, 64)
	timeStr := time.Unix(timestamp/1000, 0).Format("2006-01-02 15:04:05")
	builder.WriteString(fmt.Sprintf("📅 %s | 📝 %s\n\n", timeStr, msg.TweetType))

	// Content
	builder.WriteString("💬 Content:\n")
	builder.WriteString(msg.Content.Text)

	// Token info if available
	// if msg.Token != nil {
	// 	builder.WriteString("\n\n💰 Token Info:\n")
	// 	builder.WriteString(fmt.Sprintf("🪙 %s (%s)\n", msg.Token.Symbol, msg.Token.Chain))
	// 	if msg.Token.Price != "" {
	// 		builder.WriteString(fmt.Sprintf("💵 Price: $%s\n", msg.Token.Price))
	// 	}
	// 	if msg.Token.MarketCap != "" {
	// 		builder.WriteString(fmt.Sprintf("📊 Market Cap: $%s\n", msg.Token.MarketCap))
	// 	}
	// }

	// Source info for reposts
	if msg.SourceUser != nil {
		builder.WriteString("\n\n🔄 Original Tweet:\n")
		builder.WriteString(fmt.Sprintf("👤 %s (@%s)\n", 
			msg.SourceUser.Name, 
			msg.SourceUser.ScreenName))
		if msg.SourceContent != nil {
			builder.WriteString(fmt.Sprintf("💬 %s", msg.SourceContent.Text))
		}
	}

	builder.WriteString("\n\n---")

	return builder.String()
}

// formatWallet formats wallet data for Telegram
func (tb *TelegramBot) formatWallet(wallet *Wallet) string {
	var builder strings.Builder

	builder.WriteString("💰 Wallet Info\n\n")
	builder.WriteString(fmt.Sprintf("🏦 Address: %s\n", wallet.WalletAddress))
	builder.WriteString(fmt.Sprintf("⛓️ Chain: %s\n", wallet.Chain))

	if wallet.WalletTag != "" {
		builder.WriteString(fmt.Sprintf("🏷️ Tag: %s\n", wallet.WalletTag))
	}

	if wallet.Balance != "" {
		builder.WriteString(fmt.Sprintf("💵 Balance: $%s\n", wallet.Balance))
	}

	if wallet.PNL != "" {
		builder.WriteString(fmt.Sprintf("📈 PNL: $%s\n", wallet.PNL))
	}

	if wallet.WinRate > 0 {
		builder.WriteString(fmt.Sprintf("🎯 Win Rate: %.2f%%\n", wallet.WinRate*100))
	}

	if wallet.TotalTrades > 0 {
		builder.WriteString(fmt.Sprintf("🔄 Total Trades: %d\n", wallet.TotalTrades))
	}

	if wallet.TwitterHandle != "" {
		builder.WriteString(fmt.Sprintf("🐦 Twitter: @%s\n", wallet.TwitterHandle))
	}

	if wallet.ENS != "" {
		builder.WriteString(fmt.Sprintf("🌐 ENS: %s\n", wallet.ENS))
	}

	builder.WriteString(fmt.Sprintf("📅 Created: %s\n", wallet.CreatedAt))

	builder.WriteString("\n---")

	return builder.String()
}

// escapeMarkdown escapes special characters for Telegram MarkdownV2
func escapeMarkdown(text string) string {
	// For MarkdownV2, we need to escape these characters
	specialChars := []string{
		"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!",
	}
	
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

// formatNumber formats large numbers with commas
func formatNumber(num int) string {
	str := strconv.Itoa(num)
	n := len(str)
	if n <= 3 {
		return str
	}

	var result strings.Builder
	for i, digit := range str {
		if i > 0 && (n-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(digit)
	}
	return result.String()
}