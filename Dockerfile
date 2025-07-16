# syntax=docker/dockerfile:1
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o uploader main.go

FROM alpine:latest
WORKDIR /app

COPY --from=builder /app/uploader .
COPY static/ ./static/

RUN mkdir -p /app/upload /app/merged

EXPOSE 9876

CMD ["./uploader"]
