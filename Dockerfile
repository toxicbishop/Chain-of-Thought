# ---- Build stage ----
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cot-server ./main.go

# ---- Runtime stage ----
# Use alpine instead of scratch for CA certificates (needed for TLS to Kafka brokers)
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/cot-server /cot-server

EXPOSE 8080
ENTRYPOINT ["/cot-server"]
