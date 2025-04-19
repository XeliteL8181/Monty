/**
 * Файл: main.go
 * Полная версия финансового приложения
 * Оптимизировано для развертывания на Render.com
 */

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq" // Драйвер PostgreSQL
)

// ==================== КОНСТАНТЫ И НАСТРОЙКИ ====================

const (
	maxValue        int64         = 99999999
	defaultPort     string        = "10000"
	serverTimeout   time.Duration = 15 * time.Second
	dbInitTimeout   time.Duration = 30 * time.Second
	shutdownTimeout time.Duration = 5 * time.Second
)

// ==================== СТРУКТУРЫ ДАННЫХ ====================

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

// ==================== ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ ====================

var (
	db  *sql.DB
	mu  sync.Mutex
	ctx context.Context
)

// ==================== ОСНОВНЫЕ ФУНКЦИИ ====================

func main() {
	configureLogger()
	log.Println("Запуск финансового приложения")

	// Инициализация контекста с обработкой прерываний
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleShutdown(cancel)

	// Инициализация базы данных
	if err := initDB(); err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer closeDB()

	// Инициализация структуры БД
	if err := initDatabaseStructure(ctx); err != nil {
		log.Printf("Ошибка инициализации структуры БД: %v", err)
	}

	// Запуск сервера
	startServer(ctx)
}

func configureLogger() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func handleShutdown(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("Получен сигнал: %v", sig)
	cancel()
}

// ==================== РАБОТА С БАЗОЙ ДАННЫХ ====================

func initDB() error {
	connStr := getDBConnectionString()
	var err error

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %v", err)
	}

	configureDBPool()
	return testDBConnection()
}

func getDBConnectionString() string {
	if connStr := os.Getenv("DATABASE_URL"); connStr != "" {
		return connStr
	}
	return "postgres://finance_user:your_password@localhost:5432/finance_db?sslmode=disable"
}

func configureDBPool() {
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)
}

func testDBConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func closeDB() {
	if err := db.Close(); err != nil {
		log.Printf("Ошибка закрытия БД: %v", err)
	}
}

func initDatabaseStructure(ctx context.Context) error {
	if err := createTables(ctx); err != nil {
		return err
	}
	return initDefaultData(ctx)
}

func createTables(ctx context.Context) error {
	log.Println("Создание таблиц в базе данных...")

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
	 );`

	_, err := db.ExecContext(ctx, query)
	return err
}

func initDefaultData(ctx context.Context) error {
	if err := initCardsData(ctx); err != nil {
		return err
	}
	return initChartsData(ctx)
}

func initCardsData(ctx context.Context) error {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cards").Scan(&count); err != nil {
		return err
	}

	if count == 0 {
		_, err := db.ExecContext(ctx, "INSERT INTO cards (savings, income, expenses) VALUES (0, 0, 0)")
		return err
	}
	return nil
}

func initChartsData(ctx context.Context) error {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM charts").Scan(&count); err != nil {
		return err
	}

	if count == 0 {
		_, err := db.ExecContext(ctx, `
			 INSERT INTO charts (months, income, expenses, days, earning, spent)
			 VALUES (
				 '["Янв","Фев","Мар","Апр","Май","Июн","Июл","Авг","Сен","Окт","Ноя","Дек"]',
				 '[0,0,0,0,0,0,0,0,0,0,0,0]',
				 '[0,0,0,0,0,0,0,0,0,0,0,0]',
				 '["Пн","Вт","Ср","Чт","Пт","Сб","Вс"]',
				 '[0,0,0,0,0,0,0]',
				 '[0,0,0,0,0,0,0]'
			 )
		 `)
		return err
	}
	return nil
}

// ==================== HTTP СЕРВЕР ====================

func startServer(ctx context.Context) {
	router := setupRouter()
	port := getPort()

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  serverTimeout,
		WriteTimeout: serverTimeout,
		IdleTimeout:  2 * serverTimeout,
	}

	go runServer(server, port)
	waitForShutdown(ctx, server)
}

func getPort() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return defaultPort
}

func runServer(server *http.Server, port string) {
	log.Printf("Сервер запущен на порту %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Ошибка сервера: %v", err)
	}
}

func waitForShutdown(ctx context.Context, server *http.Server) {
	<-ctx.Done()
	shutdownServer(server)
}

func shutdownServer(server *http.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	log.Println("Завершение работы сервера...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Ошибка graceful shutdown: %v", err)
	} else {
		log.Println("Сервер успешно остановлен")
	}
}

func setupRouter() *http.ServeMux {
	router := http.NewServeMux()

	// Статические файлы
	fs := http.FileServer(http.Dir("static"))
	router.Handle("/static/", http.StripPrefix("/static/", fs))

	// Главная страница
	router.HandleFunc("/", handleIndex)

	// API endpoints
	api := http.NewServeMux()
	api.HandleFunc("/cards", getCardsData)
	api.HandleFunc("/cards/update", updateCardsData)
	api.HandleFunc("/cards/reset", resetCardsData)
	api.HandleFunc("/cards/history", getHistoryData)
	api.HandleFunc("/charts", getChartsData)
	router.Handle("/api/", http.StripPrefix("/api", api))

	// Health check
	router.HandleFunc("/health", healthCheck)

	return router
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "static/index.html")
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		http.Error(w, "Соединение с БД не активно", http.StatusServiceUnavailable)
		return
	}

	respondJSON(w, map[string]string{"status": "OK"})
}

// ==================== ОБРАБОТЧИКИ API ====================

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
		respondError(w, "Ошибка получения данных карточек", err, http.StatusInternalServerError)
		return
	}

	respondJSON(w, data)
}

func updateCardsData(w http.ResponseWriter, r *http.Request) {
	// Объявляем структуру для входных данных
	type UpdateRequest struct {
		Type          string `json:"type"`
		Value         int64  `json:"value"`
		IsIncremental bool   `json:"isIncremental"`
	}

	// Объявляем структуру для передачи в функции
	type UpdateData struct {
		Type          string
		Value         int64
		IsIncremental bool
	}

	if r.Method != "POST" {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	// Парсим JSON в переменную updateRequest
	var updateRequest UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&updateRequest); err != nil {
		respondError(w, "Ошибка разбора JSON", err, http.StatusBadRequest)
		return
	}

	// Конвертируем в нужный тип
	update := UpdateData{
		Type:          updateRequest.Type,
		Value:         updateRequest.Value,
		IsIncremental: updateRequest.IsIncremental,
	}

	if update.Value < 0 || update.Value > maxValue {
		http.Error(w, "Значение должно быть от 0 до 99999999", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		respondError(w, "Ошибка начала транзакции", err, http.StatusInternalServerError)
		return
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if err := updateCardValues(r.Context(), tx, update); err != nil {
		respondError(w, "Ошибка обновления данных", err, http.StatusInternalServerError)
		return
	}

	if update.Type == "income" || update.Type == "expenses" {
		if err := updateChartsData(r.Context(), tx, update.Type, update.Value); err != nil {
			log.Printf("Ошибка обновления графиков: %v", err)
		}
	}

	if err := saveHistoryRecord(r.Context(), tx, update); err != nil {
		log.Printf("Ошибка сохранения истории: %v", err)
	}

	if err := tx.Commit(); err != nil {
		respondError(w, "Ошибка сохранения изменений", err, http.StatusInternalServerError)
		return
	}

	respondSuccess(w, "Данные успешно обновлены")
}

func updateCardValues(ctx context.Context, tx *sql.Tx, update struct {
	Type          string
	Value         int64
	IsIncremental bool
}) error {
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
		return fmt.Errorf("неверный тип операции: %s", update.Type)
	}

	_, err := tx.ExecContext(ctx, query, update.Value)
	return err
}

func updateChartsData(ctx context.Context, tx *sql.Tx, updateType string, value int64) error {
	now := time.Now()
	month := int(now.Month()) - 1
	day := int(now.Weekday())
	weekday := (day + 6) % 7

	// Обновление месячных данных
	field := "income"
	if updateType == "expenses" {
		field = "expenses"
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
		 UPDATE charts 
		 SET %s = jsonb_set(%s, '{%d}', to_jsonb($1::bigint + (%s->>'%d')::bigint)
		 WHERE id = 1
	 `, field, field, month, field, month), value); err != nil {
		return fmt.Errorf("месячные данные: %v", err)
	}

	// Обновление недельных данных
	weeklyField := "earning"
	if updateType == "expenses" {
		weeklyField = "spent"
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
		 UPDATE charts 
		 SET %s = jsonb_set(%s, '{%d}', to_jsonb($1::bigint + (%s->>'%d')::bigint)
		 WHERE id = 1
	 `, weeklyField, weeklyField, weekday, weeklyField, weekday), value); err != nil {
		return fmt.Errorf("недельные данные: %v", err)
	}

	return nil
}

func saveHistoryRecord(ctx context.Context, tx *sql.Tx, update struct {
	Type          string
	Value         int64
	IsIncremental bool
}) error {
	record := HistoryRecord{
		Type:          update.Type,
		Value:         update.Value,
		IsIncremental: update.IsIncremental,
		Timestamp:     time.Now(),
	}

	jsonData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга истории: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		 INSERT INTO card_history (card_id, data) 
		 VALUES (1, $1)
	 `, jsonData)
	return err
}

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
		 ORDER BY updated_at DESC
		 LIMIT 1
	 `).Scan(&months, &income, &expenses, &days, &earning, &spent)

	if err != nil {
		respondError(w, "Ошибка получения данных графиков", err, http.StatusInternalServerError)
		return
	}

	if err := json.Unmarshal(months, &data.Months); err != nil {
		respondError(w, "Ошибка разбора месяцев", err, http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(income, &data.Income); err != nil {
		respondError(w, "Ошибка разбора доходов", err, http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(expenses, &data.Expenses); err != nil {
		respondError(w, "Ошибка разбора расходов", err, http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(days, &data.Days); err != nil {
		respondError(w, "Ошибка разбора дней", err, http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(earning, &data.Earning); err != nil {
		respondError(w, "Ошибка разбора доходов по дням", err, http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(spent, &data.Spent); err != nil {
		respondError(w, "Ошибка разбора расходов по дням", err, http.StatusInternalServerError)
		return
	}

	respondJSON(w, data)
}

func resetCardsData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		respondError(w, "Ошибка начала транзакции", err, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(r.Context(), "UPDATE cards SET savings = 0, income = 0, expenses = 0"); err != nil {
		respondError(w, "Ошибка сброса карточек", err, http.StatusInternalServerError)
		return
	}

	if _, err := tx.ExecContext(r.Context(), `
		 UPDATE charts 
		 SET 
			 income = '[0,0,0,0,0,0,0,0,0,0,0,0]',
			 expenses = '[0,0,0,0,0,0,0,0,0,0,0,0]',
			 earning = '[0,0,0,0,0,0,0]',
			 spent = '[0,0,0,0,0,0,0]'
		 WHERE id = 1
	 `); err != nil {
		respondError(w, "Ошибка сброса графиков", err, http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		respondError(w, "Ошибка коммита транзакции", err, http.StatusInternalServerError)
		return
	}

	respondSuccess(w, "Все данные успешно сброшены")
}

func getHistoryData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	rows, err := db.QueryContext(r.Context(), `
		 SELECT data 
		 FROM card_history 
		 ORDER BY changed_at DESC
		 LIMIT 100
	 `)
	if err != nil {
		respondError(w, "Ошибка получения истории", err, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []HistoryRecord
	for rows.Next() {
		var jsonData []byte
		var record HistoryRecord

		if err := rows.Scan(&jsonData); err != nil {
			log.Printf("Ошибка сканирования истории: %v", err)
			continue
		}

		if err := json.Unmarshal(jsonData, &record); err != nil {
			log.Printf("Ошибка разбора JSON истории: %v", err)
			continue
		}

		history = append(history, record)
	}

	if err := rows.Err(); err != nil {
		respondError(w, "Ошибка обработки истории", err, http.StatusInternalServerError)
		return
	}

	respondJSON(w, history)
}

// ==================== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ====================

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Ошибка кодирования JSON: %v", err)
	}
}

func respondError(w http.ResponseWriter, message string, err error, statusCode int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, fmt.Sprintf("%s: %v", message, err), statusCode)
}

func respondSuccess(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(message))
}
