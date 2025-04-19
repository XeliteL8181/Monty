/**
 * Файл: static/script.js
 * Основной скрипт фронтенда для финансового приложения с улучшенным визуалом
 */

// Базовый путь API (относительно корня сайта)
const API_BASE_URL = '';

// Максимально допустимое значение
const MAX_VALUE = 99999999;

// Конфигурация графиков
const chartsConfig = {
  colors: {
    income: '#78be20',
    expenses: '#8c00ff',
    grid: '#E5EDFF',
    text: '#E5EDFF',
    background: '#2c2c34'
  }
};

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
    if (isNaN(value)) {
        showAlert('Введите корректное число', 'warning');
        return;
    }
    if (value < 0 || value > MAX_VALUE) {
        showAlert(`Максимум ${formatCurrency(MAX_VALUE)}`, 'warning');
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
        showAlert('Ошибка обновления накоплений', 'error');
    }
}

// Добавить доход
async function addIncome() {
    const value = parseFloat(elements.income.input.value);
    if (isNaN(value)) {
        showAlert('Введите корректное число', 'warning');
        return;
    }
    if (value <= 0 || value > MAX_VALUE) {
        showAlert(`Максимум ${formatCurrency(MAX_VALUE)}`, 'warning');
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
        showAlert('Доходы обновлены', 'success');
    } catch (err) {
        showAlert('Ошибка обновления доходов', 'error');
    }
}

// Добавить расход
async function addExpense() {
    const value = parseFloat(elements.expenses.input.value);
    if (isNaN(value)) {
        showAlert('Введите корректное число', 'warning');
        return;
    }
    if (value <= 0 || value > MAX_VALUE) {
        showAlert(`Максимум ${formatCurrency(MAX_VALUE)}`, 'warning');
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
        showAlert('Расходы обновлены', 'success');
    } catch (err) {
        showAlert('Ошибка обновления расходов', 'error');
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
    // График по месяцам (годовой)
    Highcharts.chart('chart-year', {
        chart: {
            type: 'spline',
            backgroundColor: chartsConfig.colors.background,
            spacing: [20, 20, 20, 20],
            className: 'spline-chart',
            animation: false
        },
        title: { text: null },
        credits: { enabled: false },
        legend: { enabled: false },
        xAxis: {
            lineColor: chartsConfig.colors.grid,
            tickLength: 0,
            labels: {
                style: { 
                    color: chartsConfig.colors.text,
                    fontSize: '16px'
                }
            }
        },
        yAxis: {
            title: { text: null },
            gridLineColor: chartsConfig.colors.grid,
            labels: {
                formatter: function() { 
                    return this.value.toLocaleString() + ' ₽'; 
                },
                style: { 
                    color: chartsConfig.colors.text,
                    fontSize: '16px'
                }
            }
        },
        plotOptions: {
            series: {
                marker: { enabled: false },
                lineWidth: 5,
                states: {
                    hover: {
                        lineWidth: 6,
                        halo: false
                    },
                    inactive: {
                        opacity: 0.2
                    }
                },
                events: {
                    mouseOver() {
                        this.chart.series.forEach(s => {
                            s.update({ opacity: s === this ? 1 : 0.2 }, false);
                        });
                    }
                }
            }
        },
        series: [
            { 
                name: 'Доходы', 
                data: [], 
                color: chartsConfig.colors.income 
            },
            { 
                name: 'Расходы', 
                data: [], 
                color: chartsConfig.colors.expenses 
            }
        ],
        tooltip: {
            shared: false,
            useHTML: true,
            backgroundColor: 'transparent',
            borderWidth: 0,
            style: { padding: '0' },
            formatter: function() {
                updateTooltipColor(this.series.color);
                return `
                <div style="
                    color: ${chartsConfig.colors.text};
                    padding: 8px 12px;
                    background: ${this.series.color};
                    border-radius: 4px;
                    font-family: inherit;
                    font-size: 16px;
                ">
                    ${this.series.name}: <b>${this.y.toLocaleString()} ₽</b>
                </div>
                `;
            }
        }
    });

    // График активности (недельный)
    Highcharts.chart('activity-chart', {
        chart: {
            type: 'column',
            backgroundColor: chartsConfig.colors.background,
            spacing: [20, 20, 20, 20],
            className: 'column-chart',
            animation: false
        },
        title: { text: null },
        credits: { enabled: false },
        legend: { enabled: false },
        xAxis: {
            categories: ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'],
            lineColor: chartsConfig.colors.grid,
            tickLength: 0,
            labels: {
                style: { 
                    color: chartsConfig.colors.text,
                    fontSize: '16px'
                }
            }
        },
        yAxis: {
            title: { text: null },
            gridLineColor: chartsConfig.colors.grid,
            labels: {
                formatter: function() { 
                    return this.value.toLocaleString() + ' ₽'; 
                },
                style: { 
                    color: chartsConfig.colors.text,
                    fontSize: '16px'
                }
            }
        },
        plotOptions: {
            column: {
                borderRadius: 4,
                pointWidth: 16,
                grouping: false
            },
            series: {
                states: {
                    hover: {
                        brightness: 0.1
                    },
                    inactive: {
                        opacity: 0.2
                    }
                },
                events: {
                    mouseOver() {
                        this.chart.series.forEach(s => {
                            s.update({ opacity: s === this ? 1 : 0.2 }, false);
                        });
                    }
                }
            }
        },
        series: [
            { 
                name: 'Доходы', 
                data: [], 
                color: chartsConfig.colors.income,
                pointPadding: 0.1,
                pointPlacement: -0.2
            },
            { 
                name: 'Расходы', 
                data: [], 
                color: chartsConfig.colors.expenses,
                pointPadding: 0.1,
                pointPlacement: 0.2
            }
        ],
        tooltip: {
            shared: false,
            useHTML: true,
            backgroundColor: 'transparent',
            borderWidth: 0,
            style: { padding: '0' },
            formatter: function() {
                updateTooltipColor(this.series.color);
                return `
                <div style="
                    color: ${chartsConfig.colors.text};
                    padding: 8px 12px;
                    background: ${this.series.color};
                    border-radius: 4px;
                    font-family: inherit;
                    font-size: 16px;
                ">
                    ${this.series.name}: <b>${this.y.toLocaleString()} ₽</b>
                </div>
                `;
            }
        }
    });
}

// Отрисовка графиков с данными
function renderCharts(data) {
    // Обновляем годовой график
    const financialChart = Highcharts.chart('chart-year', {
        chart: {
            type: 'spline',
            backgroundColor: chartsConfig.colors.background,
            spacing: [20, 20, 20, 20],
            className: 'spline-chart',
            animation: false
        },
        title: { text: null },
        credits: { enabled: false },
        legend: { enabled: false },
        xAxis: {
            categories: data.months || ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек'],
            lineColor: chartsConfig.colors.grid,
            tickLength: 0,
            labels: {
                style: { 
                    color: chartsConfig.colors.text,
                    fontSize: '16px'
                }
            }
        },
        yAxis: {
            title: { text: null },
            gridLineColor: chartsConfig.colors.grid,
            labels: {
                formatter: function() { 
                    return this.value.toLocaleString() + ' ₽'; 
                },
                style: { 
                    color: chartsConfig.colors.text,
                    fontSize: '16px'
                }
            }
        },
        plotOptions: {
            series: {
                marker: { enabled: false },
                lineWidth: 5,
                states: {
                    hover: {
                        lineWidth: 6,
                        halo: false
                    },
                    inactive: {
                        opacity: 0.2
                    }
                },
                events: {
                    mouseOver() {
                        this.chart.series.forEach(s => {
                            s.update({ opacity: s === this ? 1 : 0.2 }, false);
                        });
                    }
                }
            }
        },
        series: [
            { 
                name: 'Доходы', 
                data: data.income || Array(12).fill(0),
                color: chartsConfig.colors.income 
            },
            { 
                name: 'Расходы', 
                data: data.expenses || Array(12).fill(0),
                color: chartsConfig.colors.expenses 
            }
        ],
        tooltip: {
            shared: false,
            useHTML: true,
            backgroundColor: 'transparent',
            borderWidth: 0,
            style: { padding: '0' },
            formatter: function() {
                updateTooltipColor(this.series.color);
                return `
                <div style="
                    color: ${chartsConfig.colors.text};
                    padding: 8px 12px;
                    background: ${this.series.color};
                    border-radius: 4px;
                    font-family: inherit;
                    font-size: 16px;
                ">
                    ${this.series.name}: <b>${this.y.toLocaleString()} ₽</b>
                </div>
                `;
            }
        }
    });

    // Обновляем недельный график активности
    const activityChart = Highcharts.chart('activity-chart', {
        chart: {
            type: 'column',
            backgroundColor: chartsConfig.colors.background,
            spacing: [20, 20, 20, 20],
            className: 'column-chart',
            animation: false
        },
        title: { text: null },
        credits: { enabled: false },
        legend: { enabled: false },
        xAxis: {
            categories: data.days || ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'],
            lineColor: chartsConfig.colors.grid,
            tickLength: 0,
            labels: {
                style: { 
                    color: chartsConfig.colors.text,
                    fontSize: '16px'
                }
            }
        },
        yAxis: {
            title: { text: null },
            gridLineColor: chartsConfig.colors.grid,
            labels: {
                formatter: function() { 
                    return this.value.toLocaleString() + ' ₽'; 
                },
                style: { 
                    color: chartsConfig.colors.text,
                    fontSize: '16px'
                }
            }
        },
        plotOptions: {
            column: {
                borderRadius: 4,
                pointWidth: 16,
                grouping: false
            },
            series: {
                states: {
                    hover: {
                        brightness: 0.1
                    },
                    inactive: {
                        opacity: 0.2
                    }
                },
                events: {
                    mouseOver() {
                        this.chart.series.forEach(s => {
                            s.update({ opacity: s === this ? 1 : 0.2 }, false);
                        });
                    }
                }
            }
        },
        series: [
            { 
                name: 'Доходы', 
                data: data.earning || Array(7).fill(0),
                color: chartsConfig.colors.income,
                pointPadding: 0.1,
                pointPlacement: -0.2
            },
            { 
                name: 'Расходы', 
                data: data.spent || Array(7).fill(0),
                color: chartsConfig.colors.expenses,
                pointPadding: 0.1,
                pointPlacement: 0.2
            }
        ],
        tooltip: {
            shared: false,
            useHTML: true,
            backgroundColor: 'transparent',
            borderWidth: 0,
            style: { padding: '0' },
            formatter: function() {
                updateTooltipColor(this.series.color);
                return `
                <div style="
                    color: ${chartsConfig.colors.text};
                    padding: 8px 12px;
                    background: ${this.series.color};
                    border-radius: 4px;
                    font-family: inherit;
                    font-size: 16px;
                ">
                    ${this.series.name}: <b>${this.y.toLocaleString()} ₽</b>
                </div>
                `;
            }
        }
    });

    // Создаем легенды для графиков
    createChartLegends(financialChart, activityChart);
}

// Создание легенд для графиков
function createChartLegends(financialChart, activityChart) {
    // Удаляем старые легенды, если они есть
    ['income-legend', 'expenses-legend', 
     'activity-income-legend', 'activity-expenses-legend'].forEach(id => {
        const container = document.getElementById(id);
        if (container) container.innerHTML = '';
    });

    // Легенды для годового графика
    financialChart.series.forEach(series => {
        const legendId = series.name === 'Доходы' ? 'income-legend' : 'expenses-legend';
        createLegendItem(series, legendId);
    });

    // Легенды для недельного графика
    activityChart.series.forEach(series => {
        const legendId = series.name === 'Доходы' ? 'activity-income-legend' : 'activity-expenses-legend';
        createLegendItem(series, legendId);
    });

    // Обработчики для сброса графиков при уходе мыши
    [financialChart, activityChart].forEach(chart => {
        chart.renderTo.addEventListener('mouseleave', () => resetChart(chart));
    });
}

// Создание элемента легенды
function createLegendItem(series, containerId) {
    const container = document.getElementById(containerId);
    if (!container) return null;

    const item = document.createElement('div');
    item.className = 'legend-item';
    item.style.cursor = 'pointer';
    item.style.display = 'flex';
    item.style.alignItems = 'center';
    item.style.margin = '5px 0';

    item.innerHTML = `
        <div class="legend-color" style="
            width: 16px;
            height: 16px;
            background: ${series.color};
            margin-right: 8px;
            border-radius: 3px;
        "></div>
        <span style="color: ${chartsConfig.colors.text}">${series.name}</span>
    `;

    item.addEventListener('click', function() {
        if (series.visible) {
            series.hide();
            item.style.opacity = '0.5';
        } else {
            series.show();
            item.style.opacity = '1';
        }
    });

    item.addEventListener('mouseover', function() {
        series.chart.series.forEach(s => {
            s.update({ opacity: s === series ? 1 : 0.2 }, false);
        });
        series.chart.redraw();
    });

    container.appendChild(item);
    return item;
}

// Сброс графика при уходе мыши
function resetChart(chart) {
    chart.series.forEach(s => {
        s.setState('');
        s.update({ opacity: 1 }, false);
    });
    chart.redraw();
}

// Обновление цвета tooltip
function updateTooltipColor(color) {
    document.documentElement.style.setProperty('--tooltip-color', color);
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
    const alertContainer = document.getElementById('alert-container') || createAlertContainer();
    const alert = document.createElement('div');
    
    alert.className = `alert ${type}`;
    alert.innerHTML = `
        <div class="alert-icon">${getIconForType(type)}</div>
        <div class="alert-message">${message}</div>
        <div class="alert-close" onclick="this.parentElement.remove()">&times;</div>
    `;

    alertContainer.appendChild(alert);
    
    // Автоматическое скрытие через 5 секунд
    setTimeout(() => {
        alert.style.opacity = '0';
        setTimeout(() => alert.remove(), 300);
    }, 5000);
}

function createAlertContainer() {
    const container = document.createElement('div');
    container.id = 'alert-container';
    container.style.position = 'fixed';
    container.style.top = '20px';
    container.style.right = '20px';
    container.style.zIndex = '1000';
    container.style.display = 'flex';
    container.style.flexDirection = 'column';
    container.style.gap = '10px';
    document.body.appendChild(container);
    return container;
}

function getIconForType(type) {
    const icons = {
        success: '✓',
        error: '✕',
        warning: '⚠',
        info: 'i'
    };
    return icons[type] || icons.info;
}