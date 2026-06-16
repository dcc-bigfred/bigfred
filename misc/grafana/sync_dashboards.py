#!/usr/bin/env python3
"""Push Grafana dashboard JSON files to Grafana Cloud.

Reads every *.json file from misc/grafana/dashboards/ and upserts each
dashboard via the Grafana HTTP API (POST /api/dashboards/db, overwrite=true).

Environment variables
---------------------
GRAFANA_URL
    Grafana stack root URL, e.g. https://myorg.grafana.net
GRAFANA_TOKEN
    Service account token or API key (Bearer auth).
GRAFANA_FOLDER_UID
    Optional folder UID to place dashboards in.

Example
-------
    export GRAFANA_URL=https://myorg.grafana.net
    export GRAFANA_TOKEN=glsa_...
    ./misc/grafana/sync_dashboards.py

    ./misc/grafana/sync_dashboards.py --dry-run
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path


DEFAULT_DASHBOARDS_DIR = Path(__file__).resolve().parent / "dashboards"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Sync misc/grafana/dashboards/*.json to Grafana Cloud.",
    )
    parser.add_argument(
        "--dashboards-dir",
        type=Path,
        default=DEFAULT_DASHBOARDS_DIR,
        help=f"Directory with dashboard JSON exports (default: {DEFAULT_DASHBOARDS_DIR})",
    )
    parser.add_argument(
        "--url",
        default=os.environ.get("GRAFANA_URL", "").strip(),
        help="Grafana base URL (default: $GRAFANA_URL)",
    )
    parser.add_argument(
        "--token",
        default=os.environ.get("GRAFANA_TOKEN", "").strip(),
        help="Bearer token (default: $GRAFANA_TOKEN)",
    )
    parser.add_argument(
        "--folder-uid",
        default=os.environ.get("GRAFANA_FOLDER_UID", "").strip(),
        help="Target folder UID (default: $GRAFANA_FOLDER_UID, root if empty)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate files and print actions without calling the API",
    )
    return parser.parse_args()


def load_dashboard(path: Path) -> dict:
    with path.open(encoding="utf-8") as fh:
        data = json.load(fh)
    if not isinstance(data, dict):
        raise ValueError(f"{path}: root JSON value must be an object")
    return data


def prepare_payload(dashboard: dict, folder_uid: str, message: str) -> dict:
    # Grafana assigns numeric ids; exports keep id=null for portability.
    dashboard = dict(dashboard)
    dashboard["id"] = None

    payload: dict = {
        "dashboard": dashboard,
        "overwrite": True,
        "message": message,
    }
    if folder_uid:
        payload["folderUid"] = folder_uid
    return payload


def upsert_dashboard(
    *,
    url: str,
    token: str,
    payload: dict,
) -> dict:
    endpoint = url.rstrip("/") + "/api/dashboards/db"
    body = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        endpoint,
        data=body,
        method="POST",
        headers={
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json",
            "Accept": "application/json",
        },
    )
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.loads(response.read().decode("utf-8"))


def dashboard_label(dashboard: dict, path: Path) -> str:
    title = dashboard.get("title") or path.stem
    uid = dashboard.get("uid")
    if uid:
        return f"{title} (uid={uid})"
    return title


def main() -> int:
    args = parse_args()

    if not args.url and not args.dry_run:
        print("error: Grafana URL required (--url or GRAFANA_URL)", file=sys.stderr)
        return 2
    if not args.token and not args.dry_run:
        print("error: Grafana token required (--token or GRAFANA_TOKEN)", file=sys.stderr)
        return 2

    dashboards_dir = args.dashboards_dir.resolve()
    if not dashboards_dir.is_dir():
        print(f"error: dashboards directory not found: {dashboards_dir}", file=sys.stderr)
        return 2

    paths = sorted(dashboards_dir.glob("*.json"))
    if not paths:
        print(f"warning: no *.json files in {dashboards_dir}", file=sys.stderr)
        return 0

    ok = 0
    failed = 0

    for path in paths:
        label = path.name
        try:
            dashboard = load_dashboard(path)
            label = dashboard_label(dashboard, path)
            payload = prepare_payload(
                dashboard,
                args.folder_uid,
                message=f"sync_dashboards.py: {path.name}",
            )
        except (OSError, json.JSONDecodeError, ValueError) as exc:
            failed += 1
            print(f"FAIL  {path.name}: {exc}", file=sys.stderr)
            continue

        if args.dry_run:
            uid = payload["dashboard"].get("uid", "?")
            folder = args.folder_uid or "(root)"
            print(f"DRY   {label} <- {path.name} -> folder {folder}, uid={uid}")
            ok += 1
            continue

        try:
            result = upsert_dashboard(
                url=args.url,
                token=args.token,
                payload=payload,
            )
        except urllib.error.HTTPError as exc:
            failed += 1
            detail = exc.read().decode("utf-8", errors="replace")
            print(
                f"FAIL  {label}: HTTP {exc.code} {exc.reason}\n{detail}",
                file=sys.stderr,
            )
            continue
        except urllib.error.URLError as exc:
            failed += 1
            print(f"FAIL  {label}: {exc.reason}", file=sys.stderr)
            continue

        status = result.get("status", "unknown")
        version = result.get("version", "?")
        url = result.get("url", "")
        print(f"OK    {label}: {status} (version {version}) {url}")
        ok += 1

    print(f"\n{ok} succeeded, {failed} failed, {len(paths)} total")
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
