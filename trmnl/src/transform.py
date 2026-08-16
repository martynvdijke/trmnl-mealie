"""Recipe of the Day transform for trmnl-mealie.

The polled payload is Mealie's recipe list (pagination object) plus the
`trmnl` namespace. This transform picks a stable-per-day recipe and, when
network access is available, enriches it with full details (ingredients and
instructions) from Mealie.

Local runs (trmnlp subprocess) have network access, so the detail fetch
works. The hosted TRMNL sandbox has no network access, so the detail fetch is
skipped there and the card falls back to the list summary.
"""

import datetime
import json
import sys
import urllib.parse
import urllib.request


def _esc(s):
    return urllib.parse.quote(s, safe="")


def _text(value):
    if value is None:
        return ""
    if isinstance(value, str):
        return value
    return str(value)


def _format_ingredient(ing):
    """Render an ingredient as 'qty unit food — note', skipping empties."""
    qty = _text(ing.get("quantity", "")).strip()
    unit = (ing.get("unit") or {}).get("name", "")
    food = (ing.get("food") or {}).get("name", "")
    note = _text(ing.get("note", "")).strip()
    display = _text(ing.get("display", "")).strip()
    if display:
        return display
    parts = [p for p in (qty, unit, food) if p]
    text = " ".join(parts).strip()
    if note:
        text = f"{text} — {note}"
    return text


def _day_index():
    """Stable within a day, rotates daily."""
    return int(datetime.date.today().strftime("%Y%m%d"))


def _fetch_detail(url, api_key, slug):
    """Fetch full recipe details. Returns dict or None on any failure."""
    try:
        target = f"{url}/api/recipes/{_esc(slug)}"
        req = urllib.request.Request(target, headers={"Authorization": f"Bearer {api_key}"})
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except Exception:
        return None


def _summary_card(recipe, url, detail):
    image = ""
    file_name = recipe.get("image") or ""
    if file_name:
        image = f"{url}/api/media/recipes/{recipe.get('id', '')}/images/{file_name}"

    card = {
        "name": recipe.get("name", ""),
        "slug": recipe.get("slug", ""),
        "description": recipe.get("description", "") or "",
        "image": image,
        "servings": recipe.get("recipeServings"),
        "yield": recipe.get("recipeYield", ""),
        "prep_time": recipe.get("prepTime", ""),
        "cook_time": recipe.get("cookTime", ""),
        "total_time": recipe.get("totalTime", ""),
        "rating": recipe.get("rating"),
        "categories": [c.get("name", "") for c in (recipe.get("recipeCategory") or [])],
        "tags": [t.get("name", "") for t in (recipe.get("tags") or [])],
        "url": f"{url}/recipe/{recipe.get('slug', '')}",
    }

    if detail:
        card["ingredients"] = [
            _format_ingredient(i) for i in (detail.get("recipeIngredient") or [])
        ]
        card["instructions"] = [
            _text(i.get("text", "")).strip()
            for i in (detail.get("recipeInstructions") or [])
            if _text(i.get("text", "")).strip()
        ]
    else:
        card["ingredients"] = []
        card["instructions"] = []
    return card


def run(input_data):
    url = ""
    api_key = ""
    try:
        url = input_data["trmnl"]["plugin_settings"]["custom_fields_values"]["url"]
        api_key = input_data["trmnl"]["plugin_settings"]["custom_fields_values"]["api_key"]
    except (KeyError, TypeError):
        pass
    if not url:
        return {"error": "Set the url custom field to your Mealie instance address."}

    items = (input_data.get("items") or []) if isinstance(input_data, dict) else []
    if not items:
        return {
            "error": "No recipes found in this Mealie instance. Check the url and api_key custom fields."
        }

    idx = _day_index() % len(items)
    recipe = items[idx]
    detail = _fetch_detail(url, api_key, recipe.get("slug", "")) if api_key else None
    return _summary_card(recipe, url, detail)


if __name__ == "__main__":
    raw = sys.stdin.read()
    try:
        payload = json.loads(raw) if raw.strip() else {}
    except json.JSONDecodeError:
        payload = {}
    print(json.dumps(run(payload)))
