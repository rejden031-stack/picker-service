# syntax=docker/dockerfile:1

# --- сборка ---
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# --- рантайм: нет компилятора, нет пакетного менеджера, не-root юзер ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl \
    && addgroup -S app \
    && adduser -S -G app app

COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/migrate /usr/local/bin/migrate
COPY --from=build /src/migrations /migrations

USER app
EXPOSE 8000

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -fsS http://localhost:8000/readyz || exit 1

CMD ["/usr/local/bin/server"]