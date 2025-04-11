// Конфигурация API
const API_BASE_URL = '';

// DOM элементы
const elements = {
    savings: {
        display: document.getElementById('savings-button'),
        input: document.getElementById('savings-input'),
        button: document.getElementById('update-savings-btn')
    },
    income: {
        display: document.getElementById('income-button'),
        input: document.getElementById('income-input'),
        button: document.getElementById('add-income-btn')
    },
    expenses: {
        display: document.getElementById('expenses-button'),
        input: document.getElementById('expenses-input'),
        button: document.getElementById('add-expense-btn')
    },
    balance: {
        display: document.getElementById('balance-button')
    }
};

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    loadCardsData();
    loadChartsData();
    setupEventListeners();
});

// Настройка обработчиков событий
function setupEventListeners() {
    elements.savings.button.addEventListener('click', updateSavings);
    elements.income.button.addEventListener('click', addIncome);
    elements.expenses.button.addEventListener('click', addExpense);
}

// ==================== Работа с карточками ====================

// Загрузка данных карточек
async function loadCardsData() {
    try {
        const response = await fetch(`${API_BASE_URL}/api/cards`);
        if (!response.ok) throw new Error('Ошибка загрузки данных');
        
        const data = await response.json();
        updateCardsUI(data);
    } catch (error) {
        console.error('Ошибка:', error);
        showAlert('Ошибка загрузки данных', 'error');
    }
}

// Обновление интерфейса карточек
function updateCardsUI(data) {
    elements.savings.display.textContent = `${formatCurrency(data.savings)}`;
    elements.income.display.textContent = `${formatCurrency(data.income)}`;
    elements.expenses.display.textContent = `${formatCurrency(data.expenses)}`;
    elements.balance.display.textContent = `${formatCurrency(data.balance)}`;
}

// Обновление накоплений
async function updateSavings() {
    const value = parseFloat(elements.savings.input.value);
    if (isNaN(value)) {
        showAlert('Введите корректную сумму', 'warning');
        return;
    }

    try {
        await fetch(`${API_BASE_URL}/api/cards/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                type: 'savings', 
                value: value,
                isIncremental: false 
            })
        });
        
        elements.savings.input.value = '';
        loadCardsData();
        showAlert('Накопления обновлены', 'success');
    } catch (error) {
        console.error('Ошибка:', error);
        showAlert('Ошибка обновления', 'error');
    }
}

// Добавление дохода
async function addIncome() {
    const value = parseFloat(elements.income.input.value);
    if (isNaN(value) || value <= 0) {
        showAlert('Введите корректную сумму', 'warning');
        return;
    }

    try {
        await fetch(`${API_BASE_URL}/api/cards/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                type: 'income', 
                value: value,
                isIncremental: true 
            })
        });
        
        elements.income.input.value = '';
        loadCardsData();
        showAlert('Доход добавлен', 'success');
    } catch (error) {
        console.error('Ошибка:', error);
        showAlert('Ошибка добавления дохода', 'error');
    }
}

// Добавление расхода
async function addExpense() {
    const value = parseFloat(elements.expenses.input.value);
    if (isNaN(value) || value <= 0) {
        showAlert('Введите корректную сумму', 'warning');
        return;
    }

    try {
        await fetch(`${API_BASE_URL}/api/cards/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                type: 'expenses', 
                value: value,
                isIncremental: true 
            })
        });
        
        elements.expenses.input.value = '';
        loadCardsData();
        showAlert('Расход добавлен', 'success');
    } catch (error) {
        console.error('Ошибка:', error);
        showAlert('Ошибка добавления расхода', 'error');
    }
}

// ==================== Работа с графиками ====================

// Загрузка данных графиков
async function loadChartsData() {
    try {
        const response = await fetch(`${API_BASE_URL}/api/charts`);
        if (!response.ok) throw new Error('Ошибка загрузки графиков');
        
        const data = await response.json();
        renderCharts(data);
    } catch (error) {
        console.error('Ошибка загрузки графиков:', error);
    }
}

// Обновление графиков
function renderCharts(data) {
    // Здесь будет код инициализации Highcharts с полученными данными
    // Используйте данные из data.months, data.income, data.expenses и т.д.
    console.log('Данные для графиков:', data);
}

// ==================== Вспомогательные функции ====================

// Форматирование валюты
function formatCurrency(value) {
    return new Intl.NumberFormat('ru-RU', { 
        style: 'decimal',
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
    }).format(value);
}

// Показ уведомлений
function showAlert(message, type = 'info') {
    const alert = document.createElement('div');
    alert.className = `alert alert-${type}`;
    alert.textContent = message;
    
    document.body.appendChild(alert);
    
    setTimeout(() => {
        alert.remove();
    }, 3000);
}

// Инициализация графиков (пример для Highcharts)
function initCharts() {
    // Годовой график
    Highcharts.chart('chart-year', {
        title: { text: '' },
        series: [{
            name: 'Доходы',
            data: [] // Заполнится при загрузке
        }, {
            name: 'Расходы',
            data: [] // Заполнится при загрузке
        }]
    });

    // Недельный график активности
    Highcharts.chart('activity-chart', {
        chart: { type: 'column' },
        title: { text: '' },
        series: [{
            name: 'Доходы',
            data: []
        }, {
            name: 'Расходы',
            data: []
        }]
    });
}