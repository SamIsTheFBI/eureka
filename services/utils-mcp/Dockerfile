FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/utils-mcp ./cmd/utils-mcp

FROM alpine:3.20

RUN addgroup -g 10001 -S appgroup && adduser -u 10001 -S appuser -G appgroup

COPY --from=builder /bin/utils-mcp /usr/local/bin/utils-mcp

USER 10001:10001
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/utils-mcp"]
