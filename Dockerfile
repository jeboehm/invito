FROM --platform=$BUILDPLATFORM golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS builder
ARG TARGETOS
ARG TARGETARCH
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
COPY go-webdav ./go-webdav
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o invito ./cmd/invito

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/invito /invito
EXPOSE 8080

ENV INVITO_LISTEN_ADDR=:8080
ENV INVITO_DB_PATH=/data/invito.db

VOLUME ["/data"]
ENTRYPOINT ["/invito"]
