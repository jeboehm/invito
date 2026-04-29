FROM golang:1.26-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o invito ./cmd/invito

FROM scratch
COPY --from=builder /app/invito /invito
EXPOSE 8080

ENV INVITO_LISTEN_ADDR=:8080
ENV INVITO_DB_PATH=/data/invito.db

VOLUME ["/data"]
ENTRYPOINT ["/invito"]
