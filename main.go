package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
)

// Структуры данных
type FinancialData struct {
	Savings  float64 `json:"savings"`
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
}

type ChartData struct {
	Months   []string `json:"months"`
	Income   []int    `json:"income"`
	Expenses []int    `json:"expenses"`
	Days     []string `json:"days"`
	Earning  []int    `json:"earning"`
	Spent    []int    `json:"spent"`
}

type AppState struct {
	Financial FinancialData `json:"financial"`
	Charts    ChartData     `json:"charts"`
}

var (
	state AppState
	mu    sync.Mutex
)

func init() {
	state = AppState{
		Financial: FinancialData{
			Savings:  0,
			Income:   0,
			Expenses: 0,
		},
		Charts: ChartData{
			Months:   []string{"Янв", "Фев", "Март", "Апр", "Май", "Июнь", "Июль", "Авг", "Сен", "Окт", "Нояб", "Дек"},
			Income:   []int{55400, 55000, 60000, 100000, 150000, 80000, 70000, 55000, 75000, 100000, 65000, 130000},
			Expenses: []int{40000, 35000, 120000, 10000, 70000, 190000, 20000, 25000, 85000, 20000, 35000, 175000},
			Days:     []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"},
			Earning:  []int{1800, 2200, 2500, 1900, 2300, 1700, 2100},
			Spent:    []int{2200, 900, 1400, 2100, 1300, 1800, 1000},
		},
	}
	loadData()
}

func main() {
	// Статические файлы
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// HTML страницы
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/Monty0.html")
	})
	http.HandleFunc("/Monty1.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/Monty1.html")
	})

	// API endpoints
	http.HandleFunc("/api/data", handleData)
	http.HandleFunc("/api/update", handleUpdate)
	http.HandleFunc("/api/charts", handleCharts)
	http.HandleFunc("/api/update-charts", handleUpdateCharts)

	// Запуск сервера
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Сервер запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Обработчики API
func handleData(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	json.NewEncoder(w).Encode(state.Financial)
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data FinancialData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mu.Lock()
	state.Financial = data
	saveData()
	mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func handleCharts(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	json.NewEncoder(w).Encode(state.Charts)
}

func handleUpdateCharts(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data ChartData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mu.Lock()
	state.Charts = data
	saveData()
	mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

// Работа с данными
func saveData() {
	file, err := os.Create("data.json")
	if err != nil {
		log.Println("Save error:", err)
		return
	}
	defer file.Close()
	json.NewEncoder(file).Encode(state)
}

func loadData() {
	file, err := os.Open("data.json")
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Println("Load error:", err)
		return
	}
	defer file.Close()
	json.NewDecoder(file).Decode(&state)
}