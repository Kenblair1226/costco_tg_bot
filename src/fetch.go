package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL = "https://www.costco.com.tw/rest/v2/taiwan/products/search?query=%s&currentPage=%d"
)

type Price struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

type Product struct {
	Name      string `json:"name"`
	Price     Price  `json:"price"`
	URL       string `json:"url"`
	Code      string `json:"code"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type Pagination struct {
	CurrentPage  int `json:"currentPage"`
	PageSize     int `json:"pageSize"`
	TotalPages   int `json:"totalPages"`
	TotalResults int `json:"totalResults"`
}

type ApiResponse struct {
	Products   []Product  `json:"products"`
	Pagination Pagination `json:"pagination"`
}

func fetchProducts() {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT DISTINCT keyword FROM user_keywords")
	if err != nil {
		log.Fatalf("Failed to query user keywords: %v", err)
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

	totalProductsFetched = 0
	lastFetchTime = time.Now()

	for _, keyword := range keywords {
		var totalProducts int
		var priceChanges []string

		currentPage := 0
		for {
			url := fmt.Sprintf(baseURL, keyword, currentPage)
			products, pagination := fetchProductsFromPage(url)

			totalProducts += len(products)
			totalProductsFetched += len(products)

			for _, product := range products {
				change := checkAndNotify(db, product, keyword)
				if change != "" {
					priceChanges = append(priceChanges, change)
				}
			}

			if currentPage >= pagination.TotalPages-1 {
				break
			}
			currentPage++
		}

		log.Printf("Keyword: %s, Total Products Fetched: %d", keyword, totalProducts)
		for _, change := range priceChanges {
			log.Println(change)
		}
	}
}

func fetchProductsFromPage(url string) ([]Product, Pagination) {
	var products []Product
	var pagination Pagination

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to fetch the URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Failed to fetch the URL: received status code %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read the response body: %v", err)
	}

	var apiResponse ApiResponse
	err = json.Unmarshal(bodyBytes, &apiResponse)
	if err != nil {
		log.Fatalf("Failed to parse the JSON response: %v\nResponse Body: %s", err, string(bodyBytes))
	}

	for _, product := range apiResponse.Products {
		if product.Name == "" || product.Price.Value == 0 || product.URL == "" || product.Code == "" {
			log.Printf("Incomplete product data: %+v", product)
			continue
		}

		product.URL = fmt.Sprintf("https://www.costco.com.tw%s", product.URL)
		product.URL = fixURL(product.URL)

		products = append(products, product)
	}

	pagination = apiResponse.Pagination
	return products, pagination
}

func checkAndNotify(db *sql.DB, product Product, keyword string) string {
	var oldPrice float64

	err := db.QueryRow("SELECT price FROM products WHERE code = ?", product.Code).Scan(&oldPrice)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Failed to query product: %v", err)
		return ""
	}

	if err == sql.ErrNoRows {
		_, err = db.Exec("INSERT INTO products (name, price, url, code) VALUES (?, ?, ?, ?)", product.Name, product.Price.Value, product.URL, product.Code)
		if err != nil {
			log.Printf("Failed to insert new product: %v", err)
			return ""
		}

		message := formatTelegramMessage(product.Name, 0, product.Price.Value, product.URL)
		notifySubscribers(db, keyword, message)
		return message
	} else if oldPrice != product.Price.Value {
		_, err = db.Exec("UPDATE products SET price = ?, updatedAt = CURRENT_TIMESTAMP WHERE code = ?", product.Price.Value, product.Code)
		if err != nil {
			log.Printf("Failed to update product price: %v", err)
			return ""
		}

		message := formatTelegramMessage(product.Name, oldPrice, product.Price.Value, product.URL)
		notifySubscribers(db, keyword, message)
		return message
	}
	return ""
}

func formatTelegramMessage(name string, oldPrice, newPrice float64, url string) string {
	if oldPrice == 0 {
		return fmt.Sprintf("*%s*\nPrice: *%.2f*\n[Check it out!](%s)", name, newPrice, url)
	}
	return fmt.Sprintf("*%s*\nOld Price: *%.2f*\nNew Price: *%.2f*\n[Check it out!](%s)", name, oldPrice, newPrice, url)
}

func fixURL(rawURL string) string {
	return strings.Replace(rawURL, "https://www.costco.com.tw//", "https://www.costco.com.tw/", 1)
}

func notifySubscribers(db *sql.DB, keyword, message string) {
	rows, err := db.Query("SELECT DISTINCT chat_id FROM user_keywords WHERE keyword = ?", keyword)
	if err != nil {
		log.Printf("Failed to query subscribers: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			log.Printf("Failed to scan chat_id: %v", err)
			continue
		}

		sendTelegramNotification(chatID, message)
	}
}
