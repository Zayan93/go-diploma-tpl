# go-musthave-diploma-tpl

Шаблон репозитория для индивидуального дипломного проекта курса «Go-разработчик»

## Описание

Сервис накопительной системы лояльности для обработки заказов пользователей и начисления баллов лояльности.

## Функциональность

- Регистрация и аутентификация пользователей
- Загрузка и обработка заказов
- Система начисления баллов лояльности
- Получение баланса пользователя
- Заглушка внешней системы расчёта баллов лояльности

## API Endpoints

### Аутентификация
- `POST /api/user/register` - Регистрация пользователя
- `POST /api/user/login` - Вход в систему

### Заказы
- `POST /api/user/orders` - Загрузка номера заказа
- `GET /api/user/orders` - Получение списка заказов пользователя

### Баланс
- `GET /api/user/balance` - Получение текущего баланса пользователя
- `POST /api/user/balance/withdraw` - Запрос на списание средств
- `GET /api/user/withdrawals` - Получение информации о выводах средств

### Система лояльности (заглушка)
- `GET /api/orders/{number}` - Получение информации о расчёте начислений

## Конфигурация

Сервис поддерживает конфигурирование через переменные окружения и флаги командной строки:

### Переменные окружения
- `RUN_ADDRESS` - Адрес и порт запуска сервиса
- `DATABASE_URI` - Адрес подключения к базе данных
- `ACCRUAL_SYSTEM_ADDRESS` - Адрес системы расчёта начислений
- `LOG_LEVEL` - Уровень логирования

### Флаги командной строки
- `-a` - HTTP server address
- `-d` - Database DSN for connection
- `-r` - Accrual system address
- `-l` - Log level

Подробная документация по конфигурации: [CONFIGURATION.md](CONFIGURATION.md)

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без
   префикса `https://`) для создания модуля
3. Настройте переменные окружения (см. `env.example`)
4. Запустите сервис: `go run cmd/gophermart/main.go`

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m master template https://github.com/yandex-praktikum/go-musthave-diploma-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/master .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Примеры использования

### 1. Регистрация пользователя

```bash
curl.exe -v -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/user/register \
  -d "{\"login\": \"testuser@example.com\", \"password\": \"password123\"}" \
  -c cookies.txt
```

### 2. Логин пользователя

```bash
curl.exe -v -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/user/login \
  -d "{\"login\": \"testuser@example.com\", \"password\": \"password123\"}" \
  -c cookies.txt
```

### 3. Загрузка заказа

```bash
curl.exe -v -H "Content-Type: text/plain" \
  -X POST http://localhost:8080/api/user/orders \
  -b cookies.txt \
  -d "12345678903"
```

### 4. Получение списка заказов

```bash
curl.exe -v -H "Content-Type: application/json" \
  -X GET http://localhost:8080/api/user/orders \
  -b cookies.txt
```

### 5. Получение баланса пользователя

```bash
curl.exe -v -H "Content-Type: application/json" \
  -X GET http://localhost:8080/api/user/balance \
  -b cookies.txt
```

### 6. Проверка системы лояльности (заглушка)

```bash
curl.exe -v -H "Content-Type: application/json" \
  -X GET http://localhost:8080/api/orders/12345678903
```

### 7. Вывод средств

```bash
curl.exe -v -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/user/balance/withdraw \
  -b cookies.txt \
  -d "{\"order\": \"987654321\", \"sum\": 100}"
```

### 8. Получение информации о выводах средств

```bash
curl.exe -v -H "Content-Type: application/json" \
  -X GET http://localhost:8080/api/user/withdrawals \
  -b cookies.txt
```

## Документация

- [API Examples](API_EXAMPLES.md) - Примеры использования API
- [Configuration](CONFIGURATION.md) - Подробная документация по конфигурации
