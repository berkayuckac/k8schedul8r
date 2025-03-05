FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o k8schedul8r cmd/k8schedul8r/main.go

FROM alpine:3.21.3
WORKDIR /app
COPY --from=builder /app/k8schedul8r .
# Adjust the config file path as needed
COPY --from=builder /app/test-app/scaling-config.yaml /etc/k8schedul8r/config.yaml
# TODO: Make this configurable
CMD ["./k8schedul8r", "--config", "/etc/k8schedul8r/config.yaml", "--interval", "10s"] 