# Dockerfile：多阶段构建
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=1 go build -o cloudquant ./main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/cloudquant /app/cloudquant
COPY config.yaml /app/config.yaml
RUN mkdir -p /app/data
EXPOSE 8080
CMD ["/app/cloudquant"]
