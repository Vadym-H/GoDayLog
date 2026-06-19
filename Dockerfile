# -------- BUILD STAGE --------
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/app
RUN CGO_ENABLED=0 GOOS=linux go build -o api ./cmd/api


# -------- FINAL STAGE --------
FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/app .
COPY --from=builder /app/api .
COPY config ./config

CMD ["./app"]