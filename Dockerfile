# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Кэшируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники и собираем
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker

# Runtime stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Копируем бинарники из builder stage
COPY --from=builder /bin/api /app/api
COPY --from=builder /bin/worker /app/worker
COPY --from=builder /app/migrations /app/migrations

# Non-root user (security best practice)
RUN adduser -D -u 1000 appuser
USER appuser

EXPOSE 8080

# По умолчанию запускаем API
CMD ["/app/api"]
