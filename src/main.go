package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	setupDatabase()
	ensureDatabaseStructure()
	initTelegramBot() // 初始化 Telegram Bot

	// 启动定时任务
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for {
			fetchProducts()
			// checkPriceChanges(products)

			<-ticker.C
		}
	}()

	// 启动 Telegram Bot 更新处理
	go handleTelegramUpdates()

	// 启动HTTP服务器
	http.HandleFunc("/products", productsHandler)
	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// productsHandler 处理 /products 路由
func productsHandler(w http.ResponseWriter, r *http.Request) {
	products, err := getProducts()
	if err != nil {
		http.Error(w, "Failed to fetch products from database", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(products); err != nil {
		http.Error(w, "Failed to encode products to JSON", http.StatusInternalServerError)
	}
}
