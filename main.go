package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
)

const (
	maxValue        int64         = 99999999
	defaultPort     string        = "3000"
	serverTimeout   time.Duration = 15 * time.Second
	shutdownTimeout time.Duration = 5 * time.Second
)

type CardData struct {
	Savings  int64 `json:"savings"`
	Income   int64 `json:"income"`
	Expense  int64 `json:"expense"`
	Balance  int64 `json:"balance"`
}

type ChartData struct {
	Months   []string `json:"months"`
	Income   []int64  `json:"income"`
	Expense  []int64  `json:"expense"`
	Days     []string `json:"days"`
	Earning  []int64  `json:"earning"`
	Spent    []int64  `json:"spent"`
}

type Transaction struct {
	Type      string    `json:"type"`
	Amount    int64     `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

var (
	db            *sql.DB
	mu            sync.Mutex
	cronScheduler *cron.Cron
	bgCtx         context.Context
	bgCancel      context.CancelFunc
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Запуск финансового приложения")

	bgCtx, bgCancel = context.WithCancel(context.Background())
	defer bgCancel()

	if err := initDB(); err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer db.Close()

	if err := createTables(bgCtx); err != nil {
		log.Printf("Ошибка создания таблиц: %v", err)
	}

	initScheduler(bgCtx)
	startServer(bgCtx)
}

func initDB() error {
	connStr := "postgres://postgres:postgres@localhost:5432/finance_db?sslmode=disable"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("DB open error: %v", err)
	}

	ctx, cancel := context.WithTimeout(bgCtx, 5*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		return fmt.Errorf("DB ping error: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	return nil
}

func createTables(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS cards (
		id SERIAL PRIMARY KEY,
		savings BIGINT DEFAULT 0,
		income BIGINT DEFAULT 0,
		expense BIGINT DEFAULT 0,
		balance BIGINT GENERATED ALWAYS AS (income - expense) STORED,
		last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS charts (
		id SERIAL PRIMARY KEY,
		months JSONB DEFAULT '["Янв","Фев","Мар","Апр","Май","Июн","Июл","Авг","Сен","Окт","Ноя","Дек"]',
		income JSONB DEFAULT '[0,0,0,0,0,0,0,0,0,0,0,0]',
		expense JSONB DEFAULT '[0,0,0,0,0,0,0,0,0,0,0,0]',
		days JSONB DEFAULT '["Пн","Вт","Ср","Чт","Пт","Сб","Вс"]',
		earning JSONB DEFAULT '[0,0,0,0,0,0,0]',
		spent JSONB DEFAULT '[0,0,0,0,0,0,0]'
	);
	
	CREATE TABLE IF NOT EXISTS transactions (
		id SERIAL PRIMARY KEY,
		type VARCHAR(10) NOT NULL CHECK (type IN ('income', 'expense')),
		amount BIGINT NOT NULL,
		timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS transactions_timestamp_idx ON transactions(timestamp DESC);
	`
	_, err := db.ExecContext(ctx, query)
	return err
}

func startServer(ctx context.Context) {
	router := http.NewServeMux()

	fs := http.FileServer(http.Dir("static"))
	router.Handle("/", fs)

	router.HandleFunc("/api/cards", getCardsData)
	router.HandleFunc("/api/cards/update", updateCardsData)
	router.HandleFunc("/api/charts", getChartsData)
	router.HandleFunc("/api/transactions", transactionsHandler)

	server := &http.Server{
		Addr:         ":" + getPort(),
		Handler:      router,
		ReadTimeout:  serverTimeout,
		WriteTimeout: serverTimeout,
	}

	go func() {
		log.Printf("Сервер запущен на порту %s", getPort())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownServer(server)
}

func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	log.Println("Завершение работы сервера...")
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Ошибка graceful shutdown: %v", err)
	} else {
		log.Println("Сервер успешно остановлен")
	}
}

func getPort() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return defaultPort
}

func initScheduler(ctx context.Context) {
	cronScheduler = cron.New()

	_, err := cronScheduler.AddFunc("0 0 * * 1", func() {
		resetWeeklyChart(ctx)
		resetIncomeexpense(ctx)
	})
	if err != nil {
		log.Printf("Ошибка планировщика (неделя): %v", err)
	}

	_, err = cronScheduler.AddFunc("0 0 1 1 *", func() {
		resetYearlyChart(ctx)
	})
	if err != nil {
		log.Printf("Ошибка планировщика (год): %v", err)
	}

	cronScheduler.Start()
}

func resetWeeklyChart(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()

	_, err := db.ExecContext(ctx, `
		UPDATE charts SET earning = '[0,0,0,0,0,0,0]', spent = '[0,0,0,0,0,0,0]'
	`)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка сброса недельного графика: %v", err)
	}
}

func resetYearlyChart(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()

	_, err := db.ExecContext(ctx, `
		UPDATE charts SET income = '[0,0,0,0,0,0,0,0,0,0,0,0]', expense = '[0,0,0,0,0,0,0,0,0,0,0,0]'
	`)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка сброса годового графика: %v", err)
	}
}

func resetIncomeexpense(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()

	var balance, savings int64
	err := db.QueryRowContext(ctx, `
		SELECT balance, savings FROM cards ORDER BY last_updated DESC LIMIT 1
	`).Scan(&balance, &savings)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка получения данных для сброса: %v", err)
		return
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO cards (savings, income, expense) VALUES ($1, $2, $3)
	`, savings+balance, 0, 0)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка сброса доходов и расходов: %v", err)
	}
}

// ==================== API HANDLERS ====================

func getCardsData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	var data CardData
	err := db.QueryRowContext(r.Context(), `
		SELECT savings, income, expense, balance FROM cards ORDER BY last_updated DESC LIMIT 1
	`).Scan(&data.Savings, &data.Income, &data.Expense, &data.Balance)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения данных: %v", err), http.StatusInternalServerError)
		return
	}

	respondWithJSON(w, data)
}

func updateCardsData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Type          string `json:"type"`
		Value         int64  `json:"value"`
		IsIncremental bool   `json:"isIncremental"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Неверный формат данных", http.StatusBadRequest)
		return
	}

	if request.Value < 0 || request.Value > maxValue || float64(request.Value) != math.Trunc(float64(request.Value)) {
		http.Error(w, "Недопустимое значение (целое от 0 до 99999999)", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	var query string
	switch request.Type {
	case "savings":
		if request.IsIncremental {
			query = "UPDATE cards SET savings = savings + $1"
		} else {
			query = "UPDATE cards SET savings = $1"
		}
	case "income":
		query = "UPDATE cards SET income = income + $1"
		go safeUpdateChart("income", request.Value)
	case "expense":
		query = "UPDATE cards SET expense = expense + $1"
		go safeUpdateChart("expense", request.Value)
	default:
		http.Error(w, "Неверный тип операции", http.StatusBadRequest)
		return
	}

	if _, err := db.ExecContext(r.Context(), query, request.Value); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка обновления: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Данные обновлены"))
}

func transactionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getTransactions(w, r)
	case "POST":
		addTransaction(w, r)
	default:
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
	}
}

func getTransactions(w http.ResponseWriter, r *http.Request) {
	rows, err := db.QueryContext(r.Context(), `
		SELECT type, amount, timestamp 
		FROM transactions 
		ORDER BY timestamp DESC
		LIMIT 100
	`)
	if err != nil {
		log.Printf("Database query error: %v", err)
		http.Error(w, "Failed to query transactions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.Type, &t.Amount, &t.Timestamp); err != nil {
			log.Printf("Row scanning error: %v", err)
			http.Error(w, "Failed to process transaction data", http.StatusInternalServerError)
			return
		}
		transactions = append(transactions, t)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Rows iteration error: %v", err)
		http.Error(w, "Failed to process all transactions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(transactions); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode transactions", http.StatusInternalServerError)
	}
}

func addTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	var t Transaction
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		respondWithError(w, http.StatusBadRequest, "Неверный формат данных транзакции")
		return
	}

	t.Type = strings.ToLower(t.Type)
	if t.Type != "income" && t.Type != "expense" {
		http.Error(w, "Неверный тип транзакции", http.StatusBadRequest)
		return
	}
	if t.Amount <= 0 || t.Amount > maxValue {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Недопустимая сумма (1–%d)", maxValue))
		return
	}
	if t.Timestamp.IsZero() {
		t.Timestamp = time.Now()
	}

	// Начинаем транзакцию
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 1. Добавляем транзакцию
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO transactions (type, amount, timestamp)
		VALUES ($1, $2, $3)
	`, t.Type, t.Amount, t.Timestamp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Обновляем карточки
	var cardQuery string
	if t.Type == "income" {
		cardQuery = "UPDATE cards SET income = income + $1"
	} else {
		cardQuery = "UPDATE cards SET expense = expense + $1"
	}

	_, err = tx.ExecContext(r.Context(), cardQuery, t.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Обновляем графики
	month := int(t.Timestamp.Month()) - 1
	day := int(t.Timestamp.Weekday())
	if day == 0 {
		day = 6
	} else {
		day--
	}

	var chartQuery string
	if t.Type == "income" {
		chartQuery = `
			UPDATE charts 
			SET 
				income = jsonb_set(
					income, 
					array[$1::text], 
					to_jsonb((COALESCE((income->>$1::text)::bigint, 0) + $2)::bigint)
				),
				earning = jsonb_set(
					earning, 
					array[$3::text], 
					to_jsonb((COALESCE((earning->>$3::text)::bigint, 0) + $2)::bigint)
				)
		`
	} else {
		chartQuery = `
			UPDATE charts 
			SET 
				expense = jsonb_set(
					expense, 
					array[$1::text], 
					to_jsonb((COALESCE((expense->>$1::text)::bigint, 0) + $2)::bigint)
				),
				spent = jsonb_set(
					spent, 
					array[$3::text], 
					to_jsonb((COALESCE((spent->>$3::text)::bigint, 0) + $2)::bigint)
				)
		`
	}

	_, err = tx.ExecContext(r.Context(), chartQuery, month, t.Amount, day)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJSON(w, map[string]interface{}{
		"status":  "success",
		"message": "Транзакция сохранена",
		"data":    t,
	})
}

func getChartsData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	var data ChartData
	var months, income, expense, days, earning, spent []byte
	err := db.QueryRowContext(r.Context(), `
		SELECT months, income, expense, days, earning, spent FROM charts LIMIT 1
	`).Scan(&months, &income, &expense, &days, &earning, &spent)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения графиков: %v", err), http.StatusInternalServerError)
		return
	}

	json.Unmarshal(months, &data.Months)
	json.Unmarshal(income, &data.Income)
	json.Unmarshal(expense, &data.Expense)
	json.Unmarshal(days, &data.Days)
	json.Unmarshal(earning, &data.Earning)
	json.Unmarshal(spent, &data.Spent)

	respondWithJSON(w, data)
}

// ==================== UTILITIES ====================

func safeUpdateChart(operationType string, value int64) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered in safeUpdateChart: %v", r)
		}
	}()
	updateChartData(operationType, value)
}

func updateChartData(operationType string, value int64) {
	mu.Lock()
	defer mu.Unlock()

	now := time.Now()
	month := int(now.Month()) - 1
	day := int(now.Weekday())
	if day == 0 {
		day = 6
	} else {
		day--
	}

	var dayQuery, monthQuery string
	if operationType == "income" {
		dayQuery = "UPDATE charts SET earning = jsonb_set(earning, array[$1::text], to_jsonb((COALESCE((earning->>$1)::bigint, 0) + $2)::bigint))"
		monthQuery = "UPDATE charts SET income = jsonb_set(income, array[$1::text], to_jsonb((COALESCE((income->>$1)::bigint, 0) + $2)::bigint))"
	} else {
		dayQuery = "UPDATE charts SET spent = jsonb_set(spent, array[$1::text], to_jsonb((COALESCE((spent->>$1)::bigint, 0) + $2)::bigint))"
		monthQuery = "UPDATE charts SET expense = jsonb_set(expense, array[$1::text], to_jsonb((COALESCE((expense->>$1)::bigint, 0) + $2)::bigint))"
	}

	if _, err := db.ExecContext(bgCtx, dayQuery, day, value); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка обновления дневного графика: %v", err)
	}
	if _, err := db.ExecContext(bgCtx, monthQuery, month, value); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка обновления месячного графика: %v", err)
	}
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}