package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

var bot *tgbotapi.BotAPI
var lastFetchTime time.Time
var totalProductsFetched int

func initTelegramBot() {
	var err error
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	bot, err = tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	setBotCommands()
}

func setBotCommands() {
	commands := []tgbotapi.BotCommand{
		{
			Command:     "status",
			Description: "Get system status",
		},
		{
			Command:     "q",
			Description: "Query products",
		},
		{
			Command:     "add",
			Description: "Add a keyword to track",
		},
		{
			Command:     "remove",
			Description: "Remove a keyword",
		},
		{
			Command:     "list",
			Description: "List all keywords",
		},
	}

	config := tgbotapi.NewSetMyCommands(commands...)

	_, err := bot.Request(config)
	if err != nil {
		log.Fatalf("Failed to set bot commands: %v", err)
	}
}

func sendTelegramNotification(chatID int64, message string) {
	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Failed to send message to chat ID %d: %v", chatID, err)
	}
}

func handleTelegramUpdates() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			chatID := update.Message.Chat.ID
			args := update.Message.CommandArguments()
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "q":
					listProducts(chatID, args)
				case "add":
					addKeyword(chatID, args)
				case "remove":
					removeKeyword(chatID, args)
				case "list":
					listKeywords(chatID)
				case "status":
					handleStatusCommand(chatID)
				default:
					msg := tgbotapi.NewMessage(chatID, "Unknown command")
					bot.Send(msg)
				}
			} else {
				addSubscriber(chatID)
				msg := tgbotapi.NewMessage(chatID, "You have been subscribed to price drop notifications.")
				bot.Send(msg)
			}
		} else if update.InlineQuery != nil {
			handleInlineQuery(update.InlineQuery)
		}
	}
}

func addSubscriber(chatID int64) {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("INSERT OR IGNORE INTO subscribers (chat_id) VALUES (?)", chatID)
	if err != nil {
		log.Printf("Failed to add subscriber: %v", err)
	}
}

func addKeyword(chatID int64, keyword string) {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if keyword == "" {
		sendTelegramNotification(chatID, "Please provide a keyword to add. Usage: /add <keyword>")
		return
	}

	_, err = db.Exec("INSERT INTO user_keywords (chat_id, keyword) VALUES (?, ?)", chatID, keyword)
	if err != nil {
		log.Printf("Failed to add keyword: %v", err)
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Keyword '%s' added.", keyword))
	bot.Send(msg)

	fetchProducts()

}

func removeKeyword(chatID int64, keyword string) {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if keyword == "" {
		sendTelegramNotification(chatID, "Please provide a keyword to remove. Usage: /remove <keyword>")
		return
	}

	_, err = db.Exec("DELETE FROM user_keywords WHERE chat_id = ? AND keyword = ?", chatID, keyword)
	if err != nil {
		log.Printf("Failed to remove keyword: %v", err)
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Keyword '%s' removed.", keyword))
	bot.Send(msg)
}

func listKeywords(chatID int64) {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT keyword FROM user_keywords WHERE chat_id = ?", chatID)
	if err != nil {
		log.Printf("Failed to query keywords: %v", err)
		return
	}
	defer rows.Close()

	var keywords []string
	for rows.Next() {
		var keyword string
		if err := rows.Scan(&keyword); err != nil {
			log.Printf("Failed to scan keyword: %v", err)
			continue
		}
		keywords = append(keywords, keyword)
	}

	if len(keywords) == 0 {
		msg := tgbotapi.NewMessage(chatID, "No keywords found.")
		bot.Send(msg)
	} else {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Your keywords: %s", strings.Join(keywords, ", ")))
		bot.Send(msg)
	}
}

func listProducts(chatID int64, args string) {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if args == "" {
		sendTelegramNotification(chatID, "Please provide a keyword to query. Usage: /list <keyword>")
		return
	}
	// Query products based on the provided keyword
	keyword := args
	rows, err := db.Query("SELECT name, price, url FROM products WHERE name LIKE ?", "%"+keyword+"%")
	if err != nil {
		log.Printf("Failed to query products: %v", err)
		sendTelegramNotification(chatID, "Failed to query products.")
		return
	}
	defer rows.Close()

	var products []string
	for rows.Next() {
		var name, url string
		var price float64
		if err := rows.Scan(&name, &price, &url); err != nil {
			log.Printf("Failed to scan product: %v", err)
			continue
		}
		products = append(products, fmt.Sprintf("*%s*: $%.2f\n[Check it out!](%s)\n", name, price, url))
	}

	if len(products) == 0 {
		sendTelegramNotification(chatID, fmt.Sprintf("No products found for keyword: %s", keyword))
		return
	}

	const maxMessageLength = 4096
	message := "*Product List*\n"
	for _, product := range products {
		if len(message)+len(product)+1 > maxMessageLength {
			sendTelegramNotification(chatID, message)
			message = ""
		}
		message += product + "\n"
	}

	if message != "" {
		sendTelegramNotification(chatID, message)
	}
}

func handleStatusCommand(chatID int64) {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var keywordCount, productCount int

	err = db.QueryRow("SELECT COUNT(DISTINCT keyword) FROM user_keywords").Scan(&keywordCount)
	if err != nil {
		log.Printf("Failed to query keyword count: %v", err)
		sendTelegramNotification(chatID, "Failed to query keyword count.")
		return
	}

	err = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&productCount)
	if err != nil {
		log.Printf("Failed to query product count: %v", err)
		sendTelegramNotification(chatID, "Failed to query product count.")
		return
	}

	statusMessage := fmt.Sprintf(
		"*System Status*\n- Keywords: %d\n- Products: %d\n- Last Fetch: %s\n- Total Products Fetched Last Time: %d",
		keywordCount, productCount, lastFetchTime.Format(time.RFC3339), totalProductsFetched,
	)

	sendTelegramNotification(chatID, statusMessage)
}

func handleInlineQuery(inlineQuery *tgbotapi.InlineQuery) {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	query := inlineQuery.Query

	rows, err := db.Query("SELECT DISTINCT keyword FROM user_keywords WHERE keyword LIKE ?", "%"+query+"%")
	if err != nil {
		log.Printf("Failed to query keywords: %v", err)
		return
	}
	defer rows.Close()

	var results []interface{}
	for rows.Next() {
		var keyword string
		if err := rows.Scan(&keyword); err != nil {
			log.Printf("Failed to scan keyword: %v", err)
			continue
		}

		result := tgbotapi.NewInlineQueryResultArticle(keyword, keyword, keyword)
		results = append(results, result)
	}

	inlineConf := tgbotapi.InlineConfig{
		InlineQueryID: inlineQuery.ID,
		Results:       results,
		IsPersonal:    true,
		CacheTime:     0, // 0 seconds caching
	}

	if _, err := bot.Request(inlineConf); err != nil {
		log.Printf("Failed to send inline query response: %v", err)
	}
}
