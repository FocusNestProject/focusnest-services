FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.work go.work.sum ./
COPY shared-libs/go.mod shared-libs/go.sum ./shared-libs/
COPY progress-service/go.mod progress-service/go.sum ./progress-service/

# Download dependencies
RUN go work sync

# Copy source code
COPY shared-libs ./shared-libs
COPY progress-service ./progress-service

# Build the application
WORKDIR /app/progress-service
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/progress-service/server .

EXPOSE 8080

CMD ["./server"]
