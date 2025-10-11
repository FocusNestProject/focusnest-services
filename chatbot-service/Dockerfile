FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.work go.work.sum ./
COPY shared-libs/go.mod shared-libs/go.sum ./shared-libs/
COPY chatbot-service/go.mod chatbot-service/go.sum ./chatbot-service/

# Download dependencies
RUN go work sync

# Copy source code
COPY shared-libs ./shared-libs
COPY chatbot-service ./chatbot-service

# Build the application
WORKDIR /app/chatbot-service
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/chatbot-service/server .

EXPOSE 8080

CMD ["./server"]
