package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
	"context"

	_ "github.com/go-sql-driver/mysql"
)

// Структуры данных
type CardData struct {
	Savings  float64 `json:"savings"`
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
	Balance  float64 `json:"balance"`
}

type HistoryRecord struct {
	Type           string    `json:"type"`
	Value          float64   `json:"value"`
	IsIncremental  bool      `json:"isIncremental"`
	Timestamp      time.Time `json:"timestamp"`
}

var (
	db  *sql.DB
	mu  sync.Mutex
)

func initDB() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		// Формат для MySQL: "user:password@tcp(host:port)/dbname"
		connStr = "root:password@tcp(localhost:3306)/finance_db?parseTime=true"
	}

	var err error
	db, err = sql.Open("mysql", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		log.Fatal("Database ping failed:", err)
	}
	
	log.Println("Successfully connected to MySQL")
}

func createTables() {
	query := `
	CREATE TABLE IF NOT EXISTS cards (
		id INT AUTO_INCREMENT PRIMARY KEY,
		savings DECIMAL(10,2) DEFAULT 0,
		income DECIMAL(10,2) DEFAULT 0,
		expenses DECIMAL(10,2) DEFAULT 0,
		balance DECIMAL(10,2) AS (income - expenses),
		last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB;

	CREATE TABLE IF NOT EXISTS card_history (
		id INT AUTO_INCREMENT PRIMARY KEY,
		card_id INT,
		data JSON,
		changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (card_id) REFERENCES cards(id)
	) ENGINE=InnoDB;
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Error creating tables:", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM cards").Scan(&count)
	if count == 0 {
		_, err = db.Exec("INSERT INTO cards (savings, income, expenses) VALUES (0, 0, 0)")
		if err != nil {
			log.Fatal("Error initializing data:", err)
		}
	}
}

func main() {
	initDB()
	defer db.Close()
	createTables()

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/Monty0.html")
	})

	// API endpoints
	http.HandleFunc("/api/cards", getCardsData)
	http.HandleFunc("/api/cards/update", updateCardsData)
	http.HandleFunc("/api/cards/reset", resetCardsData)
	http.HandleFunc("/api/cards/history", getHistoryData)

	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}
	log.Printf("Сервер запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Обработчики
func getCardsData(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	var data CardData
	err := db.QueryRow(`
		SELECT savings, income, expenses, balance 
		FROM cards 
		ORDER BY last_updated DESC 
		LIMIT 1
	`).Scan(&data.Savings, &data.Income, &data.Expenses, &data.Balance)
	
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func updateCardsData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var update struct {
		Type          string  `json:"type"`
		Value         float64 `json:"value"`
		IsIncremental bool    `json:"isIncremental"`
	}

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	var query string
	switch update.Type {
	case "savings":
		if update.IsIncremental {
			query = "UPDATE cards SET savings = savings + ?"
		} else {
			query = "UPDATE cards SET savings = ?"
		}
	case "income":
		if update.IsIncremental {
			query = "UPDATE cards SET income = income + ?"
		} else {
			query = "UPDATE cards SET income = ?"
		}
	case "expenses":
		if update.IsIncremental {
			query = "UPDATE cards SET expenses = expenses + ?"
		} else {
			query = "UPDATE cards SET expenses = ?"
		}
	default:
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(query, update.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Сохраняем историю изменений
	historyData := HistoryRecord{
		Type:          update.Type,
		Value:         update.Value,
		IsIncremental: update.IsIncremental,
		Timestamp:     time.Now(),
	}
	jsonData, _ := json.Marshal(historyData)

	_, err = db.Exec(`
		INSERT INTO card_history (card_id, data) 
		VALUES (1, ?)
	`, jsonData)
	if err != nil {
		log.Println("Failed to save history:", err)
	}

	w.WriteHeader(http.StatusOK)
}

func resetCardsData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	_, err := db.Exec("UPDATE cards SET savings = 0, income = 0, expenses = 0")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func getHistoryData(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	rows, err := db.Query(`
		SELECT data 
		FROM card_history 
		ORDER BY changed_at DESC
		LIMIT 100
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []HistoryRecord
	for rows.Next() {
		var jsonData []byte
		var record HistoryRecord
		
		if err := rows.Scan(&jsonData); err != nil {
			log.Println("Error scanning history:", err)
			continue
		}
		
		if err := json.Unmarshal(jsonData, &record); err != nil {
			log.Println("Error unmarshaling history:", err)
			continue
		}
		
		history = append(history, record)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}