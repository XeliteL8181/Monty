/**
 * Файл: script.js
 * Основной скрипт фронтенд части приложения для учета финансов
 * Включает работу с карточками, графиками и взаимодействие с API
 */

// ==================== КОНФИГУРАЦИЯ ====================

// Базовый URL API (оставлен пустым для работы на том же домене)
const API_BASE_URL = '';

// Максимально допустимое значение для финансовых операций
const MAX_VALUE = 99999999;

// ==================== DOM ЭЛЕМЕНТЫ ====================

// Объект со всеми важными элементами интерфейса
const elements = {
    // Блок накоплений
    savings: {
        display: document.getElementById('savings-button'), // Элемент отображения суммы
        input: document.getElementById('savings-input'),    // Поле ввода
        button: document.getElementById('update-savings-btn') // Кнопка обновления
    },
    // Блок доходов
    income: {
        display: document.getElementById('income-button'),
        input: document.getElementById('income-input'),
        button: document.getElementById('add-income-btn')
    },
    // Блок расходов
    expenses: {
        display: document.getElementById('expenses-button'),
        input: document.getElementById('expenses-input'),
        button: document.getElementById('add-expense-btn')
    },
    // Блок баланса (только отображение)
    balance: {
        display: document.getElementById('balance-button')
    }
};

// ==================== ИНИЦИАЛИЗАЦИЯ ====================

// Запуск при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    loadCardsData();         // Загрузка данных карточек
    loadChartsData();        // Загрузка данных графиков
    setupEventListeners();   // Настройка обработчиков событий
    initCharts();            // Инициализация графиков
});

// Настройка обработчиков событий для кнопок
function setupEventListeners() {
    elements.savings.button.addEventListener('click', updateSavings);
    elements.income.button.addEventListener('click', addIncome);
    elements.expenses.button.addEventListener('click', addExpense);
}

// ==================== РАБОТА С КАРТОЧКАМИ ====================

/**
 * Загрузка данных карточек с сервера
 * @async
 */
async function loadCardsData() {
    try {
        // Отправка GET запроса к API
        const response = await fetch(`${API_BASE_URL}/api/cards`);
        if (!response.ok) throw new Error('Ошибка загрузки данных');
        
        // Парсинг JSON ответа
        const data = await response.json();
        
        // Обновление интерфейса
        updateCardsUI(data);
    } catch (error) {
        console.error('Ошибка:', error);
        showAlert('Ошибка загрузки данных', 'error');
    }
}

/**
 * Обновление интерфейса карточек
 * @param {Object} data - Данные для отображения
 */
function updateCardsUI(data) {
    // Форматирование и отображение значений с символом рубля
    elements.savings.display.textContent = `${formatCurrency(data.savings)}₽`;
    elements.income.display.textContent = `${formatCurrency(data.income)}₽`;
    elements.expenses.display.textContent = `${formatCurrency(data.expenses)}₽`;
    elements.balance.display.textContent = `${formatCurrency(data.balance)}₽`;
}

/**
 * Обновление накоплений
 * @async
 */
async function updateSavings() {
    // Получение и проверка значения из поля ввода
    const value = parseFloat(elements.savings.input.value);
    if (isNaN(value) || value < 0 || value > MAX_VALUE) {
        showAlert('Максимальное значение карточки 99999999', 'warning');
        return;
    }

    try {
        // Отправка POST запроса на сервер
        const response = await fetch(`${API_BASE_URL}/api/cards/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                type: 'savings', 
                value: value,
                isIncremental: false // Полная замена значения
            })
        });
        
        if (!response.ok) throw new Error('Ошибка сервера');
        
        // Очистка поля ввода и обновление данных
        elements.savings.input.value = '';
        await loadCardsData();
        showAlert('Накопления обновлены', 'success');
    } catch (error) {
        console.error('Ошибка:', error);
        showAlert('Ошибка обновления', 'error');
    }
}

/**
 * Добавление дохода
 * @async
 */
async function addIncome() {
    const value = parseFloat(elements.income.input.value);
    if (isNaN(value) || value <= 0 || value > MAX_VALUE) {
        showAlert('Максимальное значение карточки 99999999', 'warning');
        return;
    }

    try {
        const response = await fetch(`${API_BASE_URL}/api/cards/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                type: 'income', 
                value: value,
                isIncremental: true // Добавление к текущему значению
            })
        });
        
        if (!response.ok) throw new Error('Ошибка сервера');
        
        elements.income.input.value = '';
        // Обновление и карточек и графиков
        await Promise.all([loadCardsData(), loadChartsData()]);
        showAlert('Доходы обновлены', 'success');
    } catch (error) {
        console.error('Ошибка:', error);
        showAlert('Ошибка обновления доходов', 'error');
    }
}

/**
 * Добавление расхода
 * @async
 */
async function addExpense() {
    const value = parseFloat(elements.expenses.input.value);
    if (isNaN(value) || value <= 0 || value > MAX_VALUE) {
        showAlert('Максимальное значение карточки 99999999', 'warning');
        return;
    }

    try {
        const response = await fetch(`${API_BASE_URL}/api/cards/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                type: 'expenses', 
                value: value,
                isIncremental: true 
            })
        });
        
        if (!response.ok) throw new Error('Ошибка сервера');
        
        elements.expenses.input.value = '';
        await Promise.all([loadCardsData(), loadChartsData()]);
        showAlert('Расходы обновлены', 'success');
    } catch (error) {
        console.error('Ошибка:', error);
        showAlert('Ошибка обновления расходов', 'error');
    }
}

// ==================== РАБОТА С ГРАФИКАМИ ====================

/**
 * Загрузка данных графиков с сервера
 * @async
 */
async function loadChartsData() {
    try {
        const response = await fetch(`${API_BASE_URL}/api/charts`);
        if (!response.ok) throw new Error('Ошибка загрузки графиков');
        
        const data = await response.json();
        renderCharts(data); // Отрисовка графиков
    } catch (error) {
        console.error('Ошибка загрузки графиков:', error);
    }
}

/**
 * Отрисовка графиков с использованием Highcharts
 * @param {Object} data - Данные для графиков
 */
function renderCharts(data) {
    // Годовой график (линейный)
    Highcharts.chart('chart-year', {
        title: { text: '' }, // Без заголовка
        xAxis: {
            categories: data.months // Месяцы по оси X
        },
        yAxis: {
            title: { text: 'Сумма (₽)' } // Подпись оси Y
        },
        series: [{
            name: 'Доходы',
            data: data.income,  // Данные доходов
            color: '#28a745'    // Зеленый цвет
        }, {
            name: 'Расходы',
            data: data.expenses, // Данные расходов
            color: '#dc3545'    // Красный цвет
        }]
    });

    // Недельный график активности (столбчатая диаграмма)
    Highcharts.chart('activity-chart', {
        chart: { type: 'column' }, // Тип графика
        title: { text: '' },
        xAxis: {
            categories: data.days // Дни недели по оси X
        },
        yAxis: {
            title: { text: 'Сумма (₽)' }
        },
        plotOptions: {
            column: {
                grouping: false,    // Раздельные столбцы
                shadow: false,      // Без тени
                borderWidth: 0      // Без границ
            }
        },
        series: [{
            name: 'Доходы',
            data: data.earning,     // Данные доходов
            color: '#28a745',
            pointPadding: 0.1,     // Отступы между столбцами
            pointPlacement: -0.2    // Смещение столбцов
        }, {
            name: 'Расходы',
            data: data.spent,       // Данные расходов
            color: '#dc3545',
            pointPadding: 0.1,
            pointPlacement: 0.2
        }]
    });
}

/**
 * Инициализация пустых графиков при загрузке страницы
 */
function initCharts() {
    // Пустой годовой график
    Highcharts.chart('chart-year', {
        title: { text: '' },
        series: [{
            name: 'Доходы',
            data: []
        }, {
            name: 'Расходы',
            data: []
        }]
    });

    // Пустой недельный график
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

// ==================== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ====================

/**
 * Форматирование числа в денежный формат
 * @param {number} value - Число для форматирования
 * @returns {string} Отформатированная строка
 */
function formatCurrency(value) {
    return new Intl.NumberFormat('ru-RU', { 
        style: 'decimal',
        minimumFractionDigits: 2, // Всегда 2 знака после запятой
        maximumFractionDigits: 2
    }).format(value);
}

/**
 * Показ всплывающего уведомления
 * @param {string} message - Текст сообщения
 * @param {string} type - Тип сообщения (info, success, warning, error)
 */
function showAlert(message, type = 'info') {
    const alert = document.createElement('div');
    alert.className = `alert alert-${type}`; // Классы для стилизации
    alert.textContent = message;
    
    // Добавление на страницу
    document.body.appendChild(alert);
    
    // Автоматическое удаление через 3 секунды
    setTimeout(() => {
        alert.remove();
    }, 3000);
}