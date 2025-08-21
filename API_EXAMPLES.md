# Примеры использования API

## Получение баланса пользователя

### Запрос
```http
GET /api/user/balance HTTP/1.1
Cookie: session_id=your_session_id
```

### Ответ
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "current": 500.5,
    "withdrawn": 42
}
```

## Вывод средств

### Запрос
```http
POST /api/user/balance/withdraw HTTP/1.1
Content-Type: application/json
Cookie: session_id=your_session_id

{
    "order": "2377225624",
    "sum": 751
}
```

### Возможные ответы

#### Успешный вывод средств
```http
HTTP/1.1 200 OK
```

#### Недостаточно средств
```http
HTTP/1.1 402 Payment Required
```

#### Заказ уже существует
```http
HTTP/1.1 409 Conflict
```

## Получение информации о выводах средств

### Запрос
```http
GET /api/user/withdrawals HTTP/1.1
Cookie: session_id=your_session_id
```

### Возможные ответы

#### Есть выводы средств
```http
HTTP/1.1 200 OK
Content-Type: application/json

[
    {
        "order": "2377225624",
        "sum": 500,
        "processed_at": "2020-12-09T16:09:57+03:00"
    },
    {
        "order": "1234567890",
        "sum": 250,
        "processed_at": "2020-12-08T14:30:00+03:00"
    }
]
```

#### Нет выводов средств
```http
HTTP/1.1 204 No Content
```

## Система расчёта баллов лояльности (заглушка)

### Запрос
```http
GET /api/orders/123456789 HTTP/1.1
```

### Возможные ответы

#### Успешное начисление
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "order": "123456789",
    "status": "PROCESSED",
    "accrual": 45.67
}
```

#### Заказ не подходит для начисления
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "order": "123456789",
    "status": "INVALID"
}
```

## Полный цикл работы

1. **Регистрация пользователя**
   ```http
   POST /api/user/register HTTP/1.1
   Content-Type: application/json
   
   {
       "login": "user@example.com",
       "password": "password123"
   }
   ```

2. **Вход в систему**
   ```http
   POST /api/user/login HTTP/1.1
   Content-Type: application/json
   
   {
       "login": "user@example.com",
       "password": "password123"
   }
   ```

3. **Загрузка заказа**
   ```http
   POST /api/user/orders HTTP/1.1
   Cookie: session_id=your_session_id
   
   123456789
   ```

4. **Автоматическая обработка начисления**
   - Система автоматически запрашивает информацию о начислении
   - Обновляет статус заказа и начисление баллов

5. **Проверка баланса**
   ```http
   GET /api/user/balance HTTP/1.1
   Cookie: session_id=your_session_id
   ```

6. **Вывод средств (если достаточно баллов)**
   ```http
   POST /api/user/balance/withdraw HTTP/1.1
   Content-Type: application/json
   Cookie: session_id=your_session_id
   
   {
       "order": "987654321",
       "sum": 100
   }
   ```

7. **Просмотр заказов**
   ```http
   GET /api/user/orders HTTP/1.1
   Cookie: session_id=your_session_id
   ```

8. **Просмотр выводов средств**
   ```http
   GET /api/user/withdrawals HTTP/1.1
   Cookie: session_id=your_session_id
   ```

## Примечания

- Все хендлеры требуют аутентификации (кроме регистрации и входа)
- Система лояльности работает в фоновом режиме
- Начисления происходят асинхронно после загрузки заказа
- Заглушка системы лояльности генерирует случайные результаты для демонстрации
