#!/bin/bash

# Пользователь и база
DB_USER="finance_user"
DB_PASS="your_password"
DB_NAME="finance_db"

# Функция: создать пользователя
create_user() {
  echo "Создание пользователя '$DB_USER'..."
  psql -U postgres -tc "SELECT 1 FROM pg_roles WHERE rolname='$DB_USER'" | grep -q 1 || \
    psql -U postgres -c "CREATE USER $DB_USER WITH PASSWORD '$DB_PASS';"
}

# Функция: создать базу
create_database() {
  echo "Создание базы данных '$DB_NAME'..."
  psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME'" | grep -q 1 || \
    createdb -U postgres -O $DB_USER $DB_NAME
}

# Функция: выдать права
grant_privileges() {
  echo "Назначение прав..."
  psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO $DB_USER;"
}

# Запуск всех шагов
create_user
create_database
grant_privileges

echo "Пользователь и база созданы."