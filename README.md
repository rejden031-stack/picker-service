# Picker Search Service

Учебный Go-сервис по Clean Architecture: когда на ячейке нет того, что там должно
лежать, старшему склада подсказывают, в каких других ячейках этот товар есть.
Стек: Go 1.26, chi, pgx v5, Postgres (Docker), Redis, Prometheus + Grafana, JWT.

## Пользовательский сценарий
1. Старший смотрит ячейку, где товар «пропал».
2. `GET /cells/{id}/products` — что должно лежать в ячейке.
3. `GET /products/{id}/candidates?exclude_cell=...` — где этот товар ещё есть.
   Ответ в формате `2.A.01.01` (этаж.стеллаж.полка.ячейка).

## Слои
```
cmd/            CLI: server, migrate, seedusers, importer (composition root)
internal/delivery/rest   HTTP: chi-роутер, Handler, middleware (auth, роли, rate-limit), метрики
internal/application     Юзеркейсы (Login, ChangePassword, Tree, ListCellProducts, FindCandidates,
                         Import, UserAdmin, асинхронный аудит-райтер)
                         + порты (store, cache, auth): интерфейсы владеют те, кто использует
internal/domain          Сущности: catalog, location, auth, audit
internal/adapters        postgres (хранилища), authimpl (JWT, bcrypt), cache (Noop/redis)
```
Поток запроса: HTTP → middleware (Logger, Metrics, Auth) → Handler → UseCase → Port → Adapter → Postgres.
Благодаря портам юзеркейсы и HTTP не зависят от Postgres/Redis — легко подменять тестами.

## Запуск
```powershell
docker compose up -d       # postgres (порт 5433), redis, prometheus, grafana
go run ./cmd/migrate       # применяет migrations/*.up.sql (папка ОБЯЗАТЕЛЬНО "migrations")
go run ./cmd/seedusers     # заводит senior/worker
go run ./cmd/importer      # заливает testdata/seed/catalog.json
go run ./cmd/server        # http://localhost:8000
```
Внимание: Docker-Postgres сидит на хосте на 5433, т.к. локальный Windows-PostgreSQL занимает 5432.

## Учётные данные
| user   | pass   | что может |
|--------|--------|-----------|
| senior | 123456 | всё (кроме /metrics и админки) |
| worker | 123456 | только дерево локаций; senior-ручки → 403 |
| admin  | задаётся явно | роли/блокировки/пароли пользователей, журнал аудита |

Роль `admin` в проде создаёт сам сервис из env `ADMIN_USERNAME`/`ADMIN_PASSWORD`
при первом старте (идемпотентно: существует — повышает роль/разблокирует).
Админ сам себе не снимет роль и себя не заблокирует — защита от «последнего админа».

## API
| Метод и путь | Доступ | Ответ |
|---|---|---|
| `GET /healthz` | — | `ok` (liveness) |
| `GET /readyz` | — | `ok` / `503` (readiness: Postgres, Redis) |
| `POST /api/v1/auth/login` | — | `{token, role}` |
| `POST /api/v1/auth/change-password` | Bearer | `204`; 400 — старый пароль/мин. длина 8 |
| `GET /api/v1/locations/tree` | Bearer | дерево этажей/стеллажей/полок/ячеек |
| `GET /api/v1/cells/{cellID}/products` | senior | `{products:[{id,sku,name,barcode}]}` |
| `GET /api/v1/products/{productID}/candidates?exclude_cell={id}` | senior | `{candidates:[{cell_id,address,qty,is_primary}]}` |
| `GET /api/v1/admin/users` | admin | `{users:[{id,username,role}]}` |
| `POST /api/v1/admin/users` | admin | `201`; `409` занято, `400` плохая роль |
| `PATCH /api/v1/admin/users/{id}/role` | admin | `204`; `409` последний админ |
| `PATCH /api/v1/admin/users/{id}/blocked` | admin | `204`; `409` блокировка себя |
| `POST /api/v1/admin/users/{id}/reset-password` | admin | `204` |
| `GET /api/v1/admin/audit?limit=&offset=` | admin | `{entries:[{id,time,actor_id,actor_name,action,entity_id,details}]}` |
| `GET /metrics` | — | текст Prometheus |

## Метрики и Grafana
- Свои метрики: `http_requests_total{method,path,status}`, `http_request_duration_seconds`, `pgxpool_{total,idle,acquired,max}_connections` + стандартные `go_*`.
- Prometheus скрейпит `host.docker.internal:8000` (интервал 5s), `deploy/prometheus/prometheus.yml`.
- Grafana: `http://localhost:3000`, admin/admin, дашборд `Picker Service` (RPS, latency p99, Postgres pool, goroutines). Провайдеры данных и дашбордов — в `deploy/grafana/provisioning`.
- Грабля: монтировать в volume `grafana-data` нужно ПАПКУ с дашбордами, не отдельный файл.

## Нагрузочные замеры (dev-ноутбук, один инстанс)

Измерялось на работающем сервере + Postgres в Docker на одной машине
(`hey` из `go install github.com/rakyll/hey@latest`, Redis выключен — весь поток идёт в БД).

| Что | Результат |
|---|---|
| `GET /healthz` (без БД) | ~14 000 RPS, avg ~13 мс (c=200) |
| `GET /api/v1/cells/{id}/products` | ~4200 RPS при c=100, avg ~22 мс, p99 ~87 мс |
| `GET /api/v1/products/{id}/candidates` | ~4200 RPS при c=100 (аналогично) |
| `POST /api/v1/auth/login` | ~160 RPS — bcrypt ~60 мс на запрос (CPU-bound) |

Наблюдения:
- Постгрес-запросы индексные (~1 мс), пул pgx (12) под нагрузкой почти простаивает
  (`pgxpool_acquired_connections` ~1–2).
- Разница между серверной гистограммой `/metrics` и латентностью клиента — шум
  разработческой машины (loopback, Windows-схед улер, консольный логгер).
- Боттлнеки: логгер на каждый запрос, отсутствие кэша (Noop), bcrypt на логине,
  HTTP/1.1, один инстанс.
- Что уже внедрено: JSON-логи, rate-limiter на `/auth/login`, таймауты сервера.
  Как ускорить дальше: Redis-кэш, семплинг логов, поднять пул,
  горизонтальное масштабирование.

## Тесты
```powershell
go test ./...                                             # юниты (фейки портов)
go test -short ./...                                      # интеграцию пропустит (SKIP)
$env:TEST_DATABASE_DSN = "postgres://picker:picker@localhost:5433/picker?sslmode=disable"
go test ./...                                             # + живой Postgres
```
Юниты — на фейках портов: `auth/login_test.go` (в т.ч. блокировка/аудит), `catalog/*`,
`locations/resolve_cell_test.go`, `rest/handler_test.go` (RBAC, админка, rate-limit),
`rest/ipresolver_test.go`, `ratelimit/redis_test.go`; интеграция — `postgres/store_integration_test.go`.

## Переменные окружения
`HTTP_ADDR` (:8000), `DATABASE_DSN`, `JWT_SECRET`, `REDIS_ENABLED` (выкл), `REDIS_ADDR`,
`REDIS_PASSWORD`, `LOG_LEVEL` (info/debug/warn/error), `LOGIN_RATE_PER_SEC` (3),
`LOGIN_RATE_BURST` (6), `TRUSTED_PROXIES` (CIDR через запятую), `METRICS_USER`,
`METRICS_PASSWORD`. Дефолты — в `internal/infra/config`. Дополнительно:
`ADMIN_USERNAME`/`ADMIN_PASSWORD` — стартовый админ (пустые = не создаётся).

## Прод-гигиена
- Логи — структурированный JSON (slog): request_id, method, path-шаблон, status, duration_ms, bytes;
  паники перехватывает кастомный Recoverer и отвечает JSON `{"error":"internal error"}`.
- Метрики пишут лейбл пути по шаблону роута (`/api/v1/cells/{cellID}/products`), а не по
  конкретному id — без раздувания серий.
- `/readyz` (readiness) проверяет Postgres и Redis; `/healthz` — только живость процесса.
- `/auth/login` лимитируется token bucket'ом на IP (`LOGIN_RATE_PER_SEC`/`LOGIN_RATE_BURST`) → `429`.
  Локально — in-memory; при `REDIS_ENABLED=true` — общий distributed-лимитер на Redis
  (Lua-скрипт, атомарный, ключи с TTL), корректный при горизонтальном масштабировании.
- Прокси-заголовкам (`X-Forwarded-For`/`X-Real-IP`) верим только от сетей из `TRUSTED_PROXIES` —
  подделка лимитер на IP не обойдёт.
- `/metrics` закрывается Basic Auth через `METRICS_USER`/`METRICS_PASSWORD`.
- **Аудит** `audit_events`: кто, что, когда, с какого IP. Логин/админ-действия пишутся
  синхронно (долговечность важнее), просмотры каталога — через асинхронный буфер
  (не блокирует запрос; при переполнении дропает с warn). Блокировка пользователя отрубает
  вход (`403 account blocked`), а звонки `product_viewed`/`candidates_viewed` видны админу.
- `http.Server` с полными таймаутами (ReadHeader 5s, Read 30s, Write 60s, Idle 120s); graceful shutdown.

## Деплой в прод
- `deploy/Dockerfile` — multistage: сборка (без CGO, `-trimpath`), рантайм alpine 3.20 под
  не-root пользователем `app`; содержит `migrate` и папку `/migrations`.
- `docker-compose.prod.yml` — прод-стек: однократный job `migrate` (работает до старта API,
  `service_completed_successfully`), `picker-api` (`read_only`, `tmpfs /tmp`, healthcheck `/readyz`),
  Postgres и Redis (с паролем и healthcheck, работающим и без пароля).
  Запуск: `docker compose -f docker-compose.prod.yml --env-file .env.prod up -d`.
- CI (`.github/workflows/ci.yml`): gofmt + vet + тесты с живыми Postgres/Redis (+ прогон миграций),
  сборка бинаря и образа. Ожидает `TEST_DATABASE_DSN` и `TEST_REDIS_ADDR` — интеграционные тесты
  скипаются без переменных, поэтому локально `go test ./...` работает без Docker.

### Бэкапы и восстановление
- `deploy/backup/` — контейнер с `crond`: ежедневно в 02:00 `pg_dump -Fc` в том `backups`,
  ротация 14 дней. Поднимается командой `docker compose -f docker-compose.prod.yml --env-file .env.prod up -d backup`.
- Ручной бэкап:
  `docker compose -f docker-compose.prod.yml --env-file .env.prod run --rm backup /usr/local/bin/backup`
- Восстановление в отдельную БД (drill):
  `docker compose -f docker-compose.prod.yml --env-file .env.prod run --rm backup /usr/local/bin/restore <файл.dump> <имя_бд>`
- Правило прода: бэкап считается живым, только когда drill восстановления реально прогнан.
  Для крупных данных логического дампа мало — там WAL-архивация + PITR (pgBackRest/WAL-G).
  Вынос копии за пределы хоста (S3 и т.п.) и шифрование — обязательно.

### TLS и ingress
- `docker-compose.ingress.yml` — Caddy впереди `picker-api`: TLS-терминация на `:443`
  (наружу `${APP_HTTPS_PORT:-8443}`), HTTP→HTTPS redirect, проставляет прокси-заголовки.
  Поднимается вместе с прод-стеком (тот же проект):
  `docker compose -f docker-compose.prod.yml -f docker-compose.ingress.yml --env-file .env.prod up -d`
- Сертификаты — `deploy/certs/` (самоподписанные, SAN localhost; генерация через контейнер
  `docker run --rm -v "$PWD/deploy/certs:/certs" alpine:3.20 /bin/sh -c 'apk add --no-cache openssl && ...'`).
  Для публичного домена в Caddyfile оставляют `tls {}` — Let's Encrypt выпишет сам.
- `TRUSTED_PROXIES` = подсеть нgress/прокси (для compose-стека это сеть bridge, напр. `172.22.0.0/16`):
  приложение верит `X-Forwarded-For` только от неё. Бонус: Caddy сам режет подделанные
  прокси-заголовки клиента, так что обход лимита не проходит сквозь прокси.

## Грабли этого проекта (памятка)
1. **Sequence-drift.** Сиды и импорт писали строки с явными id без `setval()` —
   первый INSERT без id коллайдил с сидами (`duplicate floors_pkey` и т.п.).
   Лечение: миграции `0004` и `0005`. Правило: явный id → сразу `setval`.
2. **Имя папки.** Мигратор ищет `migrations`, а не `migration`; версии — числовой
   префикс `NNNN_`. Применённые миграции не редактируют, а добавляют новую.
3. **pgx v5.11**: `tx.Conn().PgConn().Exec(ctx, sql)` — без лишних аргументов.
4. **Порт 5433** у Docker-Postgres (конфликт с локальным 5432).
5. **Залипший server.exe** может держать :8000 — `Stop-Process -Id <pid>`.
6. **Интеграционный тест** использует уникальные коды на каждый прогон
   (суффикс из времени) — иначе упавший прогон ломает следующий на UNIQUE.
7. **Docker: `ENTRYPOINT` перекрывает `command` из compose.** В образах с двумя
   бинарями (server+migrate) основную команду задают через `CMD`, а композовский
   `command` переопределяет её (иначе migrate-контейнер запустится как сервер).
   Проверено на `deploy/Dockerfile`.
8. **Rate-limit за docker-NAT.** Из контейнера peer = gateway bridge (172.17/22.0.1),
   а не 127.0.0.1 — так что `TRUSTED_PROXIES=127.0.0.1` в compose-smoke не доверял
   XFF. Это правильно: лимитер считается по реальному источнику.
9. **Свежая прод-БД пустая**: схема от миграций, но юзеров нет — стартовые роли
   даёт `cmd/seedusers` (в dev-БД), в проде админа поднимает бутстрап из
   `ADMIN_USERNAME`/`ADMIN_PASSWORD` при старте сервера.
10. **Роль хранится в JWT на момент выпуска.** Смена роли администратором действует
    со следующего логина: токен несёт старую роль до истечения (12ч). Для мгновенной
    отмены — блокировка: она проверяется при каждом логине.