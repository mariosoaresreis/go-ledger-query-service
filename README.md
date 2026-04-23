# go-ledger-query-service (Query Service)

Read side of the CQRS ledger. This service consumes `ledger.events` from Kafka and maintains denormalized PostgreSQL read models for balances, transactions, and statements.

## Features

- `GET /api/v1/accounts/{accountId}/balance`
- `GET /api/v1/accounts/{accountId}/transactions?page=&size=&from=&to=&direction=`
- `GET /api/v1/accounts/{accountId}/statement?month=YYYY-MM`
- `GET /api/v1/accounts?ownerId={ownerId}`
- Swagger UI at `GET /swagger/index.html`

## Project layout

- `config/` runtime config loading
- `internal/kafka/` consumer + projection processor
- `internal/repository/` read-model persistence
- `internal/services/` query services
- `internal/api/` HTTP handlers and routing
- `internal/migrations/` read-model schema
- `terraform/` GCP Terraform copied/adapted from `/home/bat/projects/java/ledger/terraform`

## Environment variables

- `PORT` (default `8081`)
- `ENVIRONMENT` (default `local`)
- `LEDGER_QUERY_DB_HOST`, `LEDGER_QUERY_DB_PORT`, `LEDGER_QUERY_DB_USERNAME`, `LEDGER_QUERY_DB_PASSWORD`, `LEDGER_QUERY_DB_NAME`, `LEDGER_QUERY_DB_SSL_MODE`
- `LEDGER_KAFKA_BOOTSTRAP_SERVERS` (default `localhost:9092`)
- `LEDGER_KAFKA_TOPIC` (default `ledger.events`)
- `LEDGER_KAFKA_GROUP_ID` (default `ledger-query-service`)

## Run locally

```bash
cd /home/bat/projects/go/go-ledger-query-service
go mod tidy
go test ./...
go run .
```

The repository includes a projection integration test in `internal/kafka/processor_integration_test.go` that verifies command-topic events become read-model projections.

## Swagger docs

A minimal docs package is committed in `docs/` so the service builds even before generation.

To regenerate OpenAPI docs from annotations:

```bash
cd /home/bat/projects/go/go-ledger-query-service
go install github.com/swaggo/swag/cmd/swag@latest
go generate ./...
```

## Quick API smoke test

```bash
curl -s http://localhost:8081/api/ping
curl -s http://localhost:8081/swagger/index.html | head -n 5
```

## Full local stack

The shared `docker-compose.yml` is in `/home/bat/projects/go/go-ledger` and starts both services, Postgres, and Kafka-compatible Redpanda.

```bash
cd /home/bat/projects/go/go-ledger
docker compose up --build -d
```

## Terraform (GCP)

Query-service Terraform environment is at:

- `terraform/environments/query-dev`

Example:

```bash
cd /home/bat/projects/go/go-ledger-query-service/terraform/environments/query-dev
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform plan
```
