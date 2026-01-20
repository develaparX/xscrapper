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

		messageText, markup := tb.formatTwitterMessageHTML(&msg)
		
		// Check if tweet has media (images OR videos)
		if len(msg.Content.Media) > 0 {
			var mediaURLs []string
			var hasVideo bool
			
			for _, media := range msg.Content.Media {
				if media.URL != "" {
					if media.Type == "image" {
						mediaURLs = append(mediaURLs, media.URL)
					} else if media.Type == "video" {
						mediaURLs = append(mediaURLs, media.URL)
						hasVideo = true
					}
				}
			}
			
			if len(mediaURLs) > 0 {
				if len(mediaURLs) == 1 && !hasVideo {
					// Single image 
					if err := tb.SendPhotoWithMarkup(mediaURLs[0], messageText, markup); err != nil {
						log.Printf("Failed to send photo: %v, falling back to text", err)
						tb.SendMessageWithMarkup(messageText, markup) // Fallback
					}
				} else {
					// Multiple images or contains video -> Send as MediaGroup
					// Note: MediaGroup caption only works on first item and doesn't support inline buttons on the album itself easily
					// Strategy: Send MediaGroup first, then send the detailed text with buttons as a separate message
					
					// Better approach for Mixed/Video:
					// Re-iterate msg.Content.Media to build proper InputMedia
					var inputMedia []interface{}
					for _, m := range msg.Content.Media {
						if m.URL == "" { continue }
						
						if m.Type == "video" {
							vid := tgbotapi.NewInputMediaVideo(tgbotapi.FileURL(m.URL))
							inputMedia = append(inputMedia, vid)
						} else if m.Type == "image" {
							photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FileURL(m.URL))
							inputMedia = append(inputMedia, photo)
						}
					}
					
					if len(inputMedia) > 0 {
						mediaGroupConfig := tgbotapi.NewMediaGroup(tb.chatID, inputMedia)
						if _, err := tb.bot.SendMediaGroup(mediaGroupConfig); err != nil {
							log.Printf("Failed to send media group: %v", err)
							// If video fails (common with URLs), try sending link in text
							if hasVideo {
								messageText += "\n\n⚠️ <i>(Video media attached, view on X)</i>"
							}
						}
					}
					
					// Always send the formatted text with buttons separately for Albums/Videos
					tb.SendMessageWithMarkup(messageText, markup)
				}
			} else {
				// No valid media URLs
				tb.SendMessageWithMarkup(messageText, markup)
			}
		} else {
			// No media
			tb.SendMessageWithMarkup(messageText, markup)
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
	return tb.SendMessageWithMarkup(text, nil)
}

// SendMessageWithMarkup sends a text message with inline keyboard markup
func (tb *TelegramBot) SendMessageWithMarkup(text string, markup interface{}) error {
	msg := tgbotapi.NewMessage(tb.chatID, text)
	msg.ParseMode = "HTML" // Use HTML for formatting
	msg.DisableWebPagePreview = true
	
	if markup != nil {
		msg.ReplyMarkup = markup
	}

	_, err := tb.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}

	return nil
}

// SendMediaGroup sends multiple photos as a media group with caption
func (tb *TelegramBot) SendMediaGroup(mediaURLs []string, caption string) error {
	if len(mediaURLs) == 0 {
		return nil
	}

	var mediaGroup []interface{}
	
	for i, url := range mediaURLs {
		// Only check HEAD for images/videos if strictly necessary, but often causes issues with some CDNs
		// Skipping strict check to let Telegram handle the fetch
		// if !hasVideo { ... }

		media := tgbotapi.NewInputMediaPhoto(tgbotapi.FileURL(url))
		// Add caption only to the first media item
		if i == 0 {
			media.Caption = caption
		}
		mediaGroup = append(mediaGroup, media)
	}

	if len(mediaGroup) == 0 {
		return fmt.Errorf("no accessible media URLs")
	}

	mediaGroupConfig := tgbotapi.NewMediaGroup(tb.chatID, mediaGroup)
	_, err := tb.bot.SendMediaGroup(mediaGroupConfig)
	if err != nil {
		return fmt.Errorf("failed to send media group: %w", err)
	}

	return nil
}

// SendPhotoWithMarkup sends a photo with HTML caption and inline buttons
func (tb *TelegramBot) SendPhotoWithMarkup(photoURL, caption string, markup interface{}) error {
	// Skip strict URL check to avoid false negatives with some CDNs
	/*
	resp, err := http.Head(photoURL)
	if err != nil || resp.StatusCode != 200 {
		return fmt.Errorf("photo URL not accessible")
	}
	*/

	photo := tgbotapi.NewPhoto(tb.chatID, tgbotapi.FileURL(photoURL))
	photo.Caption = caption
	photo.ParseMode = "HTML"
	if markup != nil {
		photo.ReplyMarkup = markup
	}

	_, err := tb.bot.Send(photo)
	return err
}

// formatTwitterMessageHTML formats message with HTML and returns inline keyboard markup
func (tb *TelegramBot) formatTwitterMessageHTML(msg *TwitterMessage) (string, interface{}) {
	var builder strings.Builder

	// Escape HTML special chars function
	esc := func(s string) string {
		return strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;").Replace(s)
	}

	// 1. Header: User Info
	// 🐦 User Name (@handle)
	builder.WriteString(fmt.Sprintf("<b>%s</b> (<a href=\"https://x.com/%s\">@%s</a>)\n", 
		esc(msg.User.Name), 
		msg.User.ScreenName, 
		esc(msg.User.ScreenName)))

	// Tweet Type & Time
	timestamp, _ := strconv.ParseInt(msg.Timestamp, 10, 64)
	timeStr := time.Unix(timestamp/1000, 0).Format("15:04:05")
	
	typeIcon := "📝"
	typeLabel := strings.ToUpper(msg.TweetType)
	switch msg.TweetType {
	case "tweet": typeIcon = "🐦"; typeLabel = "NEW TWEET"
	case "reply": typeIcon = "↩️"; typeLabel = "REPLY"
	case "repost": typeIcon = "🔄"; typeLabel = "REPOST"
	case "quote": typeIcon = "💬"; typeLabel = "QUOTE"
	case "delete_post": typeIcon = "🗑️"; typeLabel = "DELETED"
	case "pin": typeIcon = "📌"; typeLabel = "PINNED"
	case "unpin": typeIcon = "📍"; typeLabel = "UNPINNED"
	case "follow": typeIcon = "👥"; typeLabel = "FOLLOWED"
	case "unfollow": typeIcon = "🚫"; typeLabel = "UNFOLLOWED"
	case "description": typeIcon = "📝"; typeLabel = "BIO UPDATE"
	case "name": typeIcon = "✏️"; typeLabel = "NAME UPDATE"
	case "handle": typeIcon = "🔄"; typeLabel = "HANDLE UPDATE"
	}
	
	builder.WriteString(fmt.Sprintf("%s <code>%s</code> | 🕒 %s\n\n", typeIcon, typeLabel, timeStr))

	// 2. Content based on Type
	switch msg.TweetType {
	case "follow":
		if msg.Action != nil && msg.Action.Follow != nil && msg.Action.Follow.Following != nil {
			f := msg.Action.Follow.Following
			builder.WriteString(fmt.Sprintf("<b>Started following:</b>\n👤 <b>%s</b> (@%s)\n", esc(f.Name), esc(f.ScreenName)))
			if f.Followers > 0 {
				builder.WriteString(fmt.Sprintf("👥 Followers: <code>%s</code>\n", formatNumber(f.Followers)))
			}
			if f.Description != "" {
				builder.WriteString(fmt.Sprintf("📝 <i>%s</i>\n", esc(f.Description)))
			}
		}
	case "unfollow":
		if msg.Action != nil && msg.Action.Follow != nil && msg.Action.Follow.Following != nil {
			f := msg.Action.Follow.Following
			builder.WriteString(fmt.Sprintf("<b>Stopped following:</b>\n👤 <b>%s</b> (@%s)\n", esc(f.Name), esc(f.ScreenName)))
		}
	case "delete_post":
		builder.WriteString("<b>Deleted Content:</b>\n")
		if msg.Content.Text != "" {
			builder.WriteString(fmt.Sprintf("<s>%s</s>", esc(msg.Content.Text)))
		} else {
			builder.WriteString("<i>(Content unavailable)</i>")
		}
	case "description":
		if msg.Profile != nil {
			if msg.Profile.BeforeDescription != "" {
				builder.WriteString(fmt.Sprintf("❌ <b>Old:</b> %s\n", esc(msg.Profile.BeforeDescription)))
				builder.WriteString(fmt.Sprintf("✅ <b>New:</b> %s\n", esc(msg.Profile.AfterDescription)))
			} else {
				builder.WriteString(fmt.Sprintf("✅ <b>New Bio:</b> %s\n", esc(msg.Profile.Description)))
			}
		}
	case "name":
		if msg.Profile != nil {
			builder.WriteString(fmt.Sprintf("❌ <b>Old:</b> %s\n", esc(msg.Profile.BeforeName)))
			builder.WriteString(fmt.Sprintf("✅ <b>New:</b> %s\n", esc(msg.Profile.AfterName)))
		}
	case "handle":
		if msg.Profile != nil {
			builder.WriteString(fmt.Sprintf("❌ <b>Old:</b> @%s\n", esc(msg.Profile.BeforeHandle)))
			builder.WriteString(fmt.Sprintf("✅ <b>New:</b> @%s\n", esc(msg.Profile.AfterHandle)))
		}
	default: // tweet, reply, repost, quote, pin, unpin
		if msg.Content.Text != "" {
			builder.WriteString(esc(msg.Content.Text))
		}
	}

	// 3. User Tags
	if len(msg.UserTags) > 0 {
		builder.WriteString(fmt.Sprintf("\n\n🏷️ <i>#%s</i>", strings.Join(msg.UserTags, " #")))
	}

	// 4. Source / Repost Info
	if msg.SourceUser != nil {
		builder.WriteString(fmt.Sprintf("\n\n🔄 <b>Replying/Quoting:</b>\n👤 <b>%s</b> (@%s)", 
			esc(msg.SourceUser.Name), esc(msg.SourceUser.ScreenName)))
		if msg.SourceContent != nil && msg.SourceContent.Text != "" {
			text := esc(msg.SourceContent.Text)
			if len(text) > 50 { 
				text = text[:50] + "..." 
			}
			builder.WriteString(fmt.Sprintf("\n<blockquote>\"%s\"</blockquote>", text))
		}
	}

	// 5. Build Buttons (Inline Keyboard)
	var rows [][]tgbotapi.InlineKeyboardButton
	
	// Row 1: Tweet Link Action
	var row1 []tgbotapi.InlineKeyboardButton
	if msg.TweetID != "" {
		url := fmt.Sprintf("https://x.com/%s/status/%s", msg.User.ScreenName, msg.TweetID)
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonURL("🔗 Open Tweet", url))
	}
	
	// Profile Link
	profileUrl := fmt.Sprintf("https://x.com/%s", msg.User.ScreenName)
	row1 = append(row1, tgbotapi.NewInlineKeyboardButtonURL("👤 Profile", profileUrl))
	
	rows = append(rows, row1)

	// Additional Rows for specific actions
	if msg.TweetType == "follow" || msg.TweetType == "unfollow" {
		if msg.Action != nil && msg.Action.Follow != nil && msg.Action.Follow.Following != nil {
			target := msg.Action.Follow.Following
			targetUrl := fmt.Sprintf("https://x.com/%s", target.ScreenName)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL(fmt.Sprintf("View %s", target.Name), targetUrl),
			))
		}
	}
	
	markup := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return builder.String(), markup
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