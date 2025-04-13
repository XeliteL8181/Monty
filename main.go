package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// Структуры данных
type CardData struct {
	Savings  float64 `json:"savings"`
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
	Balance  float64 `json:"balance"`
}

type HistoryRecord struct {
	Type          string    `json:"type"`
	Value         float64   `json:"value"`
	IsIncremental bool      `json:"isIncremental"`
	Timestamp     time.Time `json:"timestamp"`
}

type ChartsData struct {
	Months   []string `json:"months"`
	Income   []int    `json:"income"`
	Expenses []int    `json:"expenses"`
	Days     []string `json:"days"`
	Earning  []int    `json:"earning"`
	Spent    []int    `json:"spent"`
}

type FinancialData struct {
	Financial struct {
		Savings  float64 `json:"savings"`
		Income   float64 `json:"income"`
		Expenses float64 `json:"expenses"`
	} `json:"financial"`
	Charts ChartsData `json:"charts"`
}

var (
	db         *sql.DB
	mu         sync.Mutex
	jsonData   FinancialData
	dataLoaded bool
)

func initDB() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL not set in environment variables")
	}

	var err error
	db, err = sql.Open("postgres", connStr)
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

	log.Println("Successfully connected to PostgreSQL")
}

func createTables() {
	query := `
	CREATE TABLE IF NOT EXISTS financial_data (
		id SERIAL PRIMARY KEY,
		savings DECIMAL(10,2) DEFAULT 0,
		income DECIMAL(10,2) DEFAULT 0,
		expenses DECIMAL(10,2) DEFAULT 0,
		balance DECIMAL(10,2) GENERATED ALWAYS AS (income - expenses) STORED,
		last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS charts_data (
		id SERIAL PRIMARY KEY,
		months JSONB,
		income JSONB,
		expenses JSONB,
		days JSONB,
		earning JSONB,
		spent JSONB
	);

	CREATE TABLE IF NOT EXISTS financial_history (
		id SERIAL PRIMARY KEY,
		type VARCHAR(20),
		value DECIMAL(10,2),
		is_incremental BOOLEAN,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Error creating tables:", err)
	}

	// Загружаем данные из JSON при инициализации
	loadInitialData()
}

func loadInitialData() {
	// Читаем данные из JSON файла
	file, err := os.ReadFile("data.json")
	if err != nil {
		log.Fatal("Error reading JSON file:", err)
	}

	err = json.Unmarshal(file, &jsonData)
	if err != nil {
		log.Fatal("Error parsing JSON:", err)
	}

	// Проверяем, есть ли данные в базе
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM financial_data").Scan(&count)
	if err != nil {
		log.Fatal("Error checking data count:", err)
	}

	if count == 0 {
		// Если база пустая, инициализируем её данными из JSON
		_, err = db.Exec(`
			INSERT INTO financial_data (savings, income, expenses) 
			VALUES ($1, $2, $3)`,
			jsonData.Financial.Savings,
			jsonData.Financial.Income,
			jsonData.Financial.Expenses,
		)
		if err != nil {
			log.Fatal("Error initializing financial data:", err)
		}

		// Сохраняем данные графиков
		months, _ := json.Marshal(jsonData.Charts.Months)
		income, _ := json.Marshal(jsonData.Charts.Income)
		expenses, _ := json.Marshal(jsonData.Charts.Expenses)
		days, _ := json.Marshal(jsonData.Charts.Days)
		earning, _ := json.Marshal(jsonData.Charts.Earning)
		spent, _ := json.Marshal(jsonData.Charts.Spent)

		_, err = db.Exec(`
			INSERT INTO charts_data (months, income, expenses, days, earning, spent)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			months, income, expenses, days, earning, spent,
		)
		if err != nil {
			log.Fatal("Error initializing charts data:", err)
		}
	}

	dataLoaded = true
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
	http.HandleFunc("/api/charts", getChartsData)

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
		FROM financial_data 
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
			query = "UPDATE financial_data SET savings = savings + $1"
		} else {
			query = "UPDATE financial_data SET savings = $1"
		}
	case "income":
		if update.IsIncremental {
			query = "UPDATE financial_data SET income = income + $1"
		} else {
			query = "UPDATE financial_data SET income = $1"
		}
	case "expenses":
		if update.IsIncremental {
			query = "UPDATE financial_data SET expenses = expenses + $1"
		} else {
			query = "UPDATE financial_data SET expenses = $1"
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
	_, err = db.Exec(`
		INSERT INTO financial_history (type, value, is_incremental)
		VALUES ($1, $2, $3)`,
		update.Type,
		update.Value,
		update.IsIncremental,
	)
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

	// Сбрасываем данные к значениям из JSON
	_, err := db.Exec(`
		UPDATE financial_data 
		SET savings = $1, income = $2, expenses = $3`,
		jsonData.Financial.Savings,
		jsonData.Financial.Income,
		jsonData.Financial.Expenses,
	)
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
		SELECT type, value, is_incremental, timestamp
		FROM financial_history
		ORDER BY timestamp DESC
		LIMIT 100
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []HistoryRecord
	for rows.Next() {
		var record HistoryRecord
		if err := rows.Scan(&record.Type, &record.Value, &record.IsIncremental, &record.Timestamp); err != nil {
			log.Println("Error scanning history:", err)
			continue
		}
		history = append(history, record)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func getChartsData(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	var data struct {
		Months   []string `json:"months"`
		Income   []int    `json:"income"`
		Expenses []int    `json:"expenses"`
		Days     []string `json:"days"`
		Earning  []int    `json:"earning"`
		Spent    []int    `json:"spent"`
	}

	// Получаем данные графиков из базы
	row := db.QueryRow(`
		SELECT months, income, expenses, days, earning, spent
		FROM charts_data
		ORDER BY id DESC
		LIMIT 1
	`)

	var months, income, expenses, days, earning, spent []byte
	err := row.Scan(&months, &income, &expenses, &days, &earning, &spent)
	if err != nil {
		// Если нет данных в базе, используем данные из JSON
		if !dataLoaded {
			loadInitialData()
		}
		data.Months = jsonData.Charts.Months
		data.Income = jsonData.Charts.Income
		data.Expenses = jsonData.Charts.Expenses
		data.Days = jsonData.Charts.Days
		data.Earning = jsonData.Charts.Earning
		data.Spent = jsonData.Charts.Spent
	} else {
		// Разбираем данные из базы
		json.Unmarshal(months, &data.Months)
		json.Unmarshal(income, &data.Income)
		json.Unmarshal(expenses, &data.Expenses)
		json.Unmarshal(days, &data.Days)
		json.Unmarshal(earning, &data.Earning)
		json.Unmarshal(spent, &data.Spent)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}