# almalak-back

Production-ready REST API backend на Go для управления заказами.

## Stack
- Go
- Gin
- GORM
- PostgreSQL
- JWT (24 часа)
- bcrypt
- .env через `godotenv`

## Структура
```text
cmd/
  main.go

internal/
  config/
    config.go
  database/
    database.go
  models/
    user.go
    order.go
  handlers/
    auth_handler.go
    order_handler.go
  middleware/
    auth_middleware.go
  routes/
    routes.go

.env
go.mod
README.md
```

## Запуск локально
1. Установите зависимости:
```bash
go mod tidy
```

2. Создайте базу данных PostgreSQL:
```sql
CREATE DATABASE orders_db;
```

3. Настройте `.env`:
```env
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=orders_db
JWT_SECRET=super_secret_key
```

4. Запустите сервер:
```bash
go run cmd/main.go
```

## Аутентификация
- Регистрации нет.
- При первом запуске автоматически создаётся пользователь:
  - `login: admin`
  - `password: admin`
- Логин:
```http
POST /login
Content-Type: application/json

{
  "login": "admin",
  "password": "admin"
}
```

Ответ:
```json
{
  "token": "<JWT_TOKEN>"
}
```

## Order model
- `id` (uint, primary key)
- `customer` (string)
- `phone` (string)
- `date` (string)
- `count` (int)
- `price` (float64)
- `prepayment` (float64)
- `status` (string)
- `created_at` (auto)

## Защищённые эндпоинты
Требуют заголовок:
```http
Authorization: Bearer <token>
```

- `POST /orders`
- `GET /orders`
- `GET /orders/:id`
- `PUT /orders/:id`
- `DELETE /orders/:id`

## CORS
Разрешён origin:
- `http://localhost:4200`

Разрешённые заголовки:
- `Authorization`
- `Content-Type`
