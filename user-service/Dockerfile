# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy module manifests referenced by go.work
COPY go.work go.work.sum ./
COPY shared-libs/go.mod shared-libs/go.sum ./shared-libs/
COPY focus-service/go.mod focus-service/go.sum ./focus-service/
COPY gateway-api/go.mod gateway-api/go.sum ./gateway-api/
COPY chatbot-service/go.mod chatbot-service/go.sum ./chatbot-service/
COPY progress-service/go.mod progress-service/go.sum ./progress-service/
COPY user-service/go.mod user-service/go.sum ./user-service/

RUN go work sync

# Bring in source code for shared libs + user service
COPY shared-libs ./shared-libs
COPY user-service ./user-service

WORKDIR /app/user-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/user-service/server .

EXPOSE 8080
CMD ["./server"]
