# Configuration

Environment variables; every one has a default. Personal settings (how new capsules start, the world of the day, notifications, connections, AI) live in the UI per person — the `OPENAI_*`/`NOTIFY_*` values below are copied once into the first admin's settings.

| Variable | Default | What it does |
|---|---|---|
| `BASE_URL` | – | Where the app is (links in pushes, events and other apps, the OIDC callback) |
| `HOST` / `PORT` | `0.0.0.0` / `8080` | Where it listens |
| `DATA_DIR` | `./data` | The database (pictures, recordings and files are inside it) and the Pocket Core token |
| `DB_PATH` | `$DATA_DIR/pocket-capsule.db` | The SQLite file |
| `AUTH_MODE` | `local` | `local` (passwords), `proxy` (a header from your reverse proxy), `none` (one user, for trying it out) |
| `SECURE_COOKIES` | `false` | Set `true` behind HTTPS |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` / `ADMIN_NAME` | – | The first account, made on an empty database |
| `OIDC_ISSUER` / `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | – | Sign in with an OpenID Connect provider (not needed with Pocket Core) |
| `NOTIFY_URL` / `NOTIFY_FORMAT` | – / `apprise` | Pushes: an Apprise URL, an ntfy topic or a JSON webhook |
| `NOTIFY_HOUR` | `9` | Default hour of reminders ("opens next week") and yearly nudges |
| `OPENAI_API_KEY` / `OPENAI_MODEL` / `OPENAI_BASE_URL` | – | AI without Pocket Core |
| `CORE_URL` / `CORE_REGISTRATION_KEY` / `CORE_APP_URL` | – | Pocket Core |
| `TRUST_PROXY` | `false` | Take the client address from `X-Forwarded-For` (the rate limit of the "add something" link is per address) |
| `METRICS_ENABLED` | `false` | Prometheus `/metrics` |
| `MAX_BODY_BYTES` | `67108864` | Largest request; one picture, recording or file can be up to 25 MB |
| `DEMO_DATA` | `false` | Seed seven capsules in every state — one ready to open today, one opening in ten days, a child's capsule sealed until her 18th birthday with letters through the slot, a family capsule being filled, and three opened ones with photos, a recording, guesses marked and a letter back — plus a second person and two yearly capsules (no network calls) |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `text` | |

Connections to Pocket Calendar, Contacts, Family, Email, Chat, Photos, Notes, Files, Voice, Tasks, Alerts, Blog, Weather, Bookshelf, Music, Movement, Health and Wealth are made per person under Settings → Connected apps (Pocket Core, or a URL and token).

