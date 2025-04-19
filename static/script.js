/**
 * Файл: static/script.js
 * Основной скрипт фронтенда для финансового приложения
 */

// Базовый путь API (относительно корня сайта)
const API_BASE_URL = '';

// Максимально допустимое значение
const MAX_VALUE = 99999999;

// DOM-элементы
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

// При загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    loadCardsData();
    loadChartsData();
    setupEventListeners();
    initCharts();
});

// Обработчики событий
function setupEventListeners() {
    elements.savings.button.addEventListener('click', updateSavings);
    elements.income.button.addEventListener('click', addIncome);
    elements.expenses.button.addEventListener('click', addExpense);
}

// Загрузка карточек
async function loadCardsData() {
    try {
        const res = await fetch(`${API_BASE_URL}/api/cards`);
        if (!res.ok) throw new Error('Ошибка загрузки данных');
        const data = await res.json();
        updateCardsUI(data);
    } catch (err) {
        console.error(err);
        showAlert('Ошибка загрузки данных', 'error');
    }
}

function updateCardsUI(data) {
    elements.savings.display.textContent = `${formatCurrency(data.savings)}₽`;
    elements.income.display.textContent = `${formatCurrency(data.income)}₽`;
    elements.expenses.display.textContent = `${formatCurrency(data.expenses)}₽`;
    elements.balance.display.textContent = `${formatCurrency(data.balance)}₽`;
}

// Обновить накопления
async function updateSavings() {
    const value = parseFloat(elements.savings.input.value);
    if (isNaN(value) || value < 0 || value > MAX_VALUE) {
        showAlert('Максимум 99 999 999', 'warning');
        return;
    }

    try {
        const res = await fetch(`${API_BASE_URL}/api/cards/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                type: 'savings',
                value: value,
                isIncremental: false
            })
        });
        if (!res.ok) throw new Error('Ошибка запроса');
        elements.savings.input.value = '';
        await loadCardsData();
        showAlert('Накопления обновлены', 'success');
    } catch (err) {
        showAlert('Ошибка обновления', 'error');
    }
}

// Добавить доход
async function addIncome() {
    const value = parseFloat(elements.income.input.value);
    if (isNaN(value) || value <= 0 || value > MAX_VALUE) {
        showAlert('Максимум 99 999 999', 'warning');
        return;
    }

    try {
        const res = await fetch(`${API_BASE_URL}/api/cards/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                type: 'income',
                value: value,
                isIncremental: true
            })
        });
        if (!res.ok) throw new Error('Ошибка запроса');
        elements.income.input.value = '';
        await Promise.all([loadCardsData(), loadChartsData()]);
        showAlert('Доход добавлен', 'success');
    } catch (err) {
        showAlert('Ошибка добавления дохода', 'error');
    }
}

// Добавить расход
async function addExpense() {
    const value = parseFloat(elements.expenses.input.value);
    if (isNaN(value) || value <= 0 || value > MAX_VALUE) {
        showAlert('Максимум 99 999 999', 'warning');
        return;
    }

    try {
        const res = await fetch(`${API_BASE_URL}/api/cards/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                type: 'expenses',
                value: value,
                isIncremental: true
            })
        });
        if (!res.ok) throw new Error('Ошибка запроса');
        elements.expenses.input.value = '';
        await Promise.all([loadCardsData(), loadChartsData()]);
        showAlert('Расход добавлен', 'success');
    } catch (err) {
        showAlert('Ошибка добавления расхода', 'error');
    }
}

// Загрузка графиков
async function loadChartsData() {
    try {
        const res = await fetch(`${API_BASE_URL}/api/charts`);
        if (!res.ok) throw new Error('Ошибка загрузки графиков');
        const data = await res.json();
        renderCharts(data);
    } catch (err) {
        console.error('Ошибка:', err);
    }
}

// Пустые графики при старте
function initCharts() {
    Highcharts.chart('chart-year', {
        title: { text: '' },
        series: [{ name: 'Доходы', data: [] }, { name: 'Расходы', data: [] }]
    });

    Highcharts.chart('activity-chart', {
        chart: { type: 'column' },
        title: { text: '' },
        series: [{ name: 'Доходы', data: [] }, { name: 'Расходы', data: [] }]
    });
}

// Отрисовка графиков
function renderCharts(data) {
    Highcharts.chart('chart-year', {
        title: { text: '' },
        xAxis: { categories: data.months },
        yAxis: { title: { text: '₽' } },
        series: [
            { name: 'Доходы', data: data.income, color: '#28a745' },
            { name: 'Расходы', data: data.expenses, color: '#dc3545' }
        ]
    });

    Highcharts.chart('activity-chart', {
        chart: { type: 'column' },
        xAxis: { categories: data.days },
        yAxis: { title: { text: '₽' } },
        plotOptions: {
            column: { grouping: false, shadow: false, borderWidth: 0 }
        },
        series: [
            { name: 'Доходы', data: data.earning, color: '#28a745', pointPadding: 0.1, pointPlacement: -0.2 },
            { name: 'Расходы', data: data.spent, color: '#dc3545', pointPadding: 0.1, pointPlacement: 0.2 }
        ]
    });
}

// Формат валюты
function formatCurrency(value) {
    return new Intl.NumberFormat('ru-RU', {
        style: 'decimal',
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
    }).format(value);
}

// Уведомления
function showAlert(message, type = 'info') {
    const div = document.createElement('div');
    div.className = `alert alert-${type}`;
    div.textContent = message;
    document.body.appendChild(div);
    setTimeout(() => div.remove(), 3000);
}
