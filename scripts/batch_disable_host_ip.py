#!/usr/bin/env python3
import argparse
import concurrent.futures
import json
import os
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass


DEFAULT_HOSTS = [
    "128.241.31.2",
    "192.217.227.2",
    "207.152.99.2",
    "130.94.202.2",
    "192.204.18.2",
    "82.153.114.2",
    "131.103.114.2",
    "128.242.115.2",
    "157.238.176.2",
    "198.63.246.2",
    "157.238.145.2",
]


@dataclass(frozen=True)
class Result:
    host: str
    ok: bool
    status: str
    detail: str


class PanelClient:
    def __init__(
        self,
        base_url: str,
        username: str,
        password: str,
        timeout: int,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.username = username
        self.password = password
        self.timeout = timeout
        self.token = ""

    def login(self) -> None:
        payload = {"username": self.username, "password": self.password}
        data = self.request("POST", "/api/auth/login", payload, auth=False)
        token = str(data.get("token", "")).strip()
        if not token:
            raise RuntimeError("login response did not include token")
        self.token = token

    def scan_host_ips(self) -> list[dict]:
        data = self.request("POST", "/api/host-ips/scan", {})
        if not isinstance(data, list):
            raise RuntimeError("host ip scan response is not a list")
        return data

    def list_host_ips(self) -> list[dict]:
        data = self.request("GET", "/api/host-ips")
        if not isinstance(data, list):
            raise RuntimeError("host ip list response is not a list")
        return data

    def toggle_host_ip(self, host_ip_id: int, enabled: bool) -> None:
        data = self.request(
            "POST",
            f"/api/host-ips/{host_ip_id}/toggle",
            {"enabled": enabled},
        )
        if data.get("ok") is not True:
            raise RuntimeError("toggle response did not confirm ok=true")

    def request(
        self,
        method: str,
        path: str,
        payload: dict | None = None,
        auth: bool = True,
    ):
        body = None
        headers = {"Accept": "application/json"}
        if payload is not None:
            body = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if auth:
            headers["Authorization"] = f"Bearer {self.token}"

        req = urllib.request.Request(
            self.base_url + path,
            data=body,
            headers=headers,
            method=method,
        )
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as response:
                raw = response.read().decode("utf-8")
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace").strip()
            raise RuntimeError(f"HTTP {exc.code}: {detail}") from exc
        except urllib.error.URLError as exc:
            raise RuntimeError(str(exc.reason)) from exc

        if not raw:
            return {}
        try:
            return json.loads(raw)
        except json.JSONDecodeError as exc:
            raise RuntimeError(f"invalid JSON response: {raw[:160]}") from exc


def parse_hosts(value: str) -> list[str]:
    hosts = [item.strip() for item in value.replace("\n", ",").split(",")]
    return [item for item in hosts if item]


def disable_one(args, host: str) -> Result:
    base_url = f"{args.scheme}://{host}:{args.panel_port}"
    client = PanelClient(base_url, args.username, args.password, args.timeout)
    try:
        client.login()
        rows = client.scan_host_ips() if args.scan else client.list_host_ips()
        target_rows = [row for row in rows if row.get("ip") == args.target_ip]
        if not target_rows and args.scan:
            target_rows = [
                row for row in client.list_host_ips() if row.get("ip") == args.target_ip
            ]
        if not target_rows:
            return Result(host, True, "not_found", f"{args.target_ip} not present")

        target = target_rows[0]
        host_ip_id = int(target["id"])
        if target.get("enabled") is False:
            return Result(host, True, "already_disabled", f"id={host_ip_id}")

        if not args.execute:
            return Result(host, True, "dry_run", f"would disable id={host_ip_id}")

        client.toggle_host_ip(host_ip_id, False)
        rows_after = client.list_host_ips()
        for row in rows_after:
            if row.get("id") == host_ip_id:
                if row.get("enabled") is False:
                    return Result(host, True, "disabled", f"id={host_ip_id}")
                return Result(host, False, "verify_failed", f"id={host_ip_id} still enabled")
        return Result(host, False, "verify_failed", f"id={host_ip_id} missing after toggle")
    except Exception as exc:  # pylint: disable=broad-except
        return Result(host, False, "failed", str(exc))


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Scan XrayTool panel host IPs and disable a selected host IP."
    )
    parser.add_argument(
        "--hosts",
        default=",".join(DEFAULT_HOSTS),
        help="comma-separated panel hosts",
    )
    parser.add_argument("--scheme", default="http", choices=["http", "https"])
    parser.add_argument("--panel-port", type=int, default=36172)
    parser.add_argument("--username", default="yuxi")
    parser.add_argument("--password-env", default="XRAYTOOL_ADMIN_PASS")
    parser.add_argument("--target-ip", default="127.0.0.1")
    parser.add_argument("--timeout", type=int, default=20)
    parser.add_argument("--max-workers", type=int, default=6)
    parser.add_argument("--no-scan", dest="scan", action="store_false")
    parser.add_argument("--execute", action="store_true")
    args = parser.parse_args()

    password = os.environ.get(args.password_env, "").strip()
    if not password:
        print(f"[ERROR] missing password env: {args.password_env}", file=sys.stderr)
        return 2
    args.password = password

    hosts = parse_hosts(args.hosts)
    if not hosts:
        print("[ERROR] no hosts provided", file=sys.stderr)
        return 2

    mode = "execute" if args.execute else "dry-run"
    print(
        f"mode={mode} panels={len(hosts)} scheme={args.scheme} "
        f"port={args.panel_port} target_ip={args.target_ip} scan={args.scan}"
    )

    ok = 0
    failed = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.max_workers) as pool:
        futures = [pool.submit(disable_one, args, host) for host in hosts]
        for future in concurrent.futures.as_completed(futures):
            result = future.result()
            prefix = "OK" if result.ok else "FAIL"
            print(f"[{prefix}] {result.host} {result.status}: {result.detail}")
            if result.ok:
                ok += 1
            else:
                failed += 1

    print(f"summary ok={ok} failed={failed}")
    return 0 if failed == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
