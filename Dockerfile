# -------- BUILD STAGE --------
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/app


# -------- FINAL STAGE --------
FROM alpine:3.19

WORKDIR /app

# copy binary
COPY --from=builder /app/app .

# copy config (если нужен yaml)
COPY config ./config

CMD ["./app"]