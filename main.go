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

	_ "github.com/lib/pq"
)

// Структуры данных
type CardData struct {
	Savings  float64 `json:"savings"`
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
	Balance  float64 `json:"balance"`
}

type ChartData struct {
	Months   []string `json:"months"`
	Income   []int    `json:"income"`
	Expenses []int    `json:"expenses"`
	Days     []string `json:"days"`
	Earning  []int    `json:"earning"`
	Spent    []int    `json:"spent"`
}

var (
	db  *sql.DB
	mu  sync.Mutex
)

func initDB() {
    connStr := os.Getenv("DATABASE_URL")
    if connStr == "" {
        log.Fatal("DATABASE_URL environment variable not set")
    }

    // Добавьте параметры SSL (обязательно для Render)
    connStr += "?sslmode=require"
    
    var err error
    db, err = sql.Open("postgres", connStr)
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }

    // Установите лимиты соединений
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)

    // Проверка подключения
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := db.PingContext(ctx); err != nil {
        log.Fatal("Database ping failed:", err)
    }
    
    log.Println("Successfully connected to PostgreSQL")
}

func createTables() {
	query := `
	CREATE TABLE IF NOT EXISTS cards (
		id SERIAL PRIMARY KEY,
		savings DECIMAL(10,2) DEFAULT 0,
		income DECIMAL(10,2) DEFAULT 0,
		expenses DECIMAL(10,2) DEFAULT 0,
		balance DECIMAL(10,2) GENERATED ALWAYS AS (income - expenses) STORED,
		last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Error creating tables:", err)
	}

	// Инициализация начальных данных, если таблица пуста
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

	// Статические файлы
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// HTML страницы
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/Monty0.html")
	})

	// API endpoints для карточек
	http.HandleFunc("/api/cards", getCardsData)
	http.HandleFunc("/api/cards/update", updateCardsData)
	http.HandleFunc("/api/cards/reset", resetCardsData)

	// Запуск сервера
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}
	log.Printf("Сервер запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Обработчики для карточек
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
		Type    string  `json:"type"` // "savings", "income" или "expenses"
		Value   float64 `json:"value"`
		IsIncremental bool   `json:"isIncremental"`
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
			query = "UPDATE cards SET savings = savings + $1"
		} else {
			query = "UPDATE cards SET savings = $1"
		}
	case "income":
		if update.IsIncremental {
			query = "UPDATE cards SET income = income + $1"
		} else {
			query = "UPDATE cards SET income = $1"
		}
	case "expenses":
		if update.IsIncremental {
			query = "UPDATE cards SET expenses = expenses + $1"
		} else {
			query = "UPDATE cards SET expenses = $1"
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