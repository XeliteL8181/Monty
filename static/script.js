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
    text: '#E5EDFF'
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
        chart: {
            type: 'spline',
            backgroundColor: '#2c2c34',
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
        series: [{ name: 'Доходы', data: [] }, { name: 'Расходы', data: [] }],
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

    Highcharts.chart('activity-chart', {
        chart: {
            type: 'column',
            backgroundColor: '#2c2c34',
            spacing: [20, 20, 20, 20],
            className: 'column-chart',
            animation: false
        },
        title: { 
            text: '',
            style: {
                color: chartsConfig.colors.text,
                fontSize: '16px'
            }
        },
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
        series: [{ name: 'Доходы', data: [] }, { name: 'Расходы', data: [] }],
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

// Отрисовка графиков
function renderCharts(data) {
    // График по месяцам
    const financialChart = Highcharts.chart('chart-year', {
        chart: {
            type: 'spline',
            backgroundColor: '#2c2c34',
            spacing: [20, 20, 20, 20],
            className: 'spline-chart',
            animation: false
        },
        title: { text: null },
        credits: { enabled: false },
        legend: { enabled: false },
        xAxis: {
            categories: data.months,
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
                data: data.income, 
                color: chartsConfig.colors.income 
            },
            { 
                name: 'Расходы', 
                data: data.expenses, 
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

    // График активности
    const activityChart = Highcharts.chart('activity-chart', {
        chart: {
            type: 'column',
            backgroundColor: '#2c2c34',
            spacing: [20, 20, 20, 20],
            className: 'column-chart',
            animation: false
        },
        title: { 
            text: '',
            style: {
                color: chartsConfig.colors.text,
                fontSize: '16px'
            }
        },
        credits: { enabled: false },
        legend: { enabled: false },
        xAxis: {
            categories: data.days,
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
                data: data.earning, 
                color: chartsConfig.colors.income,
                pointPadding: 0.1, 
                pointPlacement: -0.2 
            },
            { 
                name: 'Расходы', 
                data: data.spent, 
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

    // Создание легенд для обоих графиков
    financialChart.series.forEach(series => {
        const legendId = series.name === 'Доходы' ? 'income-legend' : 'expenses-legend';
        const item = createLegendItem(series, legendId);
        
        if (item) {
            item.addEventListener('mouseleave', () => resetChart(financialChart));
        }
    });

    activityChart.series.forEach(series => {
        const legendId = series.name === 'Доходы' ? 'activity-income-legend' : 'activity-expenses-legend';
        const item = createLegendItem(series, legendId);
        
        if (item) {
            item.addEventListener('mouseleave', () => resetChart(activityChart));
        }
    });

    // Обработчики для сброса графиков
    [financialChart, activityChart].forEach(chart => {
        chart.renderTo.addEventListener('mouseleave', () => resetChart(chart));
    });
}

// Общие функции для работы с графиками
function updateTooltipColor(color) {
    document.documentElement.style.setProperty('--tooltip-color', color);
}

function resetChart(chart) {
    chart.series.forEach(s => {
        s.setState('');
        s.update({ opacity: 1 }, false);
    });
    chart.redraw();
}

function createLegendItem(series, containerId) {
    const container = document.getElementById(containerId);
    if (!container) return;

    const item = document.createElement('div');
    item.className = 'legend-item';
    item.style.cursor = 'pointer';

    item.innerHTML = `
        <div class="legend-color" style="background:${series.color}"></div>
        <span>${series.name}</span>
    `;

    item.addEventListener('click', function() {
        if (series.visible) {
            series.hide();
            this.classList.add('disabled');
        } else {
            series.show();
            this.classList.remove('disabled');
        }
    });

    container.appendChild(item);
    return item;
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