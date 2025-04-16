package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

const maxValue int64 = 99999999

// Структуры данных
type CardData struct {
	Savings  int64 `json:"savings"`
	Income   int64 `json:"income"`
	Expenses int64 `json:"expenses"`
	Balance  int64 `json:"balance"`
}

type ChartData struct {
	Months   []string `json:"months"`
	Income   []int64  `json:"income"`
	Expenses []int64  `json:"expenses"`
	Days     []string `json:"days"`
	Earning  []int64  `json:"earning"`
	Spent    []int64  `json:"spent"`
}

type HistoryRecord struct {
	Type          string    `json:"type"`
	Value         int64     `json:"value"`
	IsIncremental bool      `json:"isIncremental"`
	Timestamp     time.Time `json:"timestamp"`
}

var (
	db *sql.DB
	mu sync.Mutex
)

func initDB() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://finance_user:your_password@localhost:5432/finance_db?sslmode=disable"
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
	CREATE TABLE IF NOT EXISTS cards (
		id SERIAL PRIMARY KEY,
		savings BIGINT DEFAULT 0,
		income BIGINT DEFAULT 0,
		expenses BIGINT DEFAULT 0,
		balance BIGINT GENERATED ALWAYS AS (income - expenses) STORED,
		last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS card_history (
		id SERIAL PRIMARY KEY,
		card_id INT,
		data JSONB,
		changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (card_id) REFERENCES cards(id)
	);

	CREATE TABLE IF NOT EXISTS charts (
		id SERIAL PRIMARY KEY,
		months JSONB,
		income JSONB,
		expenses JSONB,
		days JSONB,
		earning JSONB,
		spent JSONB,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Error creating tables:", err)
	}

	// Инициализация данных карточек
	var count int
	db.QueryRow("SELECT COUNT(*) FROM cards").Scan(&count)
	if count == 0 {
		_, err = db.Exec("INSERT INTO cards (savings, income, expenses) VALUES (0, 0, 0)")
		if err != nil {
			log.Fatal("Error initializing cards data:", err)
		}
	}

	// Инициализация данных графиков
	db.QueryRow("SELECT COUNT(*) FROM charts").Scan(&count)
	if count == 0 {
		_, err = db.Exec(`
			INSERT INTO charts (months, income, expenses, days, earning, spent)
			VALUES (
				'["Янв", "Фев", "Март", "Апр", "Май", "Июнь", "Июль", "Авг", "Сен", "Окт", "Нояб", "Дек"]',
				'[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]',
				'[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]',
				'["Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"]',
				'[0, 0, 0, 0, 0, 0, 0]',
				'[0, 0, 0, 0, 0, 0, 0]'
			)
		`)
		if err != nil {
			log.Fatal("Error initializing charts data:", err)
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
		http.ServeFile(w, r, "static/index.html")
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
		Type          string `json:"type"`
		Value         int64  `json:"value"`
		IsIncremental bool   `json:"isIncremental"`
	}

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if update.Value < 0 || update.Value > maxValue {
		http.Error(w, "Значение должно быть от 0 до 99999999", http.StatusBadRequest)
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

	// Обновляем графики при изменении доходов/расходов
	if update.Type == "income" || update.Type == "expenses" {
		updateChartsData(update.Type, update.Value)
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
		VALUES (1, $1)
	`, jsonData)
	if err != nil {
		log.Println("Failed to save history:", err)
	}

	w.WriteHeader(http.StatusOK)
}

func updateChartsData(updateType string, value int64) {
	now := time.Now()
	month := int(now.Month()) - 1 // 0-based
	day := int(now.Weekday())     // 0=Sunday

	var field string
	if updateType == "income" {
		field = "income"
	} else {
		field = "expenses"
	}

	// Обновляем месячные данные
	_, err := db.Exec(fmt.Sprintf(`
		UPDATE charts 
		SET %s = jsonb_set(%s, '{%d}', to_jsonb($1::bigint + (%s->>'%d')::bigint))
		WHERE id = 1
	`, field, field, month, field, month), value)
	if err != nil {
		log.Println("Failed to update monthly chart data:", err)
	}

	// Обновляем недельные данные
	weeklyField := "earning"
	if updateType == "expenses" {
		weeklyField = "spent"
	}

	// Для недельного графика используем день недели (0=воскресенье)
	// Переводим в формат где 0=понедельник
	weekday := (day + 6) % 7

	_, err = db.Exec(fmt.Sprintf(`
		UPDATE charts 
		SET %s = jsonb_set(%s, '{%d}', to_jsonb($1::bigint + (%s->>'%d')::bigint))
		WHERE id = 1
	`, weeklyField, weeklyField, weekday, weeklyField, weekday), value)
	if err != nil {
		log.Println("Failed to update weekly chart data:", err)
	}
}

func getChartsData(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	var data ChartData
	var months, income, expenses, days, earning, spent []byte

	err := db.QueryRow(`
		SELECT months, income, expenses, days, earning, spent
		FROM charts
		ORDER BY updated_at DESC
		LIMIT 1
	`).Scan(&months, &income, &expenses, &days, &earning, &spent)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.Unmarshal(months, &data.Months)
	json.Unmarshal(income, &data.Income)
	json.Unmarshal(expenses, &data.Expenses)
	json.Unmarshal(days, &data.Days)
	json.Unmarshal(earning, &data.Earning)
	json.Unmarshal(spent, &data.Spent)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
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

	// Также сбрасываем графики
	_, err = db.Exec(`
		UPDATE charts 
		SET 
			income = '[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]',
			expenses = '[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]',
			earning = '[0, 0, 0, 0, 0, 0, 0]',
			spent = '[0, 0, 0, 0, 0, 0, 0]'
		WHERE id = 1
	`)
	if err != nil {
		log.Println("Failed to reset charts:", err)
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