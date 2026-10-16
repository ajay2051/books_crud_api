# ---- build stage ----
FROM golang:1.23-alpine AS build
WORKDIR /src

# Copy go.mod/go.sum first so Docker caches the dependency download layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO off => a static binary. Migrations are embedded, so this one file is the whole app.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/api ./cmd/api

# ---- runtime stage ----
FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["api"]