# syntax=docker/dockerfile:1

FROM golang:1.22 AS builder

WORKDIR /workspace

COPY shared-libs/ shared-libs/
COPY session-service/ session-service/

WORKDIR /workspace/session-service
RUN go mod download
RUN mkdir -p /out
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=builder /out/server /app/server
EXPOSE 8080
ENTRYPOINT ["/app/server"]
