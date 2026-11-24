# Builds Slowpoke Oasis as one image: the Go server, which also delivers the
# web app, so app and API share one origin (the API lives under /api).
#
#   docker compose up --build        (see docs/self-hosting.md)

# 1. The web app, exported as static files.
FROM node:24-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# Same origin as the server, so a relative address is enough. No .env files
# from a developer's machine, and no usage data sent to Expo while building.
ENV EXPO_PUBLIC_API_URL=/api EXPO_NO_DOTENV=1 EXPO_NO_TELEMETRY=1
RUN npx expo export -p web --output-dir /web

# 2. The server. The SQLite driver is pure Go, so the binary needs no C
# libraries and runs in a minimal image.
FROM golang:1.26-alpine AS server
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /server ./cmd/server \
 && mkdir /data

# 3. The final image: no shell, no package manager, not running as root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /server /server
COPY --from=web /web /web
# The database lives in a volume; the directory must belong to the user the
# server runs as, or SQLite cannot create its files there.
COPY --from=server --chown=nonroot:nonroot /data /data
ENV ADDR=:8080 DB_PATH=/data/slowpoke.db WEB_DIR=/web
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["/server"]
