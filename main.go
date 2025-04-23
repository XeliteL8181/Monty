/**
 * Файл: main.go
 * Оптимизированная версия финансового приложения с транзакциями
 */

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
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
)

// Константы приложения
const (
	maxValue        int64         = 99999999
	defaultPort     string        = "10000"
	serverTimeout   time.Duration = 15 * time.Second
	shutdownTimeout time.Duration = 5 * time.Second
)

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

type Transaction struct {
	Type      string    `json:"type"` // "income" или "expense"
	Amount    int64     `json:"amount"`
	Category  string    `json:"category"`
	Timestamp time.Time `json:"timestamp"`
}

// Глобальные переменные
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

	// Инициализация контекста
	bgCtx, bgCancel = context.WithCancel(context.Background())
	defer bgCancel()

	// Инициализация базы данных
	if err := initDB(); err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer db.Close()

	// Создание таблиц при их отсутствии
	if err := createTables(bgCtx); err != nil {
		log.Printf("Ошибка создания таблиц: %v", err)
	}

	// Инициализация планировщика
	initScheduler(bgCtx)

	// Запуск сервера
	startServer(bgCtx)
}

// Обработка сигналов завершения
func handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("Получен сигнал: %v", sig)
	bgCancel()
}

// Инициализация планировщика для сброса графиков
func initScheduler(ctx context.Context) {
	cronScheduler = cron.New()

	// Сброс недельного графика и обнуление доходов/расходов (с сохранением баланса) каждый понедельник в 00:00
	_, err := cronScheduler.AddFunc("0 0 * * 1", func() {
		resetWeeklyChart(ctx)
		resetIncomeExpenses(ctx)
	})
	if err != nil {
		log.Printf("Ошибка добавления задачи сброса недельного графика: %v", err)
	}

	// Сброс годового графика 1 января в 00:00
	_, err = cronScheduler.AddFunc("0 0 1 1 *", func() {
		resetYearlyChart(ctx)
	})
	if err != nil {
		log.Printf("Ошибка добавления задачи сброса годового графика: %v", err)
	}

	cronScheduler.Start()
}

// Сброс недельного графика
func resetWeeklyChart(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()

	_, err := db.ExecContext(ctx, `
		 UPDATE charts 
		 SET earning = '[0,0,0,0,0,0,0]', 
			 spent = '[0,0,0,0,0,0,0]'
	 `)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка сброса недельного графика: %v", err)
	}
}

// Сброс годового графика
func resetYearlyChart(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()

	_, err := db.ExecContext(ctx, `
		 UPDATE charts 
		 SET income = '[0,0,0,0,0,0,0,0,0,0,0,0]', 
			 expenses = '[0,0,0,0,0,0,0,0,0,0,0,0]'
	 `)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка сброса годового графика: %v", err)
	}
}

// Сброс доходов и расходов с сохранением баланса
func resetIncomeExpenses(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()

	// Получаем текущий баланс и накопления
	var balance, savings int64
	err := db.QueryRowContext(ctx, `
		 SELECT balance, savings 
		 FROM cards 
		 ORDER BY last_updated DESC 
		 LIMIT 1
	 `).Scan(&balance, &savings)

	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Printf("Ошибка получения данных для сброса: %v", err)
		}
		return
	}

	// Создаем новую запись с нулевыми доходами и расходами, но сохраняем баланс через savings
	_, err = db.ExecContext(ctx, `
		 INSERT INTO cards (savings, income, expenses) 
		 VALUES ($1, $2, $3)
	 `, savings+balance, 0, 0)

	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка сброса доходов и расходов: %v", err)
	}
}

// Инициализация подключения к БД
func initDB() error {
	connStr := "postgres://postgres:postgres@localhost:5432/finance_db?sslmode=disable"

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("DB open error: %v", err)
	}

	// Проверка соединения с БД
	ctx, cancel := context.WithTimeout(bgCtx, 5*time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		return fmt.Errorf("DB ping error: %v", err)
	}

	// Установка параметров пула соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return nil
}

// Создание таблиц в БД
func createTables(ctx context.Context) error {
	query := `
		 CREATE TABLE IF NOT EXISTS cards (
			 id SERIAL PRIMARY KEY,
			 savings BIGINT DEFAULT 0,
			 income BIGINT DEFAULT 0,
			 expenses BIGINT DEFAULT 0,
			 balance BIGINT GENERATED ALWAYS AS (income - expenses) STORED,
			 last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		 );
		 
		 CREATE TABLE IF NOT EXISTS charts (
			 id SERIAL PRIMARY KEY,
			 months JSONB DEFAULT '["Янв","Фев","Мар","Апр","Май","Июн","Июл","Авг","Сен","Окт","Ноя","Дек"]',
			 income JSONB DEFAULT '[0,0,0,0,0,0,0,0,0,0,0,0]',
			 expenses JSONB DEFAULT '[0,0,0,0,0,0,0,0,0,0,0,0]',
			 days JSONB DEFAULT '["Пн","Вт","Ср","Чт","Пт","Сб","Вс"]',
			 earning JSONB DEFAULT '[0,0,0,0,0,0,0]',
			 spent JSONB DEFAULT '[0,0,0,0,0,0,0]'
		 );
 
		 CREATE TABLE IF NOT EXISTS transactions (
			 id SERIAL PRIMARY KEY,
			 type VARCHAR(10) NOT NULL CHECK (type IN ('income', 'expense')),
			 amount BIGINT NOT NULL,
			 category VARCHAR(50),
			 timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		 );
 
		 CREATE INDEX IF NOT EXISTS transactions_timestamp_idx ON transactions(timestamp DESC);
	 `

	_, err := db.ExecContext(ctx, query)
	return err
}

// Запуск HTTP сервера
func startServer(ctx context.Context) {
	router := http.NewServeMux()

	// Обработка статических файлов
	fs := http.FileServer(http.Dir("static"))
	router.Handle("/", fs)

	// API endpoints
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

	// Запуск сервера в горутине
	go func() {
		log.Printf("Сервер запущен на порту %s", getPort())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	// Ожидание сигнала завершения
	<-ctx.Done()
	shutdownServer(server)
}

func getPort() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return defaultPort
}

// Graceful shutdown сервера
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

// ==================== ОБРАБОТЧИКИ API ====================

// Получение данных карточек
func getCardsData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	var data CardData
	err := db.QueryRowContext(r.Context(), `
		 SELECT savings, income, expenses, balance 
		 FROM cards 
		 ORDER BY last_updated DESC 
		 LIMIT 1
	 `).Scan(&data.Savings, &data.Income, &data.Expenses, &data.Balance)

	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения данных: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// Обновление данных карточек
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
		http.Error(w, "Недопустимое значение (число должно быть целым, и находиться в диапозоне от 0 до 99999999)", http.StatusBadRequest)
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
	case "expenses":
		query = "UPDATE cards SET expenses = expenses + $1"
		go safeUpdateChart("expenses", request.Value)
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

// Безопасное обновление графиков с обработкой паники
func safeUpdateChart(operationType string, value int64) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered in safeUpdateChart: %v", r)
		}
	}()

	updateChartData(operationType, value)
}

// Обновление данных графиков
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

	// Получаем текущие значения для дня и месяца
	var currentDayValue, currentMonthValue int64
	var dayData, monthData []byte

	// Для недельного графика
	err := db.QueryRowContext(bgCtx, `
        SELECT 
            COALESCE((earning->>$1)::bigint, 0) as day_value,
            earning
        FROM charts
        LIMIT 1
    `, day).Scan(&currentDayValue, &dayData)

	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка получения данных дня: %v", err)
		return
	}

	// Для месячного графика
	err = db.QueryRowContext(bgCtx, `
        SELECT 
            COALESCE((income->>$1)::bigint, 0) as month_value,
            income
        FROM charts
        LIMIT 1
    `, month).Scan(&currentMonthValue, &monthData)

	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка получения данных месяца: %v", err)
		return
	}

	// Обновляем значения
	// Обновляем значения
	var query string
	if operationType == "income" {
		query = `
        UPDATE charts 
        SET 
            income = jsonb_set(
                income, 
                array[$1::text], 
                to_jsonb(($2 + $3)::bigint)
            ),
            earning = jsonb_set(
                earning, 
                array[$4::text], 
                to_jsonb(($2 + $5)::bigint)
            )
    `
	} else {
		query = `
        UPDATE charts 
        SET 
            expenses = jsonb_set(
                expenses, 
                array[$1::text], 
                to_jsonb(($2 + $3)::bigint)
            ),
            spent = jsonb_set(
                spent, 
                array[$4::text], 
                to_jsonb(($2 + $5)::bigint)
            )
    `
	}
	// Выполняем обновление с суммарными значениями
	_, err = db.ExecContext(bgCtx, query,
		month, value, currentMonthValue,
		day, currentDayValue)

	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Ошибка обновления графиков: %v", err)
	}
}

// Обработчик транзакций
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

// Получение списка транзакций
func getTransactions(w http.ResponseWriter, r *http.Request) {
	rows, err := db.QueryContext(r.Context(), `
		 SELECT type, amount, category, timestamp 
		 FROM transactions 
		 ORDER BY timestamp DESC
		 LIMIT 100
	 `)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.Type, &t.Amount, &t.Category, &t.Timestamp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		transactions = append(transactions, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transactions)
}

// Добавление новой транзакции
func addTransaction(w http.ResponseWriter, r *http.Request) {
	var t Transaction
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Валидация данных
	if t.Type != "income" && t.Type != "expense" {
		http.Error(w, "Неверный тип транзакции", http.StatusBadRequest)
		return
	}

	if t.Amount <= 0 || t.Amount > maxValue {
		http.Error(w, "Недопустимая сумма транзакции", http.StatusBadRequest)
		return
	}

	// Установка текущего времени, если не указано
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
		 INSERT INTO transactions (type, amount, category, timestamp)
		 VALUES ($1, $2, $3, $4)
	 `, t.Type, t.Amount, t.Category, t.Timestamp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Обновляем карточки
	var cardQuery string
	if t.Type == "income" {
		cardQuery = "UPDATE cards SET income = income + $1"
	} else {
		cardQuery = "UPDATE cards SET expenses = expenses + $1"
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
				expenses = jsonb_set(
					expenses, 
					array[$1::text], 
					to_jsonb((COALESCE((expenses->>$1::text)::bigint, 0) + $2)::bigint)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

// Получение данных графиков
func getChartsData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	var data ChartData
	var months, income, expenses, days, earning, spent []byte

	err := db.QueryRowContext(r.Context(), `
		 SELECT months, income, expenses, days, earning, spent
		 FROM charts
		 LIMIT 1
	 `).Scan(&months, &income, &expenses, &days, &earning, &spent)

	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка отрисовки графиков: %v", err), http.StatusInternalServerError)
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
