# go-musthave-diploma-tpl

Шаблон репозитория для индивидуального дипломного проекта курса «Go-разработчик»

# Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без
   префикса `https://`) для создания модуля

# Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m master template https://github.com/yandex-praktikum/go-musthave-diploma-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/master .github
```

Затем добавьте полученные изменения в свой репозиторий.


# Пример ручного запрос на Windows

1. Регистрация пользователя

curl.exe -v -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/user/register \
  -d "{\"login\": \"testuser@example.com\", \"password\": \"password123\"}" \
  -c cookies.txt

2. Логин пользователя

curl.exe -v -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/user/login \
  -d "{\"login\": \"testuser@example.com\", \"password\": \"password123\"}" \
  -c cookies.txt


3. Загрузка первого заказа

curl.exe -v -H "Content-Type: text/plain" \
  -X POST http://localhost:8080/api/user/orders \
  -b cookies.txt \
  -d "12345678903"

4. Получение списка заказов

curl.exe -v -H "Content-Type: application/json" \
  -X GET http://localhost:8080/api/user/orders \
  -b cookies.txt
