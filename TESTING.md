# Тестирование

Этот документ описывает, как запускать тесты для проекта.

## Структура тестов

### Тесты для handlers.go
- **Файл**: `internal/app/handler_test.go`
- **Описание**: Тесты для HTTP обработчиков
- **Тип**: Unit тесты с использованием стандартного пакета `testing`

### Тесты для database.go
- **Файл**: `internal/storage/database_test.go`
- **Описание**: Тесты для логики базы данных
- **Тип**: Unit тесты без подключения к реальной БД

## Запуск тестов

### Запуск всех тестов
```bash
go test ./...
```

### Запуск тестов для конкретного пакета
```bash
# Тесты для handlers
go test ./internal/app

# Тесты для database
go test ./internal/storage
```

### Запуск конкретного теста
```bash
# Тест валидации номеров заказов
go test ./internal/app -run TestIsValidOrderNumber

# Тест структуры таблиц
go test ./internal/storage -run TestTableStructure
```

### Запуск тестов с подробным выводом
```bash
go test -v ./...
```

### Запуск тестов с покрытием
```bash
go test -cover ./...
```

## Описание тестов

### Handlers тесты

#### TestIsValidOrderNumber
Проверяет корректность алгоритма Луна для валидации номеров заказов.

**Тестируемые случаи:**
- Валидные номера заказов
- Невалидные номера заказов
- Пустые строки
- Нечисловые символы

#### TestOrderResponseStructure
Проверяет корректность структуры ответа для заказов.

#### TestWithdrawalRequestStructure
Проверяет корректность структуры запроса на вывод средств.

#### TestJSONSerialization
Проверяет корректность JSON сериализации/десериализации.

#### TestHTTPMethodValidation
Проверяет валидацию HTTP методов.

#### TestUserContext
Проверяет работу с контекстом пользователя.

#### TestLuhnAlgorithm
Проверяет алгоритм Луна на известных тестовых номерах.

### Database тесты

#### TestDatabaseStorageInitialization
Проверяет создание структуры DatabaseStorage.

#### TestContextOperations
Проверяет работу с контекстом.

#### TestDatabaseQueries
Проверяет корректность SQL запросов (без выполнения).

#### TestTableStructure
Проверяет структуру таблиц базы данных.

#### TestIndexes
Проверяет корректность индексов.

#### TestTransactionQueries
Проверяет транзакционные запросы.

#### TestErrorHandling
Проверяет обработку ошибок.

#### TestDataTypes
Проверяет типы данных в базе.

## Требования

- Go 1.19 или выше
- Стандартный пакет `testing`

## Примечания

1. **Тесты database.go** не требуют подключения к реальной базе данных
2. **Тесты handlers.go** используют mock объекты
3. Все тесты используют стандартные функции Go без внешних зависимостей
4. Тесты покрывают основную логику и структуры данных

## Добавление новых тестов

При добавлении новых функций в handlers.go или database.go:

1. Создайте соответствующий тест в `*_test.go` файле
2. Используйте стандартный пакет `testing`
3. Следуйте существующему стилю именования тестов
4. Добавьте описание теста в этот документ

## Пример добавления теста

```go
func TestNewFunction(t *testing.T) {
    // Подготовка
    input := "test input"
    expected := "expected output"
    
    // Выполнение
    result := newFunction(input)
    
    // Проверка
    if result != expected {
        t.Errorf("Expected %s, got %s", expected, result)
    }
}
```
