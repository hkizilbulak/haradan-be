FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.22 AS runner

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/api /out/worker ./

EXPOSE 8080
ENV HTTP_ADDR=:8080

# Railway's API service uses this default. Its worker service uses the same
# image with the start command overridden to `./worker`.
CMD ["./api"]
