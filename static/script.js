const API_BASE_URL = 'http://localhost:8080/api';

// Общая конфигурация для обоих графиков
const chartsConfig = {
  // Конфиг для первого графика (финансовый обзор)
  financialOverview: {
    months: ['Янв', 'Фев', 'Март', 'Апр', 'Май', 'Июнь', 
             'Июль', 'Авг', 'Сен', 'Окт', 'Нояб', 'Дек'],
    income: [55400, 55000, 60000, 100000, 150000, 80000, 
             70000, 55000, 75000, 100000, 65000, 130000],
    expenses: [40000, 35000, 120000, 10000, 70000, 190000, 
               20000, 25000, 85000, 20000, 35000, 175000],
    colors: {
      income: '#78be20',
      expenses: '#8c00ff',
      grid: '#E5EDFF',
      text: '#E5EDFF'
    }
  },
  
  // Конфиг для второго графика (активность)
  activity: {
    days: ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'],
    earning: [1800, 2200, 2500, 1900, 2300, 1700, 2100],
    spent: [2200, 900, 1400, 2100, 1300, 1800, 1000],
    colors: {
      earning: '#78be20',
      spent: '#8c00ff',
      grid: '#E5EDFF',
      text: '#E5EDFF'
    }
  },
};

// Функция для загрузки данных с сервера
async function loadFinancialData() {
  try {
    const response = await fetch(`${API_BASE_URL}/data`);
    if (!response.ok) throw new Error('Ошибка загрузки данных');
    financialData = await response.json();
    updateDisplay();
  } catch (error) {
    console.error('Ошибка:', error);
  }
}

// Функция для сохранения данных на сервер
async function saveFinancialData() {
  try {
    const response = await fetch(`${API_BASE_URL}/update`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(financialData)
    });
    if (!response.ok) throw new Error('Ошибка сохранения данных');
  } catch (error) {
    console.error('Ошибка:', error);
  }
}

// Функция для загрузки данных графиков с сервера
async function loadChartsData() {
  try {
    const response = await fetch(`${API_BASE_URL}/charts`);
    if (!response.ok) throw new Error('Ошибка загрузки данных графиков');
    const data = await response.json();
    
    // Обновляем конфигурацию графиков
    chartsConfig.financialOverview.months = data.months;
    chartsConfig.financialOverview.income = data.income;
    chartsConfig.financialOverview.expenses = data.expenses;
    chartsConfig.activity.days = data.days;
    chartsConfig.activity.earning = data.earning;
    chartsConfig.activity.spent = data.spent;
    
    // Пересоздаем графики с новыми данными
    initCharts();
  } catch (error) {
    console.error('Ошибка:', error);
  }
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

// Инициализация графиков после загрузки DOM
document.addEventListener('DOMContentLoaded', () => {
  // 1. График финансового обзора
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
      categories: chartsConfig.financialOverview.months,
      lineColor: chartsConfig.financialOverview.colors.grid,
      tickLength: 0,
      labels: {
        style: { 
          color: chartsConfig.financialOverview.colors.text,
          fontSize: '16px'
        }
      }
    },
    yAxis: {
      title: { text: null },
      gridLineColor: chartsConfig.financialOverview.colors.grid,
      labels: {
        formatter: function() { 
          return this.value.toLocaleString() + ' ₽'; 
        },
        style: { 
          color: chartsConfig.financialOverview.colors.text,
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
    series: [{
      name: 'Доходы',
      data: chartsConfig.financialOverview.income,
      color: chartsConfig.financialOverview.colors.income
    }, {
      name: 'Расходы',
      data: chartsConfig.financialOverview.expenses,
      color: chartsConfig.financialOverview.colors.expenses
    }],
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
          color: ${chartsConfig.financialOverview.colors.text};
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

  // 2. График активности
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
        color: chartsConfig.activity.colors.text,
        fontSize: '16px'
      }
    },
    credits: { enabled: false },
    legend: { enabled: false },
    xAxis: {
      categories: chartsConfig.activity.days,
      lineColor: chartsConfig.activity.colors.grid,
      tickLength: 0,
      labels: {
        style: { 
          color: chartsConfig.activity.colors.text,
          fontSize: '16px'
        }
      }
    },
    yAxis: {
      title: { text: null },
      gridLineColor: chartsConfig.activity.colors.grid,
      labels: {
        formatter: function() { 
          return this.value.toLocaleString() + ' ₽'; 
        },
        style: { 
          color: chartsConfig.activity.colors.text,
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
    series: [{
      name: 'Доходы',
      data: chartsConfig.activity.earning,
      color: chartsConfig.activity.colors.earning
    }, {
      name: 'Расходы',
      data: chartsConfig.activity.spent,
      color: chartsConfig.activity.colors.spent
    }],
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
          color: ${chartsConfig.activity.colors.text};
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
});

// Код для работы с карточками
let financialData = {
  savings: 0,
  income: 0,
  expenses: 0
};

function updateDisplay() {
  document.getElementById("savings-button").textContent = `${financialData.savings.toLocaleString()}₽`;
  document.getElementById("income-button").textContent = `${financialData.income.toLocaleString()}₽`;
  document.getElementById("expenses-button").textContent = `${financialData.expenses.toLocaleString()}₽`;
  document.getElementById("balance-button").textContent = `${(financialData.income - financialData.expenses).toLocaleString()}₽`;
}

async function updateSavings() {
  const input = document.getElementById("savings-input");
  const value = parseFloat(input.value) || 0;
  financialData.savings = value;
  input.value = "";
  await saveFinancialData();
  updateDisplay();
}

async function addIncome() {
  const input = document.getElementById("income-input");
  const value = parseFloat(input.value) || 0;
  financialData.income += value;
  input.value = "";
  await saveFinancialData();
  updateDisplay();
}

async function addExpense() {
  const input = document.getElementById("expenses-input");
  const value = parseFloat(input.value) || 0;
  financialData.expenses += value;
  input.value = "";
  await saveFinancialData();
  updateDisplay();
}

// Инициализация при загрузке
updateDisplay();

document.addEventListener('DOMContentLoaded', () => {
  loadFinancialData();
  loadChartsData();
});