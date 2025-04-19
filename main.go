/**
 * Файл: main.go
 * Оптимизированная версия финансового приложения
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
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq" // Драйвер PostgreSQL
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

// Глобальные переменные
var (
	db *sql.DB
	mu sync.Mutex
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Запуск финансового приложения")

	// Инициализация контекста с обработкой прерываний
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleShutdown(cancel)

	// Инициализация базы данных
	if err := initDB(); err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer db.Close()

	// Создание таблиц при их отсутствии
	if err := createTables(ctx); err != nil {
		log.Printf("Ошибка создания таблиц: %v", err)
	}

	// Запуск сервера
	startServer(ctx)
}

// Обработка сигналов завершения
func handleShutdown(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("Получен сигнал: %v", sig)
	cancel()
}

// Инициализация подключения к БД
func initDB() error {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://finance_user:your_password@localhost:5432/finance_db?sslmode=disable"
	}

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("ошибка подключения: %v", err)
	}

	// Настройка пула соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db.Ping()
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
	 );`

	_, err := db.ExecContext(ctx, query)
	return err
}

// Главный обработчик
type handler struct {
	mux *http.ServeMux
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Сначала проверяем API пути
	if strings.HasPrefix(r.URL.Path, "/api") {
		h.mux.ServeHTTP(w, r)
		return
	}

	// Все остальное - статические файлы
	http.FileServer(http.Dir("static")).ServeHTTP(w, r)
}

// Запуск HTTP сервера
func startServer(ctx context.Context) {
	router := http.NewServeMux()

	// Обработка статических файлов
	fs := http.FileServer(http.Dir("static"))
	router.Handle("/", fs) // Все запросы будут обрабатываться из папки static

	// API endpoints
	router.HandleFunc("/api/cards", getCardsData)
	router.HandleFunc("/api/cards/update", updateCardsData)
	router.HandleFunc("/api/charts", getChartsData)

	server := &http.Server{
		Addr:         ":" + getPort(),
		Handler:      router,
		ReadTimeout:  serverTimeout,
		WriteTimeout: serverTimeout,
	}

	go func() {
		log.Printf("Сервер запущен на порту %s", getPort())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

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

	if request.Value < 0 || request.Value > maxValue {
		http.Error(w, "Недопустимое значение", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Обновление данных в БД
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
	case "expenses":
		query = "UPDATE cards SET expenses = expenses + $1"
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
