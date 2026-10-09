FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/geo-decoding-server .

FROM scratch

COPY --from=builder /out/geo-decoding-server /geo-decoding-server

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD ["/geo-decoding-server", "-healthcheck"]

ENTRYPOINT ["/geo-decoding-server"]
