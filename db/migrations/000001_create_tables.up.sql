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
);