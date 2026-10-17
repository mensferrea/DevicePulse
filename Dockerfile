FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/devicepulse ./cmd/all-in-one
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/load-generator ./cmd/load-generator

FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/devicepulse /usr/local/bin/devicepulse
COPY --from=builder /bin/load-generator /usr/local/bin/load-generator

EXPOSE 8080 50051 9090

ENTRYPOINT ["devicepulse"]
