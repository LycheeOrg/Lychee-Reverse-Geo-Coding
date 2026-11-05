FROM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder
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
