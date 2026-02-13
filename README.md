# almalak-back

Go backend сервис для проекта Almalak.

## Стек
- Go 1.22+
- PostgreSQL
- Docker / Docker Compose (опционально)
- golang-migrate (для миграций, если используется)

## Быстрый старт

### 1. Клонирование и переход в проект
```bash
git clone <repo-url>
cd almalak-back
```

### 2. Настройка окружения
Создайте файл `.env` (или скопируйте из `.env.example`, если он появится позже):

```env
APP_ENV=development
APP_PORT=8080
APP_HOST=0.0.0.0

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=almalak
DB_SSLMODE=disable

JWT_SECRET=change_me
JWT_TTL=24h
```

### 3. Установка зависимостей
```bash
go mod tidy
```

### 4. Запуск приложения
```bash
go run ./cmd/server
```

Если у вас другой entrypoint, замените путь `./cmd/server` на актуальный.

## Сборка
```bash
go build -o bin/app ./cmd/server
```

## Тесты
```bash
go test ./...
```

## Линтинг (опционально)
```bash
golangci-lint run
```

## Docker

### Запуск через Docker Compose
```bash
docker compose up --build
```

### Ручная сборка образа
```bash
docker build -t almalak-back .
docker run --env-file .env -p 8080:8080 almalak-back
```

## Миграции базы данных
Если в проекте используются SQL-миграции:

```bash
migrate -path ./migrations -database "postgres://postgres:postgres@localhost:5432/almalak?sslmode=disable" up
```

Откат на 1 шаг:

```bash
migrate -path ./migrations -database "postgres://postgres:postgres@localhost:5432/almalak?sslmode=disable" down 1
```

## Переменные окружения

| Переменная | Описание | Пример |
|---|---|---|
| `APP_ENV` | Режим окружения | `development` |
| `APP_HOST` | Хост приложения | `0.0.0.0` |
| `APP_PORT` | Порт приложения | `8080` |
| `DB_HOST` | Хост БД | `localhost` |
| `DB_PORT` | Порт БД | `5432` |
| `DB_USER` | Пользователь БД | `postgres` |
| `DB_PASSWORD` | Пароль БД | `postgres` |
| `DB_NAME` | Имя базы | `almalak` |
| `DB_SSLMODE` | SSL режим БД | `disable` |
| `JWT_SECRET` | Секрет подписи JWT | `change_me` |
| `JWT_TTL` | Время жизни JWT | `24h` |

## Рекомендуемая структура проекта
```text
.
├─ cmd/
│  └─ server/
│     └─ main.go
├─ internal/
│  ├─ config/
│  ├─ handler/
│  ├─ service/
│  ├─ repository/
│  └─ model/
├─ migrations/
├─ pkg/
├─ .env
├─ .gitignore
├─ go.mod
└─ README.md
```

## API

### Healthcheck
```http
GET /health
```

Пример ответа:

```json
{
  "status": "ok"
}
```

## Полезные команды
```bash
# Форматирование
go fmt ./...

# Проверка зависимостей
go mod verify

# Очистка кеша сборки
go clean -cache
```

## Roadmap
- [ ] Добавить `.env.example`
- [ ] Добавить `Dockerfile` и `docker-compose.yml`
- [ ] Добавить миграции в `migrations/`
- [ ] Добавить OpenAPI/Swagger спецификацию
- [ ] Добавить CI (lint + test)

## Лицензия
Укажите лицензию проекта (например, MIT).
