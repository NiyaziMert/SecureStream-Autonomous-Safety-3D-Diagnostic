FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod ./
# Bağımlılık yok, go.sum gerekmiyor
COPY . .
RUN CGO_ENABLED=0 go build -p 1 -o simulator .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/simulator .
CMD ["./simulator"]
