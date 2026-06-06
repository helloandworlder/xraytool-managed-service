#!/usr/bin/env python3
"""Find and pause overloaded XrayTool-managed SOCKS5 accounts.

The inventory contains credentials and should live outside the repository.
Use scan first; pause only changes orders when explicitly requested.
"""

from __future__ import annotations

import argparse
import collections
import dataclasses
import json
import os
import re
import subprocess
import sys
import textwrap
from typing import Any


REMOTE_COLLECT_SCRIPT = r'''
import argparse
import glob
import json
import os
import sqlite3
import subprocess
import time


def read_text(path):
    try:
        with open(path, "r", encoding="utf-8") as handle:
            return handle.read()
    except Exception:
        return ""


def api_server_from_config(config):
    for inbound in config.get("inbounds", []):
        if inbound.get("tag") == "api-in" or inbound.get("protocol") == "dokodemo-door":
            listen = str(inbound.get("listen") or "127.0.0.1")
            port = inbound.get("port")
            if listen and port:
                return f"{listen}:{port}"
    return "127.0.0.1:8080"


def instance_dirs():
    seen = set()
    for path in sorted(glob.glob("/opt/xraytool*")):
        cfg = os.path.join(path, "data", "xray", "config.json")
        binary = os.path.join(path, "data", "xray", "xray")
        db_path = os.path.join(path, "data", "xraytool.db")
        if os.path.isfile(cfg) and os.path.isfile(binary) and os.path.isfile(db_path):
            real = os.path.realpath(path)
            if real not in seen:
                seen.add(real)
                yield path


def query_stats(xray, server):
    result = subprocess.run(
        [xray, "api", "statsquery", f"--server={server}", "-timeout", "5", "-pattern", "user>>>"],
        text=True,
        capture_output=True,
        timeout=8,
    )
    return {"returncode": result.returncode, "stdout": result.stdout, "stderr": result.stderr}


def load_order_items(db_path):
    out = []
    try:
        con = sqlite3.connect(db_path)
        con.row_factory = sqlite3.Row
        cur = con.cursor()
        rows = cur.execute(
            """
            select o.id order_id, o.order_no, o.status order_status,
                   c.name customer_name,
                   oi.id item_id, oi.ip, oi.port, oi.username, oi.status item_status
            from order_items oi
            join orders o on o.id = oi.order_id
            left join customers c on c.id = o.customer_id
            where oi.username <> ''
            """
        ).fetchall()
        out = [dict(row) for row in rows]
        con.close()
    except Exception as exc:
        out = [{"error": str(exc)}]
    return out


parser = argparse.ArgumentParser()
parser.add_argument("--sample-seconds", type=float, default=10.0)
args = parser.parse_args()

ss = subprocess.run(["ss", "-tan"], text=True, capture_output=True, timeout=8)
instances = []
for path in instance_dirs():
    service = os.path.basename(path.rstrip("/")) or "xraytool"
    cfg_path = os.path.join(path, "data", "xray", "config.json")
    xray = os.path.join(path, "data", "xray", "xray")
    db_path = os.path.join(path, "data", "xraytool.db")
    config_text = read_text(cfg_path)
    try:
        config = json.loads(config_text)
    except Exception:
        config = {}
    api_server = api_server_from_config(config)
    first = query_stats(xray, api_server)
    time.sleep(max(0, args.sample_seconds))
    second = query_stats(xray, api_server)
    instances.append(
        {
            "service": service,
            "path": path,
            "db_path": db_path,
            "api_server": api_server,
            "config_json": config_text,
            "stats_first": first,
            "stats_second": second,
            "order_items": load_order_items(db_path),
        }
    )

print(
    json.dumps(
        {
            "ss_stdout": ss.stdout,
            "ss_stderr": ss.stderr,
            "instances": instances,
        },
        ensure_ascii=False,
    )
)
'''


@dataclasses.dataclass(frozen=True)
class Node:
    name: str
    host: str
    user: str
    password: str
    port: int = 22


@dataclasses.dataclass(frozen=True)
class Target:
    ip: str
    port: int
    username: str | None = None


@dataclasses.dataclass(frozen=True)
class Thresholds:
    estab: int
    fin: int
    syn: int = 100
    speed_bps: float = 5 * 1024 * 1024


@dataclasses.dataclass(frozen=True)
class UserStats:
    uplink: int = 0
    downlink: int = 0

    @property
    def total(self) -> int:
        return self.uplink + self.downlink


@dataclasses.dataclass(frozen=True)
class UserRate:
    up_bps: float = 0
    down_bps: float = 0

    @property
    def total_bps(self) -> float:
        return self.up_bps + self.down_bps


@dataclasses.dataclass(frozen=True)
class OrderItem:
    order_id: int
    order_no: str
    item_id: int
    ip: str
    port: int
    username: str
    order_status: str
    item_status: str
    customer: str


@dataclasses.dataclass(frozen=True)
class InstanceSnapshot:
    node: str
    service: str
    db_path: str
    api_server: str
    endpoint_users: dict[tuple[str, int], list[str]]
    connection_counts: dict[tuple[str, int], dict[str, int]]
    user_stats: dict[str, UserStats]
    user_rates: dict[str, UserRate]
    order_items: dict[str, OrderItem]


@dataclasses.dataclass(frozen=True)
class Hotspot:
    node: str
    service: str
    db_path: str
    ip: str
    port: int
    username: str
    order_id: int | None
    order_no: str
    customer: str
    estab: int
    fin: int
    syn: int
    time_wait: int
    total_bytes: int
    speed_bps: float
    reason: str
    pause_allowed: bool = True


def parse_endpoint(value: str) -> tuple[str, int] | None:
    local = value.strip()
    if local.startswith("[") and "]:" in local:
        host, port_text = local.rsplit(":", 1)
        host = host.strip("[]")
    elif ":" in local:
        host, port_text = local.rsplit(":", 1)
    else:
        return None
    if port_text == "*":
        return None
    try:
        return host, int(port_text)
    except ValueError:
        return None


def parse_ss_counts(raw: str) -> dict[tuple[str, int], dict[str, int]]:
    counts: dict[tuple[str, int], dict[str, int]] = collections.defaultdict(lambda: collections.defaultdict(int))
    for line in raw.splitlines():
        parts = line.split()
        if len(parts) < 4 or parts[0].lower() == "state":
            continue
        endpoint = parse_endpoint(parts[3])
        if endpoint is None:
            continue
        counts[endpoint][parts[0]] += 1
    return {key: dict(value) for key, value in counts.items()}


def parse_xray_config(raw: str) -> dict[tuple[str, int], list[str]]:
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    mapping: dict[tuple[str, int], list[str]] = {}
    for inbound in data.get("inbounds", []):
        listen = str(inbound.get("listen") or "")
        port = inbound.get("port")
        accounts = (inbound.get("settings") or {}).get("accounts") or []
        if not listen or not isinstance(port, int) or not accounts:
            continue
        users = [str(account.get("user")) for account in accounts if isinstance(account, dict) and account.get("user")]
        if users:
            mapping[(listen, port)] = users
    return mapping


def parse_xray_stats(raw: str) -> dict[str, UserStats]:
    pairs: dict[str, dict[str, int]] = collections.defaultdict(lambda: {"uplink": 0, "downlink": 0})
    stats_rows: list[dict[str, Any]] = []
    try:
        payload = json.loads(raw)
        rows = payload.get("stat") or payload.get("stats") or []
        if isinstance(rows, dict):
            rows = [rows]
        stats_rows = [row for row in rows if isinstance(row, dict)]
    except json.JSONDecodeError:
        stats_rows = []

    if not stats_rows:
        for block in re.findall(r"stat\s*:\s*<([^>]+)>", raw, flags=re.S):
            name = re.search(r'name\s*:\s*"([^"]+)"', block)
            value = re.search(r"value\s*:\s*(\d+)", block)
            if name:
                stats_rows.append({"name": name.group(1), "value": int(value.group(1)) if value else 0})

    for row in stats_rows:
        name = str(row.get("name") or "")
        parts = name.split(">>>")
        if len(parts) != 4 or parts[0] != "user" or parts[2] != "traffic":
            continue
        value = int(row.get("value") or 0)
        if parts[3] in ("uplink", "downlink"):
            pairs[parts[1]][parts[3]] += value
    return {user: UserStats(values["uplink"], values["downlink"]) for user, values in pairs.items()}


def parse_target(raw: str) -> Target:
    parts = raw.strip().split(":")
    if len(parts) not in (2, 3):
        raise argparse.ArgumentTypeError("target must be ip:port or ip:port:username")
    try:
        port = int(parts[1])
    except ValueError as exc:
        raise argparse.ArgumentTypeError("target port must be a number") from exc
    return Target(ip=parts[0], port=port, username=parts[2] if len(parts) == 3 else None)


def load_inventory(path: str) -> list[Node]:
    with open(path, "r", encoding="utf-8") as handle:
        payload = json.load(handle)
    nodes = payload.get("nodes") if isinstance(payload, dict) else payload
    if not isinstance(nodes, list):
        raise ValueError("inventory must be a list or an object with nodes")
    out = []
    for item in nodes:
        if not isinstance(item, dict):
            raise ValueError("inventory node must be an object")
        host = str(item["host"])
        out.append(
            Node(
                name=str(item.get("name") or host),
                host=host,
                user=str(item.get("user") or "root"),
                password=str(item["password"]),
                port=int(item.get("port") or 22),
            )
        )
    return out


def ssh_command(node: Node, remote_command: str) -> list[str]:
    return [
        "sshpass",
        "-p",
        node.password,
        "ssh",
        "-o",
        "StrictHostKeyChecking=no",
        "-o",
        "UserKnownHostsFile=/dev/null",
        "-o",
        "ConnectTimeout=15",
        "-o",
        "ServerAliveInterval=5",
        "-o",
        "ServerAliveCountMax=3",
        "-p",
        str(node.port),
        f"{node.user}@{node.host}",
        remote_command,
    ]


def run_ssh_script(node: Node, script: str, remote_command: str, timeout: int = 90) -> str:
    result = subprocess.run(
        ssh_command(node, remote_command),
        input=script,
        text=True,
        capture_output=True,
        timeout=timeout,
    )
    if result.returncode != 0:
        raise RuntimeError((result.stderr or result.stdout).strip())
    return result.stdout


def collect_node(node: Node, sample_seconds: float) -> list[InstanceSnapshot]:
    stdout = run_ssh_script(
        node,
        REMOTE_COLLECT_SCRIPT,
        f"python3 - --sample-seconds {sample_seconds}",
        timeout=max(90, int(sample_seconds + 60)),
    )
    payload = json.loads(stdout)
    connection_counts = parse_ss_counts(payload.get("ss_stdout") or "")
    instances = []
    for raw in payload.get("instances") or []:
        endpoint_users = parse_xray_config(raw.get("config_json") or "")
        first = parse_xray_stats(((raw.get("stats_first") or {}).get("stdout")) or "")
        second = parse_xray_stats(((raw.get("stats_second") or {}).get("stdout")) or "")
        rates = {}
        users = set(first) | set(second)
        for user in users:
            a = first.get(user, UserStats())
            b = second.get(user, UserStats())
            rates[user] = UserRate(
                up_bps=max(0, b.uplink - a.uplink) / max(sample_seconds, 1),
                down_bps=max(0, b.downlink - a.downlink) / max(sample_seconds, 1),
            )
        order_items = {}
        for item in raw.get("order_items") or []:
            if "error" in item:
                continue
            username = str(item.get("username") or "")
            if not username:
                continue
            order_items[username] = OrderItem(
                order_id=int(item.get("order_id") or 0),
                order_no=str(item.get("order_no") or ""),
                item_id=int(item.get("item_id") or 0),
                ip=str(item.get("ip") or ""),
                port=int(item.get("port") or 0),
                username=username,
                order_status=str(item.get("order_status") or ""),
                item_status=str(item.get("item_status") or ""),
                customer=str(item.get("customer_name") or ""),
            )
        instances.append(
            InstanceSnapshot(
                node=node.name,
                service=str(raw.get("service") or ""),
                db_path=str(raw.get("db_path") or ""),
                api_server=str(raw.get("api_server") or ""),
                endpoint_users=endpoint_users,
                connection_counts=connection_counts,
                user_stats=second,
                user_rates=rates,
                order_items=order_items,
            )
        )
    return instances


def target_matches(targets: list[Target], ip: str, port: int, username: str) -> bool:
    for target in targets:
        if target.ip == ip and target.port == port and (target.username is None or target.username == username):
            return True
    return False


def find_hotspots(
    instances: list[InstanceSnapshot],
    thresholds: Thresholds,
    targets: list[Target],
) -> list[Hotspot]:
    hotspots = []
    for instance in instances:
        for endpoint, users in instance.endpoint_users.items():
            ip, port = endpoint
            counts = instance.connection_counts.get(endpoint, {})
            estab = counts.get("ESTAB", 0)
            fin = counts.get("FIN-WAIT-1", 0) + counts.get("FIN-WAIT-2", 0)
            syn = counts.get("SYN-RECV", 0)
            time_wait = counts.get("TIME-WAIT", 0)
            endpoint_triggered = False
            if estab >= thresholds.estab or fin >= thresholds.fin or syn >= thresholds.syn:
                endpoint_triggered = True
            for username in users:
                stats = instance.user_stats.get(username, UserStats())
                rate = instance.user_rates.get(username, UserRate())
                matched_target = target_matches(targets, ip, port, username)
                speed_triggered = rate.total_bps >= thresholds.speed_bps
                reasons = []
                if matched_target:
                    reasons.append("target")
                if estab >= thresholds.estab:
                    reasons.append(f"estab>={thresholds.estab}")
                if fin >= thresholds.fin:
                    reasons.append(f"fin>={thresholds.fin}")
                if syn >= thresholds.syn:
                    reasons.append(f"syn>={thresholds.syn}")
                if speed_triggered:
                    reasons.append(f"speed>={format_bytes(thresholds.speed_bps)}/s")
                if not reasons:
                    continue
                pause_allowed = matched_target or speed_triggered or (endpoint_triggered and len(users) == 1)
                if endpoint_triggered and len(users) > 1 and not pause_allowed:
                    reasons.append("shared-endpoint-review")
                item = instance.order_items.get(username)
                hotspots.append(
                    Hotspot(
                        node=instance.node,
                        service=instance.service,
                        db_path=instance.db_path,
                        ip=ip,
                        port=port,
                        username=username,
                        order_id=item.order_id if item else None,
                        order_no=item.order_no if item else "",
                        customer=item.customer if item else "",
                        estab=estab,
                        fin=fin,
                        syn=syn,
                        time_wait=time_wait,
                        total_bytes=stats.total,
                        speed_bps=rate.total_bps,
                        reason=",".join(reasons),
                        pause_allowed=pause_allowed,
                    )
                )
    return sorted(hotspots, key=lambda row: (row.speed_bps, row.estab + row.fin + row.syn), reverse=True)


def build_pause_sql(order_id: int) -> str:
    return textwrap.dedent(
        f"""
        update orders set status = 'disabled', updated_at = datetime('now') where id = {int(order_id)};
        update order_items set status = 'disabled', updated_at = datetime('now') where order_id = {int(order_id)};
        """
    ).strip()


def build_remote_pause_script(hotspots: list[Hotspot]) -> str:
    payload = [
        {
            "service": hotspot.service,
            "db_path": hotspot.db_path,
            "order_id": hotspot.order_id,
            "username": hotspot.username,
        }
        for hotspot in hotspots
        if hotspot.order_id
    ]
    return (
        "import json, os, shutil, sqlite3, subprocess, time\n"
        f"payload = json.loads({json.dumps(json.dumps(payload))!r})\n"
        "seen = set()\n"
        "results = []\n"
        "for item in payload:\n"
        "    key = (item['db_path'], int(item['order_id']))\n"
        "    if key in seen:\n"
        "        continue\n"
        "    seen.add(key)\n"
        "    db_path = item['db_path']\n"
        "    backup = f\"{db_path}.bak-hotspot-guard-{item['order_id']}-{time.strftime('%Y%m%d-%H%M%S')}\"\n"
        "    shutil.copy2(db_path, backup)\n"
        "    con = sqlite3.connect(db_path)\n"
        "    con.row_factory = sqlite3.Row\n"
        "    cur = con.cursor()\n"
        "    row = cur.execute('select o.id order_id, o.order_no, o.status order_status, oi.username, oi.status item_status from orders o join order_items oi on oi.order_id=o.id where o.id=? and oi.username=? limit 1', (int(item['order_id']), item['username'])).fetchone()\n"
        "    if not row:\n"
        "        raise SystemExit(f\"target order/user not found: {item}\")\n"
        "    with con:\n"
        "        con.execute(\"update orders set status = 'disabled', updated_at = datetime('now') where id = ?\", (int(item['order_id']),))\n"
        "        con.execute(\"update order_items set status = 'disabled', updated_at = datetime('now') where order_id = ?\", (int(item['order_id']),))\n"
        "    con.close()\n"
        "    results.append({'order_id': item['order_id'], 'username': item['username'], 'backup': backup, 'service': item['service']})\n"
        "services = sorted({item['service'] for item in payload})\n"
        "for service in services:\n"
        "    subprocess.run(['systemctl', 'restart', service], check=True, timeout=30)\n"
        "time.sleep(8)\n"
        "for item in payload:\n"
        "    root = os.path.dirname(os.path.dirname(item['db_path']))\n"
        "    config_path = os.path.join(root, 'data', 'xray', 'config.json')\n"
        "    with open(config_path, 'r', encoding='utf-8') as handle:\n"
        "        config_text = handle.read()\n"
        "    if item['username'] in config_text:\n"
        "        raise SystemExit(f\"username still present after restart: {item['username']} in {config_path}\")\n"
        "print(json.dumps({'paused': results, 'services': services}, ensure_ascii=False))\n"
    )


def pause_hotspots(nodes: list[Node], hotspots: list[Hotspot]) -> list[dict[str, Any]]:
    by_node: dict[str, list[Hotspot]] = collections.defaultdict(list)
    node_by_name = {node.name: node for node in nodes}
    for hotspot in hotspots:
        if hotspot.order_id is None or not hotspot.pause_allowed:
            continue
        by_node[hotspot.node].append(hotspot)
    results = []
    for node_name, node_hotspots in by_node.items():
        script = build_remote_pause_script(node_hotspots)
        stdout = run_ssh_script(node_by_name[node_name], script, "python3 -", timeout=120)
        results.append({"node": node_name, "result": json.loads(stdout)})
    return results


def format_bytes(value: float | int) -> str:
    units = ["B", "KB", "MB", "GB", "TB"]
    num = float(value or 0)
    for unit in units:
        if abs(num) < 1024 or unit == units[-1]:
            return f"{int(num)}B" if unit == "B" else f"{num:.2f}{unit}"
        num /= 1024
    return f"{num:.2f}TB"


def print_hotspots(hotspots: list[Hotspot]) -> None:
    if not hotspots:
        print("No hotspots matched.")
        return
    for item in hotspots:
        print(
            "HOTSPOT node={node} service={service} {ip}:{port} user={user} "
            "order={order} customer={customer} estab={estab} fin={fin} syn={syn} "
            "time_wait={time_wait} total={total} speed={speed}/s pause_allowed={pause_allowed} reason={reason}".format(
                node=item.node,
                service=item.service,
                ip=item.ip,
                port=item.port,
                user=item.username,
                order=item.order_no or "-",
                customer=item.customer or "-",
                estab=item.estab,
                fin=item.fin,
                syn=item.syn,
                time_wait=item.time_wait,
                total=format_bytes(item.total_bytes),
                speed=format_bytes(item.speed_bps),
                pause_allowed=str(item.pause_allowed).lower(),
                reason=item.reason,
            )
        )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["scan", "pause"])
    parser.add_argument("--inventory", required=True, help="JSON inventory path. Keep credentials outside git.")
    parser.add_argument("--sample-seconds", type=float, default=10.0)
    parser.add_argument("--estab", type=int, default=500)
    parser.add_argument("--fin", type=int, default=1000)
    parser.add_argument("--syn", type=int, default=100)
    parser.add_argument("--speed-mbps", type=float, default=5.0)
    parser.add_argument("--target", action="append", type=parse_target, default=[], help="ip:port or ip:port:username")
    parser.add_argument("--json", action="store_true", help="Print machine-readable JSON.")
    parser.add_argument("--dry-run", action="store_true", help="For pause: show matches without changing nodes.")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    nodes = load_inventory(args.inventory)
    thresholds = Thresholds(
        estab=args.estab,
        fin=args.fin,
        syn=args.syn,
        speed_bps=args.speed_mbps * 1024 * 1024,
    )
    all_instances: list[InstanceSnapshot] = []
    errors = []
    for node in nodes:
        try:
            all_instances.extend(collect_node(node, args.sample_seconds))
        except Exception as exc:
            errors.append({"node": node.name, "error": str(exc)})
            print(f"ERROR node={node.name} {exc}", file=sys.stderr)
    hotspots = find_hotspots(all_instances, thresholds, args.target)
    if args.json:
        print(json.dumps({"hotspots": [dataclasses.asdict(row) for row in hotspots], "errors": errors}, ensure_ascii=False))
    else:
        print_hotspots(hotspots)
        if errors:
            print(f"Errors: {len(errors)} node(s) failed", file=sys.stderr)
    if args.command == "pause" and hotspots and not args.dry_run:
        pause_candidates = [hotspot for hotspot in hotspots if hotspot.order_id and hotspot.pause_allowed]
        if not pause_candidates:
            print("No pause candidates have matching order_id and safe attribution.", file=sys.stderr)
            return 2
        results = pause_hotspots(nodes, pause_candidates)
        if args.json:
            print(json.dumps({"pause_results": results}, ensure_ascii=False))
        else:
            print("Pause results:")
            for result in results:
                print(json.dumps(result, ensure_ascii=False))
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
