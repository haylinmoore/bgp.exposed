FROM golang:1.16-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY backend/ ./backend
RUN go build -o /app/core backend/core/core.go 

## Deploy
FROM alpine

WORKDIR /app

COPY --from=builder /app/core /app/core

EXPOSE 2000
EXPOSE 8080

CMD [ "/app/core" ]