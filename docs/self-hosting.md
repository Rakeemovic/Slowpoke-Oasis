# Self-Hosting

Slowpoke Oasis runs as a single server that delivers both the web app and the API from one address. The API lives under `/api`, everything else is the web app. Because both share one origin, the web login (an HttpOnly cookie with `SameSite=Strict`) works without any CORS setup.

The data is one SQLite file. Nothing is sent anywhere else.

## Requirements

- Docker with the Compose plugin (`docker compose version`).
- About 1 GB of free memory while building the image. The finished image is about 25 MB.
- For HTTPS on the internet: a domain that points to the machine, and ports 80 and 443 reachable from outside.

## Quick start in a home network

Without HTTPS, browsers drop the login cookie unless `COOKIE_SECURE` is turned off. Only do this in a network you trust: anyone on it can read the cookie and take over a session.

```sh
git clone https://github.com/Rakeemovic/Slowpoke-Oasis.git
cd Slowpoke-Oasis
COOKIE_SECURE=false docker compose up -d --build
```

Open `http://<address of the machine>:8080`. The server logs a warning about the insecure cookie at startup; that is expected here.

## With HTTPS (Caddy)

`compose.https.yaml` adds [Caddy](https://caddyserver.com) in front of the server. Caddy gets a certificate from Let's Encrypt for your domain and renews it by itself. The server is then only reachable through Caddy.

```sh
DOMAIN=money.example.org docker compose -f compose.yaml -f compose.https.yaml up -d --build
```

To avoid typing the variables every time, put them in a file named `.env` next to `compose.yaml`:

```sh
DOMAIN=money.example.org
COMPOSE_FILE=compose.yaml:compose.https.yaml
```

Then `docker compose up -d --build` is enough.

## Behind your own reverse proxy

If you already run nginx, Traefik or similar with HTTPS, use only `compose.yaml` and forward all requests for your domain to port 8080. Keep `COOKIE_SECURE` at its default (`true`). Do not publish port 8080 to the internet directly; change the port line in `compose.yaml` to `"127.0.0.1:8080:8080"` if the proxy runs on the same machine.

## Settings

All settings are environment variables. In Compose, set them in `.env` or in the `environment` section of `compose.yaml`.

| Variable | Default (image) | Meaning |
|---|---|---|
| `COOKIE_SECURE` | `true` | Send the login cookie only over HTTPS. `false` only for a home network without HTTPS. Any other value stops the server. |
| `CORS_ORIGINS` | empty | Comma-separated exact origins allowed to call the API from a browser. Only needed when a web app is hosted at another address. |
| `DB_PATH` | `/data/slowpoke.db` | The SQLite database. `/data` is a Docker volume. |
| `WEB_DIR` | `/web` | The exported web app. Empty means the server only serves the API. |
| `ADDR` | `:8080` | Address and port the server listens on inside the container. |

## Updating

```sh
git pull
docker compose up -d --build
```

Database migrations run automatically at startup. Make a backup first.

## Backups

The whole state is the database in the volume `<project>_slowpoke-data` (see `docker volume ls`; the project is the folder name, e.g. `slowpoke-oasis`). Stop the server while copying, so the database file is consistent:

```sh
docker compose stop slowpoke
docker run --rm -v slowpoke-oasis_slowpoke-data:/data -v "$PWD":/backup alpine \
  tar czf /backup/slowpoke-backup.tar.gz -C /data .
docker compose start slowpoke
```

To restore, stop the server, unpack the archive into the volume the same way (`tar xzf … -C /data`) and start it again.

## Forgotten passwords

Users can reset their password themselves with their recovery code. If a user has lost the code too, the operator can set a new password:

```sh
docker compose exec slowpoke /server reset-password <username>
```

It prints a new password and a new recovery code and ends all logins of that user.

## Without Docker

The server is a single Go binary. Build the web app and the server, then point `WEB_DIR` at the export:

```sh
cd frontend
npm ci
EXPO_PUBLIC_API_URL=/api npx expo export -p web --output-dir ../web
cd ../backend
CGO_ENABLED=0 go build -o ../slowpoke ./cmd/server
cd ..
WEB_DIR=./web DB_PATH=./slowpoke.db ./slowpoke
```

## The Android app

The Android app talks to the API directly and does not need the cookie. Its server address is set when the app is built: `EXPO_PUBLIC_API_URL=https://money.example.org/api`.
