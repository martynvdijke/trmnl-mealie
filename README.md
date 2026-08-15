# trmnl-mealie

A [TRMNL](https://usetrmnl.com) plugin and small Go backend that shows a random
recipe from your [Mealie](https://mealie.app) instance on your e-ink device —
"recipe of the day".

## How it works

- The Go backend authenticates to Mealie with an API key (`Authorization: Bearer`).
- `GET /api/trmnl/recipe-of-the-day` picks a random recipe (random page + random
  item + detail by slug) and returns a JSON payload shaped for TRMNL Liquid.
- `GET /api/trmnl/recipe-image?id=...&file=...` proxies Mealie recipe images so
  the TRMNL device never needs credentials.
- The `trmnl/` directory is the TRMNL plugin project (four layouts + settings).

```
TRMNL device ── polls ──> your trmnl-mealie backend ──> Mealie API (api key)
                              ▲
                    image requests proxied here
```

## Running

Requires two environment variables:

| Variable         | Description                                              |
| ---------------- | -------------------------------------------------------- |
| `MEALIE_URL`     | Base URL of your Mealie instance, e.g. `https://mealie.example.com` |
| `MEALIE_API_KEY` | API key created in Mealie: User Settings -> API Tokens    |
| `PORT`           | Listen port (default `8080`)                              |

```sh
export MEALIE_URL=https://mealie.example.com
export MEALIE_API_KEY=your-api-key
go run .
```

Or with Docker:

```sh
docker compose up -d   # edit docker-compose.yaml first
```

## TRMNL plugin setup

1. Create the plugin in the TRMNL dashboard and point its polling URL at
   `https://<your-backend>/api/trmnl/recipe-of-the-day`.
2. Set the `url` custom field to `https://<your-backend>`.
3. The device refreshes daily (refresh_interval 1440).

Local dev: `trmnlp serve` from inside `trmnl/` (see
[trmnlp](https://github.com/usetrmnl/trmnlp)).

## Development

```sh
task setup     # go mod download
task dev       # hot reload via air
task test      # go test ./...
task build     # go build
```

## License

See [LICENSE](LICENSE).
