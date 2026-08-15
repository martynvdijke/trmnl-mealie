# trmnl-mealie

A [TRMNL](https://usetrmnl.com) plugin that shows a recipe of the day from
your [Mealie](https://mealie.app) instance. The recipe changes daily and is
stable throughout the day.

## How it works

No backend to host. The plugin polls Mealie directly from the TRMNL plugin
service using your API key, and a small Python transform script picks and
shapes the recipe for the Liquid templates:

```
┌──────────┐  GET /api/recipes?perPage=100   ┌─────────┐
│ TRMNL    │ ──────────────────────────────► │ Mealie  │
│ service  │   Authorization: Bearer <key>   │ server  │
│  (polls) │ ◄────────────────────────────── │         │
└──────────┘        recipe list JSON         └─────────┘
      │
      ▼
 src/transform.py   picks the day's recipe (day index % list size)
      │
      ▼
   Liquid templates render the card
```

The transform:

- picks a recipe with `day_index % len(items)` — stable within a day, rotates
  daily (Mealie's own `orderBy=random` shuffles deterministically from a seed,
  so a static URL would show the same recipe forever);
- when network access is available (local `trmnlp serve`), fetches full recipe
  details for ingredients and instructions. The hosted TRMNL sandbox has no
  network access, so there it gracefully falls back to a summary card.

The `trmnl/` directory is a [trmnlp](https://github.com/owise1/trmnlp) plugin
project (Liquid templates + settings + transform) pushed to your TRMNL plugin
via `trmnlp push`.

## TRMNL plugin setup

1. Create a new plugin in the TRMNL dashboard (or push via `trmnlp push`).
2. Set the custom fields:
   - **url** — your Mealie instance address, e.g. `https://mealie.example.com`.
   - **api_key** — a Mealie API token (User Settings > API Tokens).
3. Set the refresh interval to daily (1440 minutes) for a recipe of the day.

## Development

The transform is plain Python and needs no dependencies. Validate it against
a sample polled payload:

```sh
python3 src/transform.py < fixture.json
```

For a live preview, run `trmnlp serve` inside `trmnl/` (the transform runs in
a local subprocess and can reach your Mealie instance).

## Security notes

- The API key is stored in your TRMNL plugin settings (custom fields) and is
  sent as the `Authorization: Bearer` header on each poll.
- Transform scripts run automatically against the polled response — review
  any third-party plugin's `src/transform.*` before serving it, or set
  `transform_runtime: disabled` in `.trmnlp.yml`.

## License

See [LICENSE](LICENSE).
