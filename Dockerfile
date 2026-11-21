# Standalone sparring bot, statically linked Go, no network or writable disk required.
FROM golang:1.23-bookworm AS builder
WORKDIR /src
COPY go.mod *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /harrier .
FROM scratch
COPY --from=builder /harrier /harrier
USER 65534:65534
ENTRYPOINT ["/harrier"]
