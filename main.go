/**
 * Файл: main.go
 * Основной серверный скрипт приложения для учета финансов
 * Реализует REST API, работу с базой данных и бизнес-логику
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
	 "sync"
	 "time"
 
	 _ "github.com/lib/pq" // Драйвер PostgreSQL
 )
 
 // ==================== КОНСТАНТЫ И СТРУКТУРЫ ====================
 
 const maxValue int64 = 99999999 // Максимальное значение для операций
 
 // Данные карточек (хранятся в БД)
 type CardData struct {
	 Savings  int64 `json:"savings"`  // Накопления
	 Income   int64 `json:"income"`   // Доходы
	 Expenses int64 `json:"expenses"` // Расходы
	 Balance  int64 `json:"balance"`  // Баланс (вычисляемое поле)
 }
 
 // Данные для графиков
 type ChartData struct {
	 Months   []string `json:"months"`   // Названия месяцев
	 Income   []int64  `json:"income"`   // Доходы по месяцам
	 Expenses []int64  `json:"expenses"` // Расходы по месяцам
	 Days     []string `json:"days"`     // Дни недели
	 Earning  []int64  `json:"earning"`  // Доходы по дням недели
	 Spent    []int64  `json:"spent"`    // Расходы по дням недели
 }
 
 // Запись истории изменений
 type HistoryRecord struct {
	 Type          string    `json:"type"`          // Тип операции
	 Value         int64     `json:"value"`         // Сумма
	 IsIncremental bool      `json:"isIncremental"` // Добавление или замена
	 Timestamp     time.Time `json:"timestamp"`     // Время операции
 }
 
 // Глобальные переменные
 var (
	 db *sql.DB     // Подключение к БД
	 mu sync.Mutex  // Мьютекс для потокобезопасности
 )
 
 // ==================== ИНИЦИАЛИЗАЦИЯ БАЗЫ ДАННЫХ ====================
 
 /**
  * Инициализация подключения к PostgreSQL
  */
 func initDB() {
	 // Получение строки подключения из переменных окружения
	 connStr := os.Getenv("DATABASE_URL")
	 if connStr == "" {
		 // Значение по умолчанию для разработки
		 connStr = "postgres://finance_user:your_password@localhost:5432/finance_db?sslmode=disable"
	 }
 
	 var err error
	 // Открытие соединения с БД
	 db, err = sql.Open("postgres", connStr)
	 if err != nil {
		 log.Fatal("Failed to connect to database:", err)
	 }
 
	 // Настройка пула соединений
	 db.SetMaxOpenConns(10) // Максимальное число открытых соединений
	 db.SetMaxIdleConns(5)  // Максимальное число неактивных соединений
 
	 // Проверка соединения
	 ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	 defer cancel()
 
	 if err := db.PingContext(ctx); err != nil {
		 log.Fatal("Database ping failed:", err)
	 }
 
	 log.Println("Successfully connected to PostgreSQL")
 }
 
 /**
  * Создание таблиц при первом запуске
  */
 func createTables() {
	 log.Println("Creating database tables...")
 
	 // SQL запрос для создания таблиц
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
 
	 // Выполнение запроса
	 _, err := db.Exec(query)
	 if err != nil {
		 log.Fatal("Error creating tables:", err)
	 }
 
	 // Инициализация данных карточек, если таблица пуста
	 var count int
	 db.QueryRow("SELECT COUNT(*) FROM cards").Scan(&count)
	 if count == 0 {
		 _, err = db.Exec("INSERT INTO cards (savings, income, expenses) VALUES (0, 0, 0)")
		 if err != nil {
			 log.Fatal("Error initializing cards data:", err)
		 }
	 }
 
	 // Инициализация данных графиков, если таблица пуста
	 db.QueryRow("SELECT COUNT(*) FROM charts").Scan(&count)
	 if count == 0 {
		 _, err = db.Exec(`
			 INSERT INTO charts (months, income, expenses, days, earning, spent)
			 VALUES (
				 '["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]',
				 '[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]',
				 '[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]',
				 '["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]',
				 '[0, 0, 0, 0, 0, 0, 0]',
				 '[0, 0, 0, 0, 0, 0, 0]'
			 )
		 `)
		 if err != nil {
			 log.Fatal("Error initializing charts data:", err)
		 }
	 }
 
	 log.Println("Database tables initialized successfully")
 }
 
 // ==================== ОСНОВНАЯ ФУНКЦИЯ ====================
 
 func main() {
	 log.SetFlags(log.LstdFlags | log.Lshortfile)
	 log.Println("Starting application...")
 
	 initDB()         // Инициализация БД
	 defer db.Close() // Закрытие соединения при выходе
 
	 // Запуск создания таблиц в фоне с таймаутом
	 go func() {
		 ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		 defer cancel()
		 
		 done := make(chan struct{})
		 go func() {
			 createTables()
			 close(done)
		 }()
 
		 select {
		 case <-done:
			 log.Println("Tables created successfully")
		 case <-ctx.Done():
			 log.Println("Table creation timed out")
		 }
	 }()
 
	 // Настройка маршрутов
	 http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		 if r.URL.Path == "/" {
			 http.ServeFile(w, r, "static/index.html")
			 return
		 }
 
		 // Попытка обслужить статический файл
		 _, err := os.Stat("static" + r.URL.Path)
		 if os.IsNotExist(err) {
			 http.NotFound(w, r)
			 return
		 }
 
		 http.ServeFile(w, r, "static"+r.URL.Path)
	 })
 
	 // API endpoints
	 api := http.NewServeMux()
	 api.HandleFunc("/cards", getCardsData)           // Получение данных карточек
	 api.HandleFunc("/cards/update", updateCardsData)  // Обновление данных
	 api.HandleFunc("/cards/reset", resetCardsData)    // Сброс данных
	 api.HandleFunc("/cards/history", getHistoryData) // История изменений
	 api.HandleFunc("/charts", getChartsData)         // Данные графиков
	 
	 http.Handle("/api/", http.StripPrefix("/api", api))
 
	 // Health check endpoint
	 http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		 w.WriteHeader(http.StatusOK)
		 w.Write([]byte("OK"))
	 })
 
	 // Запуск сервера с таймаутами
	 port := os.Getenv("PORT")
	 if port == "" {
		 port = "10000"
	 }
 
	 server := &http.Server{
		 Addr:         ":" + port,
		 ReadTimeout:  15 * time.Second,
		 WriteTimeout: 15 * time.Second,
		 IdleTimeout:  60 * time.Second,
	 }
 
	 log.Printf("Server starting on port %s", port)
	 if err := server.ListenAndServe(); err != nil {
		 log.Fatal("Server error:", err)
	 }
 }
 
 // ==================== ОБРАБОТЧИКИ API ====================
 
 /**
  * Получение данных карточек
  */
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
 
 /**
  * Обновление данных карточек
  */
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
		 http.Error(w, "Value must be between 0 and 99999999", http.StatusBadRequest)
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
 
	 if update.Type == "income" || update.Type == "expenses" {
		 updateChartsData(update.Type, update.Value)
	 }
 
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
 
 /**
  * Обновление данных графиков
  */
 func updateChartsData(updateType string, value int64) {
	 now := time.Now()
	 month := int(now.Month()) - 1
	 day := int(now.Weekday())
	 weekday := (day + 6) % 7
 
	 var field string
	 if updateType == "income" {
		 field = "income"
	 } else {
		 field = "expenses"
	 }
 
	 _, err := db.Exec(fmt.Sprintf(`
		 UPDATE charts 
		 SET %s = jsonb_set(%s, '{%d}', to_jsonb($1::bigint + (%s->>'%d')::bigint)
		 WHERE id = 1
	 `, field, field, month, field, month), value)
	 if err != nil {
		 log.Println("Failed to update monthly chart data:", err)
	 }
 
	 weeklyField := "earning"
	 if updateType == "expenses" {
		 weeklyField = "spent"
	 }
 
	 _, err = db.Exec(fmt.Sprintf(`
		 UPDATE charts 
		 SET %s = jsonb_set(%s, '{%d}', to_jsonb($1::bigint + (%s->>'%d')::bigint))
		 WHERE id = 1
	 `, weeklyField, weeklyField, weekday, weeklyField, weekday), value)
	 if err != nil {
		 log.Println("Failed to update weekly chart data:", err)
	 }
 }
 
 /**
  * Получение данных графиков
  */
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
 
 /**
  * Сброс всех данных
  */
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
 
 /**
  * Получение истории изменений
  */
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