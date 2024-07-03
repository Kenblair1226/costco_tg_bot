package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

const (
	dbName = "products.db"
)

func setupDatabase() {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlStmt := `
	CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		price REAL,
		url TEXT,
		code TEXT,
		createdAt TEXT,
		updatedAt TEXT,
		last_checked TEXT
	);
	CREATE TABLE IF NOT EXISTS subscribers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER UNIQUE
	);
	CREATE TABLE IF NOT EXISTS user_keywords (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER,
		keyword TEXT,
		FOREIGN KEY(chat_id) REFERENCES subscribers(chat_id)
	);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Fatal(err)
	}
}

func ensureDatabaseStructure() {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 检查表结构并添加缺失的列
	addColumnIfNotExists(db, "products", "code", "TEXT")
	addColumnIfNotExists(db, "products", "createdAt", "TEXT")
	addColumnIfNotExists(db, "products", "updatedAt", "TEXT")
}

func addColumnIfNotExists(db *sql.DB, tableName, columnName, columnType string) {
	query := `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`
	row := db.QueryRow(query, tableName, columnName)

	var count int
	if err := row.Scan(&count); err != nil {
		log.Fatal(err)
	}

	if count == 0 {
		log.Printf("Adding '%s' column to '%s' table", columnName, tableName)
		_, err := db.Exec("ALTER TABLE " + tableName + " ADD COLUMN " + columnName + " " + columnType)
		if err != nil {
			log.Fatalf("Failed to add '%s' column: %v", columnName, err)
		}
	}
}

// getProducts 从数据库中获取所有产品
func getProducts() ([]Product, error) {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT name, price, url, code, createdAt, updatedAt FROM products")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		var price float64
		var createdAt, updatedAt sql.NullString
		if err := rows.Scan(&product.Name, &price, &product.URL, &product.Code, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		product.Price = Price{Value: price}
		product.CreatedAt = createdAt.String
		product.UpdatedAt = updatedAt.String
		products = append(products, product)
	}

	return products, nil
}
