FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify
RUN apt-get update && apt-get install -y postgresql-client && rm -rf /var/lib/apt/lists/*

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api-server ./cmd/api-server

FROM debian:bookworm-slim

WORKDIR /app
COPY --from=builder /out/api-server /app/api-server

EXPOSE 8080

CMD ["/app/api-server"]


