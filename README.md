# kintsugi-storage

`kintsugi-storage` - HTTP-сервис для хранения JSON-объектов в оперативной памяти с поддержкой TTL, сохранения состояния на диск, health probes и Prometheus-метрик.

## Реализованный функционал

- `PUT /objects/{key}` - сохранить или перезаписать JSON-объект.
- `GET /objects/{key}` - получить объект по ключу.
- `GET /probes/liveness` - liveness probe.
- `GET /probes/readiness` - readiness probe.
- `GET /metrics` - метрики в Prometheus exposition format.
- `GET /docs` - Swagger UI.


## Примеры запросов и ответов

### Happy path

#### Сохранить объект без TTL

```bash
curl -i -X PUT "http://localhost:8080/objects/user-1" \
  -H "Content-Type: application/json" \
  -d '{"name":"alice","age":30}'
```

Пример ответа:

```http
HTTP/1.1 201 Created
Date: Mon, 24 Mar 2026 12:00:00 GMT
Content-Length: 0
```

#### Получить сохраненный объект

```bash
curl -i "http://localhost:8080/objects/user-1"
```

Пример ответа:

```http
HTTP/1.1 200 OK
Content-Type: application/json
Date: Mon, 24 Mar 2026 12:00:01 GMT
Content-Length: 25

{"name":"alice","age":30}
```

#### Сохранить объект с TTL

```bash
EXP=$(LC_TIME=C date -u -v+30S "+%a, %d %b %Y %H:%M:%S UTC")

curl -i -X PUT "http://localhost:8080/objects/session-1" \
  -H "Content-Type: application/json" \
  -H "Expires: $EXP" \
  -d '{"status":"active"}'
```

Пример ответа:

```http
HTTP/1.1 201 Created
Date: Mon, 24 Mar 2026 12:00:02 GMT
Content-Length: 0
```

#### Получить объект до истечения TTL

```bash
curl -i "http://localhost:8080/objects/session-1"
```

Пример ответа:

```http
HTTP/1.1 200 OK
Content-Type: application/json
Date: Mon, 24 Mar 2026 12:00:03 GMT
Content-Length: 19

{"status":"active"}
```

#### Получить метрики

```bash
curl -i "http://localhost:8080/metrics"
```

Пример ответа:

```http
HTTP/1.1 200 OK
Content-Type: text/plain; version=0.0.4; charset=utf-8
Date: Mon, 24 Mar 2026 12:00:06 GMT

# HELP storage_put_requests_total Total number of PUT /objects requests.
# TYPE storage_put_requests_total counter
storage_put_requests_total 2
```

## Принятые решения

- Хранилище in-memory и потокобезопасно за счет `sync.RWMutex`.
- При повторном `PUT` по тому же ключу объект перезаписывается.
- `Expires` задается как абсолютное время в формате `RFC1123`, например:
  `Mon, 23 Mar 2026 12:30:00 UTC`.
- Если `Expires` указывает на момент в прошлом, запрос отклоняется с `400 Bad Request`.
- В сервисе принимается любой валидный JSON, а не только JSON object.
- Просроченные объекты не попадают в snapshot и не восстанавливаются после рестарта.
- Snapshot сохраняется периодически в фоне и дополнительно при graceful shutdown.

## Архитектура

- `internal/storage` - core-логика хранения, TTL, snapshot/restore.
- `internal/persistence` - работа со snapshot-файлом на диске.
- `internal/httpapi` - HTTP handlers, middleware, роутинг и встроенная документация.
- `internal/observability` - логирование и прикладные метрики.
- `internal/app` - сборка зависимостей, запуск воркеров и graceful shutdown.

## Запуск

### Локально

```bash
make run
```

### В Docker Compose

```bash
docker compose up --build
```

## Тесты и качество

```bash
make lint
make test
make test-race
```

## Ограничения текущей реализации

- Сервис рассчитан на single-instance deployment.
- При горизонтальном масштабировании без внешнего shared storage инстансы будут расходиться по данным.
- Persistence реализован через snapshot-файл, без WAL и распределенной репликации.

