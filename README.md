# Service template

Каркас Go-модулей с HTTP/OpenAPI, gRPC/protobuf и событийным взаимодействием через `go-sdk`.

## Проверка и локальное окружение

Нужны mise, Python 3 и Docker.

```sh
mise trust
mise install
mise run generate-module billing
mise run all
mise run local-up
```

`local-up` находит все `modules/*/compose.yaml`, генерирует контракты и собирает
Linux-бинарники через модульные задачи mise. Затем запускает общую инфраструктуру,
создаёт базы и пользователей из модульных `docker/mysql/init.sql` и поднимает
сервисы. SQL подготовки запускается при каждом старте, чтобы новые модули работали
и с уже существующим volume MySQL.

Корневой `compose.yaml` содержит только общую инфраструктуру: MySQL, RabbitMQ и Redis (для сессий). Каждый модуль описывает
свои сервисы, очередь, настройки подключения и подготовку базы у себя.

```sh
mise run local-config
mise run local-status
mise run local-logs
mise run local-down
```

`local-config` показывает объединённую конфигурацию без запуска контейнеров.
`local-down` сохраняет volumes с данными. Имя Compose-проекта задаётся через
`LOCAL_ENV_PROJECT` (по умолчанию `distributed-programming-local`).

Для первого созданного модуля по умолчанию доступны gRPC на 8081, HTTP на 8082 и health consumer
на 8083. Остальные модули получают следующие свободные порты по порядку имён;
`local-up` печатает назначенные порты. Назначения можно задать явно через
`<MODULE>_GRPC_PORT`, `<MODULE>_HTTP_PORT`, `<MODULE>_MESSAGE_HANDLER_PORT`.
Инфраструктурные порты переопределяются через `LOCAL_MYSQL_PORT`, `LOCAL_AMQP_PORT`
и `LOCAL_AMQP_MANAGEMENT_PORT`.

```sh
curl -X POST localhost:8082/api/public/v1/echo \
  -H 'Content-Type: application/json' -d '{"body":"привет"}'
```

Ответ содержит `operationID`; consumer пишет `echo received` в JSON-лог.
Подробности контрактов, настроек и гарантий: [README шаблона](mise-tasks/generate-module.d/README.md.tpl).

## Gateway

Единая точка входа — [gateway](https://github.com/distributed-programming-2026/gateway),
исходники которого лежат в `gateway/` (коммит `0c05cd3`) и собираются его собственными задачами:
`mise run //gateway:build`, `//gateway:all`. Каталог не меняется под проект, чтобы его было
просто обновлять из исходного репозитория.

Маршруты проекта — в `deploy/gateway/config`, compose-сервис — в `deploy/gateway/compose.yaml`.
`local-up` собирает бинарник gateway, монтирует конфигурацию и поднимает его на порту 8000
(`LOCAL_GATEWAY_PORT`), health — на 15000 (`LOCAL_GATEWAY_HEALTH_PORT`).

| Путь | Сервис | Интерцептор session |
| --- | --- | --- |
| `POST /api/public/v1/register`, `/login` | auth | нет |
| `POST /api/public/v1/logout`, `/password` | auth | да, `Authorization` сохраняется |
| `GET`/`PUT /api/public/v1/users/me` | identity | да |

Интерцептор session проверяет `Authorization: Bearer <token>`, выставляет
`X-Auth-User` и `X-Auth-Session` или отвечает 401. Эти заголовки перечислены в
`internalHeaders`, поэтому gateway удаляет их из запросов клиента.

```sh
curl -X POST localhost:8000/api/public/v1/login \
  -H 'Content-Type: application/json' -d '{"login":"ivan","password":"password1"}'
curl localhost:8000/api/public/v1/users/me -H 'Authorization: Bearer <token>'
```

## Новый модуль и миграции

```sh
mise run generate-module billing
mise run //modules/billing:all
mise run //modules/billing:generate-migration "Create invoices table"
```

Исходник шаблона находится в `mise-tasks/generate-module.d` и не входит в `go.work`
или список модулей mise. Все файлы шаблона имеют суффикс `.tpl`.
Генератор записывает их в `modules/<name>` без этого суффикса, переименовывает module path, импорты,
каталог `internal/template`, protobuf-сервисы, CLI/env-префиксы и модульный Compose.
Затем генерирует Go-код из контрактов и добавляет модуль в `go.work`.
Имя должно соответствовать `[a-z][a-z0-9_]*`; существующий каталог не перезаписывается.

Генератор миграций создаёт `version<timestamp>.go` и регистрирует миграцию в
`factory.go`. Пока `Up` не реализован, миграция возвращает ошибку. Версия всегда
больше существующих, в том числе при генерации нескольких миграций в одну секунду.

Общий exchange `domain-event` описан в `lib/event`. Модули описывают свои очереди
и bindings, сохраняя общий exchange.

## Проверки

```sh
mise run all
mise run //modules/billing:integration-tests
mise run check-generator
```

`all` выполняет генерацию, tidy, Linux-сборку, unit и интеграционные тесты, lint. Интеграционные
тесты находятся в `internal/integration-tests` и исключены из обычной задачи `test`.
Задача mise собирает Linux-бинарник и Docker-образ модуля. `TestMain` поднимает
MySQL, RabbitMQ, сервис и consumer в общей изолированной Docker-сети.
Оба приложения запускаются из собранного образа; API доступны через случайные
порты хоста.

Проверка генераторов работает во временной копии: создаёт `sample_echo`,
проверяет миграции, объединение Compose и полный `all`, а также отказ при
неверном имени и очистку после ошибки генерации.

## Docker

Dockerfile только копирует готовый бинарник. Сборка выполняется локально:

```sh
mise run //modules/billing:build
docker build -t distributed-programming-billing:local modules/billing
```

`build` создаёт Linux-бинарник в `dist/billing`, с отключённым CGO.
`GOARCH` позволяет выбрать архитектуру вручную; `local-up` выбирает архитектуру
Docker-сервера. Образ запускает `service` по умолчанию; для consumer передаётся
команда `message-handler`.
