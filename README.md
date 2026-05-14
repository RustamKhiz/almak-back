# Almak Backend

Go REST API для авторизации и работы с заказами.

Бэкенд хранит заказ целиком: шапку заказа, статусы и все товарные позиции.

## Что умеет бэк

- загрузка конфигурации из `.env`;
- подключение к PostgreSQL;
- автоприменение миграций через GORM;
- создание дефолтного пользователя;
- логин и выдача JWT;
- CRUD по заказам;
- смена статуса заказа;
- серверная валидация и пересчёт суммы заказа.

## Стек

- Go
- Gin
- GORM
- PostgreSQL
- JWT
- bcrypt

## Структура проекта

- [cmd/main.go](/c:/main/projects/diplom/almak-back/cmd/main.go) — запуск сервера.
- [internal/config/config.go](/c:/main/projects/diplom/almak-back/internal/config/config.go) — загрузка env.
- [internal/database/database.go](/c:/main/projects/diplom/almak-back/internal/database/database.go) — подключение к БД, миграции, дефолтный пользователь.
- [internal/handlers/auth_handler.go](/c:/main/projects/diplom/almak-back/internal/handlers/auth_handler.go) — логин.
- [internal/handlers/order_handler.go](/c:/main/projects/diplom/almak-back/internal/handlers/order_handler.go) — заказы.
- [internal/models/order.go](/c:/main/projects/diplom/almak-back/internal/models/order.go) — шапка заказа.
- [internal/models/door.go](/c:/main/projects/diplom/almak-back/internal/models/door.go) — все дочерние позиции заказа.
- [internal/routes/routes.go](/c:/main/projects/diplom/almak-back/internal/routes/routes.go) — маршруты и CORS.

## Поддерживаемые позиции заказа

Сейчас бэк хранит:

- `InteriorDoors`
- `EntranceDoors`
- `Moldings`
- `Extensions`
- `Capitals`
- `Panelings`

Связь всех дочерних сущностей с заказом идёт через `order_id` и `OnDelete:CASCADE`.

## Модель заказа

`Order` содержит:

- `customer`
- `phone`
- `date`
- `price`
- `prepayment`
- `discount`
- `needsDelivery`
- `deliveryAddress`
- `comment`
- `status`
- `created_at`

и массивы дочерних сущностей:

- `interiorDoors`
- `entranceDoors`
- `moldings`
- `extensions`
- `capitals`
- `panelings`

## Особенности расчёта суммы

Сервер сам пересчитывает `price` и не доверяет итоговой сумме с фронта.

В расчёт сейчас входят:

- межкомнатные двери;
- входные двери;
- погонаж;
- доборы;
- обшивка.

Капитель в серверную сумму не входит, потому что у неё нет цены.

Погонаж считается как:

- `framePrice * frameCount`
- `platbandPrice * platbandCount`

Скидка, предоплата и долг клиента не записываются в `price` как итог к оплате. Это отдельная логика уровня UI.

## Валидация заказа

[order_handler.go](/c:/main/projects/diplom/almak-back/internal/handlers/order_handler.go) проверяет:

- что тело запроса валидно;
- что в заказе есть хотя бы одна позиция;
- что при `needsDelivery = true` указан `deliveryAddress`.

Также сервер нормализует:

- строки через `TrimSpace`;
- опциональные строки в `nil`, если они пустые;
- опциональные числа в `nil`, если значение не задано или `<= 0`;
- адрес доставки в пустую строку, если доставка выключена.

## Как работает создание заказа

`POST /orders`:

1. принимает `orderRequest`;
2. валидирует тело;
3. проверяет состав заказа;
4. пересчитывает `price`;
5. собирает `models.Order`;
6. сохраняет заказ и дочерние сущности;
7. перечитывает заказ с `Preload`;
8. возвращает полный объект.

## Как работает обновление заказа

`PUT /orders/:id` работает как полная перезапись состава заказа:

1. загружает заказ по `id`;
2. обновляет поля шапки;
3. удаляет старые дочерние записи всех типов;
4. создаёт новые дочерние записи заново;
5. перечитывает заказ с `Preload`;
6. возвращает обновлённый заказ.

Это не patch по позициям, а полная замена снимком с фронта.

## Как работает смена статуса

`PATCH /orders/:id/status` принимает:

```json
{
  "status": 2
}
```

Коды:

- `1 -> accepted`
- `2 -> progress`
- `3 -> completed`

После обновления сервер возвращает полный заказ с `Preload`.

## Маршруты

Маршруты регистрируются и с префиксом `/api`, и без него.

Работают оба варианта:

- `POST /login`
- `POST /api/login`

Заказы:

- `POST /orders`
- `GET /orders`
- `GET /orders/:id`
- `PUT /orders/:id`
- `PATCH /orders/:id/status`
- `DELETE /orders/:id`

И те же маршруты через `/api/...`.

Все маршруты заказов защищены `AuthMiddleware` и требуют:

```http
Authorization: Bearer <token>
```

## Дефолтный пользователь

При старте сервер обеспечивает наличие пользователя:

- `login: almak`
- `password: almak05`

## Локальный запуск

### 1. Подготовить PostgreSQL

```sql
CREATE DATABASE orders_db;
```

### 2. Создать `.env`

```env
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=orders_db
JWT_SECRET=super_secret_key
FRONTEND_ORIGINS=http://localhost:4200
```

### 3. Запустить сервер

```bash
go mod tidy
go run cmd/main.go
```

## Полезно помнить при доработках

- `GetOrders()` отдаёт список заказов без preload дочерних позиций.
- `GetOrderByID()` всегда подгружает все дочерние сущности.
- `DeleteOrder()` удаляет заказ каскадно вместе со всеми позициями.
- При добавлении нового типа позиции нужно обновить:
  - модели;
  - миграции;
  - request DTO в handler;
  - `calculateOrderPrice`;
  - mapping create/update;
  - `preloadOrder`.
