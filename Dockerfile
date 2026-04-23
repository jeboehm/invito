FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o invito ./cmd/invito

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /src/invito /invito
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/invito"]
