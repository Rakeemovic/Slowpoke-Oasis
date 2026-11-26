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

### Creating the accounts

With `compose.https.yaml`, registration is closed, so strangers cannot create accounts on your server. Open it, register everyone, then close it again:

```sh
REGISTRATION_OPEN=true docker compose up -d
# register the accounts in the web app, then:
docker compose up -d
```

This assumes the `.env` file above, without a `REGISTRATION_OPEN` line.

## Behind your own reverse proxy

Set `REGISTRATION_OPEN=false` once all accounts exist.

If you already run nginx, Traefik or similar with HTTPS, use only `compose.yaml` and forward all requests for your domain to port 8080. Keep `COOKIE_SECURE` at its default (`true`). Do not publish port 8080 to the internet directly; change the port line in `compose.yaml` to `"127.0.0.1:8080:8080"` if the proxy runs on the same machine.

## Settings

All settings are environment variables. In Compose, set them in `.env` or in the `environment` section of `compose.yaml`.

| Variable | Default (image) | Meaning |
|---|---|---|
| `COOKIE_SECURE` | `true` | Send the login cookie only over HTTPS. `false` only for a home network without HTTPS. Any other value stops the server. |
| `REGISTRATION_OPEN` | `true`; `false` with `compose.https.yaml` | Whether new accounts can be registered. Existing accounts are not affected. Any value other than a boolean stops the server. |
| `CORS_ORIGINS` | empty | Comma-separated exact origins allowed to call the API from a browser. Only needed when a web app is hosted at another address. |
| `DB_PATH` | `/data/slowpoke.db` | The SQLite database. `/data` is a Docker volume. |
| `WEB_DIR` | `/web` | The exported web app. Empty means the server only serves the API. |
| `BACKUP_DIR` | `/data/backups` | Where the daily backups go. Empty turns them off. |
| `ADDR` | `:8080` | Address and port the server listens on inside the container. |

## Updating

```sh
git pull
docker compose up -d --build
```

Database migrations run automatically at startup. Write a backup first (see below).

## Backups

The whole state is one database. The server copies it to `/data/backups` once a day (UTC), while it keeps running, and keeps the newest 14 copies. To write the copy of today right now, for example before an update:

```sh
docker compose exec slowpoke /server backup
```

### Keep a copy elsewhere

The backups sit on the same disk as the database, so they do not help if the machine is lost. Fetch them regularly to another computer. They contain everything, so store them somewhere only you can read.

On your own computer, with `me@server` and the folder of the project adjusted:

```sh
ssh me@server "cd Slowpoke-Oasis && docker compose cp slowpoke:/data/backups/. ~/slowpoke-backups"
scp -r me@server:slowpoke-backups .
```

### Restore

Stop the server, put the chosen copy in place of the database and start it again. The helper container needs the name of the volume, `<project>_slowpoke-data` (see `docker volume ls`; the project is the folder name, e.g. `slowpoke-oasis`):

```sh
docker compose stop slowpoke
docker run --rm -v slowpoke-oasis_slowpoke-data:/data alpine sh -c '
  cp /data/backups/slowpoke-2026-12-01.db /data/slowpoke.db &&
  rm -f /data/slowpoke.db-wal /data/slowpoke.db-shm /data/slowpoke.db-journal &&
  chown 65532:65532 /data/slowpoke.db'
docker compose start slowpoke
```

To restore a copy from another computer, copy it to the server first and mount its folder into the helper container: add `-v /home/me/slowpoke-backups:/import:ro` and copy from `/import/`.

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

The Android app talks to the API directly and does not need the cookie. Its server address is set when the app is built.

To build an installable APK with [EAS Build](https://docs.expo.dev/build-reference/apk/), Expo's cloud service, you need an Expo account. The source is uploaded to Expo for the build. If you publish your own version, change `android.package` in `frontend/app.json` first. In `frontend/`:

```sh
npx eas-cli@latest login
npx eas-cli@latest init
npx eas-cli@latest env:set --name EXPO_PUBLIC_API_URL --value https://money.example.org/api --environment preview --visibility plaintext
npx eas-cli@latest build --platform android --profile preview
```

`init` links the folder to your Expo project and is needed once. The build ends with a link to the APK; open it on the phone to install.
