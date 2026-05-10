# Booking Service — Learning Go Project

Полноценный сервис бронирования переговорок с API на Go и веб-интерфейсом на React.
Создан для изучения ключевых навыков Go-разработчика.

## Запуск

Одна команда поднимает всё:

```bash
make docker-up     # postgres + redis + api + worker + web
make docker-logs   # логи всех сервисов
make docker-down   # остановить
```

После запуска открой **http://localhost:3000** — там фронтенд.

| Порт | Сервис |
|------|--------|
| `:3000` | Фронтенд (React + nginx) |
| `:8080` | REST API (Go) |
| `:5432` | PostgreSQL |
| `:6379` | Redis |

### Локальная разработка

```bash
# Бэкенд
cp .env.example .env
make run            # API на :8080
make worker         # воркер в другом терминале

# Фронтенд
cd web && npm install && npm run dev   # :5173 с прокси на :8080
```

## Скриншот

Открой http://localhost:3000 после `make docker-up`. Увидишь:

- Шапка с навигацией по датам (← сегодня →)
- Карточки переговорок с таймлайном (07:00–22:00, слоты по 30 мин)
- Свободные слоты кликабельны → диалог бронирования
- Секция «Мои бронирования» с кнопкой отмены
- Защита от двойного бронирования (409 Conflict)

## Структура проекта

```
.
├── cmd/                  # Точки входа
│   ├── api/              # HTTP сервер
│   └── worker/           # Фоновый воркер
├── internal/             # Приватный код
│   ├── config/           # Конфигурация (env)
│   ├── handler/          # HTTP handlers + middleware
│   ├── model/            # Доменные модели
│   ├── repository/       # PostgreSQL + Redis
│   ├── service/          # Бизнес-логика
│   └── worker/           # Воркер (ticker + context)
├── pkg/                  # Публичные пакеты
│   ├── errors/           # Кастомные ошибки
│   └── httputil/         # HTTP response helpers
├── web/                  # Фронтенд (React + TypeScript + Tailwind + shadcn/ui)
│   ├── src/
│   │   ├── api/          # API клиент
│   │   ├── components/   # UI компоненты
│   │   └── App.tsx       # Главное приложение
│   ├── Dockerfile        # Multi-stage build → nginx
│   └── nginx.conf        # Прокси /api → Go бэкенд
├── migrations/           # SQL миграции
├── docker-compose.yml
├── Dockerfile            # Multi-stage build для Go
└── Makefile
```

## API Endpoints

| Method | Path | Описание |
|--------|------|----------|
| `GET` | `/health` | Health check |
| `POST` | `/rooms` | Создать переговорку |
| `GET` | `/rooms` | Список переговорок |
| `GET` | `/rooms/{id}` | Получить переговорку |
| `DELETE` | `/rooms/{id}` | Удалить переговорку |
| `POST` | `/bookings` | Создать бронирование |
| `GET` | `/bookings` | Бронирования пользователя |
| `GET` | `/bookings/{id}` | Получить бронирование |
| `DELETE` | `/bookings/{id}` | Отменить бронирование |
| `GET` | `/rooms/{id}/schedule?date=2026-05-10` | Расписание на день |

### Примеры

```bash
# Создать переговорку
curl -X POST http://localhost:8080/rooms \
  -H "Content-Type: application/json" \
  -d '{"name":"Big Room","capacity":10}'

# Забронировать
curl -X POST http://localhost:8080/bookings \
  -H "Content-Type: application/json" \
  -H "X-User-ID: user1" \
  -d '{"room_id":1,"title":"Sprint Planning","start_time":"2026-05-12T10:00:00Z","end_time":"2026-05-12T11:00:00Z"}'

# Попытка двойного бронирования → 409 Conflict
curl -X POST http://localhost:8080/bookings \
  -H "Content-Type: application/json" \
  -H "X-User-ID: user2" \
  -d '{"room_id":1,"title":"Double","start_time":"2026-05-12T10:30:00Z","end_time":"2026-05-12T11:30:00Z"}'

# Расписание комнаты
curl "http://localhost:8080/rooms/1/schedule?date=2026-05-12"
```

## Что изучаешь в этом проекте

### Backend (Go)

| # | Навык | Где |
|---|-------|-----|
| 1 | **Структура проекта** | `cmd/`, `internal/`, `pkg/` |
| 2 | **Clean Architecture** | handler → service → repository |
| 3 | **REST API** | stdlib `net/http.ServeMux` (Go 1.22+) |
| 4 | **PostgreSQL + sqlx** | `StructScan`, `sqlx.In`, EXCLUDE constraint |
| 5 | **Redis** | cache-aside, TTL, distributed lock |
| 6 | **Конкурентность** | race condition → lock + DB constraint |
| 7 | **Background worker** | `time.Ticker`, context cancellation |
| 8 | **Middleware chain** | recovery, logging, rate limit, CORS, auth |
| 9 | **context.Context** | от HTTP handler до SQL запросов |
| 10 | **Обработка ошибок** | кастомные AppError, wrapped errors |
| 11 | **Graceful shutdown** | signal.Notify, srv.Shutdown |
| 12 | **Тестирование** | table-driven tests, httptest, benchmarks |
| 13 | **Docker** | multi-stage build, non-root, healthchecks |

### Frontend (React)

| # | Навык | Где |
|---|-------|-----|
| 1 | **React + TypeScript** | Vite, строгая типизация |
| 2 | **Tailwind CSS v4** | утилити-классы, OKLCH-палитра |
| 3 | **shadcn/ui** | Button, Card, Dialog, Badge, Input, Label |
| 4 | **API клиент** | fetch + типизированные ответы |
| 5 | **nginx reverse proxy** | `/api/*` → Go бэкенд |
| 6 | **Docker** | multi-stage build → nginx |

## Команды

```bash
make docker-up       # поднять всё (бэк + фронт + БД + Redis)
make docker-down     # остановить
make docker-logs     # логи
make build           # собрать Go бинарники
make test            # тесты с -race
make bench           # бенчмарки
make lint            # go vet + go fmt
```

## Чего тут нет (и стоит изучить отдельно)

- **gRPC** — для микросервисов
- **CI/CD** — GitHub Actions
- **Kubernetes manifests** — deployment, service, configmap
- **OpenTelemetry** — распределённая трассировка
- **Message queues** — Kafka, RabbitMQ, NATS
- **JWT auth** — сейчас заглушка через X-User-ID header
- **Integration tests** — с testcontainers-go
- **OpenAPI/Swagger** — автогенерация документации API
