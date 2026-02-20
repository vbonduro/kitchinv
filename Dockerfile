FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /kitchinv ./cmd/kitchinv

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
RUN mkdir -p /data/photos

COPY --from=builder /kitchinv /app/kitchinv

EXPOSE 8080
ENTRYPOINT ["/app/kitchinv"]
