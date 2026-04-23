FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ledger-query-service ./main.go

FROM alpine:3.20
RUN adduser -D -H appuser
USER appuser
WORKDIR /app
COPY --from=builder /out/ledger-query-service /app/ledger-query-service
EXPOSE 8081
ENTRYPOINT ["/app/ledger-query-service"]

