# Конфигурация сервиса накопительной системы лояльности

## Переменные окружения

Сервис поддерживает следующие переменные окружения:

| Переменная | Описание | Значение по умолчанию |
|------------|----------|----------------------|
| `RUN_ADDRESS` | Адрес и порт запуска сервиса | `localhost:8080` |
| `DATABASE_URI` | Адрес подключения к базе данных | `host=localhost user=postgres password=fmx274TQVw111w111w dbname=users sslmode=disable` |
| `ACCRUAL_SYSTEM_ADDRESS` | Адрес системы расчёта начислений | `http://localhost:8081` |
| `LOG_LEVEL` | Уровень логирования | `info` |
| `FILE_STORAGE_PATH` | Путь до файла с данными | `./storage.txt` |
| `BASE_URL` | Базовый URL для сокращённых ссылок | `http://localhost:8080` |

## Флаги командной строки

Сервис также поддерживает флаги командной строки (имеют приоритет над переменными окружения):

| Флаг | Описание | Соответствующая переменная окружения |
|------|----------|--------------------------------------|
| `-a` | HTTP server address | `RUN_ADDRESS` |
| `-d` | Database DSN for connection | `DATABASE_URI` |
| `-r` | Accrual system address | `ACCRUAL_SYSTEM_ADDRESS` |
| `-l` | Log level | `LOG_LEVEL` |
| `-f` | File storage path | `FILE_STORAGE_PATH` |
| `-b` | Base URL for short links | `BASE_URL` |

## Примеры использования

### Запуск с переменными окружения

```bash
export RUN_ADDRESS=":8080"
export DATABASE_URI="host=localhost user=myuser password=mypass dbname=mydb sslmode=disable"
export ACCRUAL_SYSTEM_ADDRESS="http://loyalty-service:8081"
export LOG_LEVEL="debug"

./gophermart
```

### Запуск с флагами командной строки

```bash
./gophermart -a :8080 -d "host=localhost user=myuser password=mypass dbname=mydb sslmode=disable" -r "http://loyalty-service:8081" -l debug
```

### Комбинированный запуск

```bash
export RUN_ADDRESS=":8080"
export DATABASE_URI="host=localhost user=myuser password=mypass dbname=mydb sslmode=disable"

./gophermart -r "http://loyalty-service:8081" -l debug
```

В этом случае:
- `RUN_ADDRESS` и `DATABASE_URI` берутся из переменных окружения
- `ACCRUAL_SYSTEM_ADDRESS` и `LOG_LEVEL` берутся из флагов командной строки

## Приоритет конфигурации

1. **Флаги командной строки** (высший приоритет)
2. **Переменные окружения**
3. **Значения по умолчанию** (низший приоритет)

## Docker Compose пример

```yaml
version: '3.8'
services:
  gophermart:
    build: .
    ports:
      - "8080:8080"
    environment:
      - RUN_ADDRESS=:8080
      - DATABASE_URI=host=postgres user=postgres password=password dbname=gophermart sslmode=disable
      - ACCRUAL_SYSTEM_ADDRESS=http://loyalty-service:8081
      - LOG_LEVEL=info
    depends_on:
      - postgres
      - loyalty-service

  postgres:
    image: postgres:13
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=password
      - POSTGRES_DB=gophermart
    ports:
      - "5432:5432"

  loyalty-service:
    image: loyalty-service:latest
    ports:
      - "8081:8081"
```

## Проверка конфигурации

При запуске сервис выводит в лог информацию о текущей конфигурации:

```
INFO    Running server address=:8080
INFO    Connected to PSQL server
INFO    Accrual system address=http://loyalty-service:8081
INFO    Log level=info
```
