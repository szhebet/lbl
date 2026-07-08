# ============================================================
# Stage 1: Build the Go backend application
# ============================================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY src/ ./src/

RUN go build -o main ./src

# ============================================================
# Stage 2: Production runtime image
# ============================================================
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app/

COPY --from=builder /app/main .

COPY static/ ./static/
COPY templates/ ./templates/
COPY certres/ ./certres/

EXPOSE 9091 443

CMD ["./main"]
