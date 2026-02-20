#!/usr/bin/env python3
import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request


def read_env_file(path: str) -> dict:
    out = {}
    if not os.path.isfile(path):
        return out
    with open(path, "r", encoding="utf-8", errors="replace") as f:
        for line in f:
            text = line.strip()
            if not text or text.startswith("#") or "=" not in text:
                continue
            key, value = text.split("=", 1)
            out[key.strip()] = value.strip()
    return out


def request_json(
    base: str, method: str, path: str, token: str = "", payload=None, timeout: int = 20
):
    url = base + path
    headers = {"Accept": "application/json"}
    body = None
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if payload is not None:
        headers["Content-Type"] = "application/json"
        body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        url=url, data=body, method=method.upper(), headers=headers
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            text = raw.decode("utf-8", errors="replace")
            parsed = None
            if text:
                try:
                    parsed = json.loads(text)
                except json.JSONDecodeError:
                    parsed = None
            return resp.status, dict(resp.headers), raw, parsed
    except urllib.error.HTTPError as e:
        raw = e.read()
        text = raw.decode("utf-8", errors="replace")
        parsed = None
        if text:
            try:
                parsed = json.loads(text)
            except json.JSONDecodeError:
                parsed = None
        return e.code, dict(e.headers), raw, parsed


def request_raw(
    base: str, method: str, path: str, token: str = "", payload=None, timeout: int = 60
):
    url = base + path
    headers = {}
    body = None
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if payload is not None:
        headers["Content-Type"] = "application/json"
        body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        url=url, data=body, method=method.upper(), headers=headers
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            return resp.status, dict(resp.headers), raw
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read()


class Runner:
    def __init__(self):
        self.pass_count = 0
        self.fail_count = 0
        self.warn_count = 0

    def ok(self, msg: str):
        self.pass_count += 1
        print(f"[PASS] {msg}")

    def fail(self, msg: str):
        self.fail_count += 1
        print(f"[FAIL] {msg}")

    def warn(self, msg: str):
        self.warn_count += 1
        print(f"[WARN] {msg}")


def main() -> int:
    parser = argparse.ArgumentParser(description="xraytool online regression checks")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", default="")
    parser.add_argument("--admin-user", default="")
    parser.add_argument("--admin-pass", default="")
    parser.add_argument("--env-file", default="/etc/default/xraytool")
    args = parser.parse_args()

    env = read_env_file(args.env_file)
    port = args.port or env.get("XTOOL_LISTEN", ":18080").lstrip(":")
    admin_user = args.admin_user or env.get("XTOOL_ADMIN_USER", "admin")
    admin_pass = args.admin_pass or env.get("XTOOL_ADMIN_PASS", "admin123456")

    base = f"http://{args.host}:{port}"
    run = Runner()

    status, _, _, health = request_json(base, "GET", "/healthz")
    if status == 200 and isinstance(health, dict) and health.get("ok") is True:
        run.ok("healthz")
    else:
        run.fail(f"healthz failed (status={status})")
        print(
            f"Summary: pass={run.pass_count} fail={run.fail_count} warn={run.warn_count}"
        )
        return 1

    status, _, _, login = request_json(
        base,
        "POST",
        "/api/auth/login",
        payload={"username": admin_user, "password": admin_pass},
    )
    if status != 200 or not isinstance(login, dict) or not login.get("token"):
        run.fail(f"login failed (status={status})")
        print(
            f"Summary: pass={run.pass_count} fail={run.fail_count} warn={run.warn_count}"
        )
        return 1
    token = str(login["token"])
    run.ok("login")

    for path in [
        "/api/auth/me",
        "/api/customers",
        "/api/host-ips",
        "/api/oversell",
        "/api/orders",
        "/api/settings",
        "/api/runtime/customers",
        "/api/db/backups",
    ]:
        status, _, _, _ = request_json(base, "GET", path, token=token)
        if status == 200:
            run.ok(path)
        else:
            run.fail(f"{path} failed (status={status})")

    status, _, _, customers = request_json(base, "GET", "/api/customers", token=token)
    customer_id = 0
    if status == 200 and isinstance(customers, list) and customers:
        customer_id = int(customers[0].get("id", 0) or 0)
    if customer_id > 0:
        status, _, _, _ = request_json(
            base,
            "GET",
            f"/api/orders/allocation/preview?customer_id={customer_id}",
            token=token,
        )
        if status == 200:
            run.ok("/api/orders/allocation/preview")
        else:
            run.fail(f"/api/orders/allocation/preview failed (status={status})")
    else:
        run.warn("skip allocation preview: no customer")

    status, _, _, orders = request_json(base, "GET", "/api/orders", token=token)
    first_order_id = 0
    if status == 200 and isinstance(orders, list) and orders:
        first_order_id = int(orders[0].get("id", 0) or 0)

    if first_order_id > 0:
        status, headers, raw = request_raw(
            base,
            "GET",
            f"/api/orders/{first_order_id}/export?count=1&shuffle=true",
            token=token,
        )
        content_disposition = headers.get("Content-Disposition", "")
        if status == 200 and raw and "filename" in content_disposition.lower():
            run.ok("order export filename + body")
        else:
            run.fail(f"order export failed (status={status})")

        status, _, raw = request_raw(
            base,
            "POST",
            f"/api/orders/{first_order_id}/test/stream",
            token=token,
            payload={"sample_percent": 5},
            timeout=90,
        )
        text = raw.decode("utf-8", errors="replace")
        if (
            status == 200
            and '"type":"meta"' in text
            and ('"type":"done"' in text or '"type":"error"' in text)
        ):
            run.ok("stream test endpoint")
        else:
            run.fail(f"stream test failed (status={status})")
    else:
        run.warn("skip order export/stream test: no orders")

    status, _, _, backup_create = request_json(
        base, "POST", "/api/db/backups", token=token, payload={}
    )
    backup_name = ""
    if status == 200 and isinstance(backup_create, dict):
        backup_name = str(backup_create.get("name", ""))
        run.ok("create backup")
    else:
        run.fail(f"create backup failed (status={status})")

    if backup_name:
        safe_name = urllib.parse.quote(backup_name, safe="")
        status, headers, raw = request_raw(
            base, "GET", f"/api/db/backups/{safe_name}/download", token=token
        )
        if (
            status == 200
            and len(raw) > 0
            and "attachment" in headers.get("Content-Disposition", "").lower()
        ):
            run.ok("download backup")
        else:
            run.fail(f"download backup failed (status={status})")

    status, _, _, settings = request_json(base, "GET", "/api/settings", token=token)
    if status == 200 and isinstance(settings, dict):
        bark_enabled = str(settings.get("bark_enabled", "")).lower() == "true"
        bark_base = str(settings.get("bark_base_url", "")).strip()
        bark_key = str(settings.get("bark_device_key", "")).strip()
        if bark_enabled and bark_base and bark_key:
            status, _, _, _ = request_json(
                base, "POST", "/api/settings/bark/test", token=token, payload={}
            )
            if status == 200:
                run.ok("bark test notify")
            else:
                run.fail(f"bark test notify failed (status={status})")
        else:
            run.warn("skip bark test notify: bark not fully configured")

    print(f"Summary: pass={run.pass_count} fail={run.fail_count} warn={run.warn_count}")
    return 1 if run.fail_count > 0 else 0


if __name__ == "__main__":
    sys.exit(main())
