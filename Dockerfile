FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /kitchinv ./cmd/kitchinv

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S kitchinv && \
    adduser -S -G kitchinv kitchinv && \
    mkdir -p /data/photos && \
    chown -R kitchinv:kitchinv /data

COPY --from=builder /kitchinv /app/kitchinv

USER kitchinv

EXPOSE 8080
ENTRYPOINT ["/app/kitchinv"]
