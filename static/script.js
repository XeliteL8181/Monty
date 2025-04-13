// Конфигурация API
const API_BASE_URL = window.location.origin;

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
document.addEventListener('DOMContentLoaded', initApp);

function initApp() {
    loadCardsData();
    loadChartsData();
    setupEventListeners();
}

// ==================== Слушатели ====================

function setupEventListeners() {
    elements.savings.button.addEventListener('click', () => handleCardUpdate('savings', false, elements.savings.input));
    elements.income.button.addEventListener('click', () => handleCardUpdate('income', true, elements.income.input));
    elements.expenses.button.addEventListener('click', () => handleCardUpdate('expenses', true, elements.expenses.input));
}

// ==================== Карточки ====================

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

function updateCardsUI(data) {
    elements.savings.display.textContent = formatCurrency(data.savings);
    elements.income.display.textContent = formatCurrency(data.income);
    elements.expenses.display.textContent = formatCurrency(data.expenses);
    elements.balance.display.textContent = formatCurrency(data.balance);
}

async function handleCardUpdate(type, isIncremental, inputElement) {
    const value = parseFloat(inputElement.value);
    if (isNaN(value) || value <= 0) {
        showAlert('Введите корректную сумму', 'warning');
        return;
    }

    try {
        await fetch(`${API_BASE_URL}/api/cards/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ type, value, isIncremental })
        });

        inputElement.value = '';
        await loadCardsData();
        showAlert(`${capitalize(type)} обновлены`, 'success');
    } catch (error) {
        console.error('Ошибка обновления:', error);
        showAlert('Ошибка обновления', 'error');
    }
}

// ==================== Графики ====================

async function loadChartsData() {
    try {
        const response = await fetch(`${API_BASE_URL}/api/charts`);
        if (!response.ok) throw new Error('Ошибка загрузки графиков');

        const data = await response.json();
        renderCharts(data);
    } catch (error) {
        console.error('Ошибка загрузки графиков:', error);
        showAlert('Ошибка загрузки графиков', 'error');
    }
}

function renderCharts(data) {
    Highcharts.chart('chart-year', {
        title: { text: '' },
        xAxis: { categories: data.months },
        series: [
            { name: 'Доходы', data: data.income },
            { name: 'Расходы', data: data.expenses }
        ]
    });

    Highcharts.chart('activity-chart', {
        chart: { type: 'column' },
        title: { text: '' },
        xAxis: { categories: data.days },
        series: [
            { name: 'Доходы', data: data.earning },
            { name: 'Расходы', data: data.spent }
        ]
    });
}

// ==================== Утилиты ====================

function formatCurrency(value) {
    return new Intl.NumberFormat('ru-RU', {
        style: 'decimal',
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
    }).format(value);
}

function showAlert(message, type = 'info') {
    const alert = document.createElement('div');
    alert.className = `alert alert-${type}`;
    alert.textContent = message;
    document.body.appendChild(alert);

    setTimeout(() => alert.remove(), 3000);
}

function capitalize(str) {
    return str.charAt(0).toUpperCase() + str.slice(1);
}