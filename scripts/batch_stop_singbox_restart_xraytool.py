#!/usr/bin/env python3
import argparse
import concurrent.futures
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


KV_RE = re.compile(r"(\w+)=(\"[^\"]*\"|\S+)")
DOT2_HOST_RE = re.compile(r"^\d+\.\d+\.\d+\.2$")
DEFAULT_EXCLUDE = {
    "168.143.226.2",
    "38.13.167.2",
    "38.13.188.2",
}
DEFAULT_REMOTE_COMMAND = (
    "sudo systemctl stop sing-box 2>/dev/null || true; "
    "sudo systemctl disable sing-box 2>/dev/null || true; "
    "sudo systemctl stop singbox 2>/dev/null || true; "
    "sudo systemctl disable singbox 2>/dev/null || true; "
    "sudo systemctl restart xraytool; "
    "systemctl is-active xraytool"
)


@dataclass(frozen=True)
class ServerEntry:
    line_no: int
    host: str
    port: int
    user: str
    password: str

    @property
    def key(self) -> tuple[str, int, str]:
        return (self.host, self.port, self.user)


def resolve_default_servers_file() -> Path:
    return (Path(__file__).resolve().parent.parent.parent / "servers.md").resolve()


def parse_servers(path: Path) -> tuple[list[ServerEntry], list[str]]:
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    out: list[ServerEntry] = []
    warnings: list[str] = []
    for idx, line in enumerate(lines, 1):
        text = line.strip()
        if not text:
            continue
        kv = {}
        for key, value in KV_RE.findall(text):
            if len(value) >= 2 and value[0] == '"' and value[-1] == '"':
                value = value[1:-1]
            kv[key] = value
        if not kv:
            warnings.append(f"line {idx}: cannot parse key-values")
            continue

        host = kv.get("host", "").strip()
        if not host:
            warnings.append(f"line {idx}: host missing")
            continue
        port_text = kv.get("port", "22").strip()
        try:
            port = int(port_text)
        except ValueError:
            warnings.append(f"line {idx}: invalid port '{port_text}'")
            continue
        if not (1 <= port <= 65535):
            warnings.append(f"line {idx}: invalid port {port}")
            continue

        user = kv.get("user", "root").strip() or "root"
        password = kv.get("password", "")
        if not password:
            warnings.append(f"line {idx}: password missing for {host}")
            continue

        out.append(ServerEntry(idx, host, port, user, password))
    return out, warnings


def dedupe(entries: list[ServerEntry]) -> list[ServerEntry]:
    seen: set[tuple[str, int, str]] = set()
    out: list[ServerEntry] = []
    for entry in entries:
        if entry.key in seen:
            continue
        seen.add(entry.key)
        out.append(entry)
    return out


def run_remote(
    entry: ServerEntry, remote_command: str, timeout: int
) -> tuple[ServerEntry, bool, str]:
    cmd = [
        "sshpass",
        "-p",
        entry.password,
        "ssh",
        "-o",
        "StrictHostKeyChecking=no",
        "-o",
        "UserKnownHostsFile=/dev/null",
        "-o",
        "LogLevel=ERROR",
        "-o",
        "ConnectTimeout=10",
        "-o",
        "PreferredAuthentications=password",
        "-p",
        str(entry.port),
        f"{entry.user}@{entry.host}",
        "bash",
        "-lc",
        remote_command,
    ]
    try:
        result = subprocess.run(
            cmd, capture_output=True, text=True, timeout=timeout, check=False
        )
    except subprocess.TimeoutExpired:
        return entry, False, f"timeout after {timeout}s"
    except Exception as exc:  # pylint: disable=broad-except
        return entry, False, str(exc)

    if result.returncode != 0:
        detail = (result.stderr or result.stdout or "").strip()
        if not detail:
            detail = f"exit code {result.returncode}"
        return entry, False, detail

    return entry, True, (result.stdout or "ok").strip()


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Stop sing-box and restart xraytool on batch servers"
    )
    parser.add_argument(
        "--servers-file",
        default=str(resolve_default_servers_file()),
        help="servers.md path",
    )
    parser.add_argument(
        "--host-regex", default=DOT2_HOST_RE.pattern, help="host filter regex"
    )
    parser.add_argument(
        "--exclude-hosts",
        default=",".join(sorted(DEFAULT_EXCLUDE)),
        help="comma-separated hosts to skip",
    )
    parser.add_argument("--max-workers", type=int, default=6, help="parallel workers")
    parser.add_argument(
        "--timeout", type=int, default=180, help="per-host timeout seconds"
    )
    parser.add_argument(
        "--limit", type=int, default=0, help="run first N hosts after filtering"
    )
    parser.add_argument(
        "--remote-command",
        default=DEFAULT_REMOTE_COMMAND,
        help="remote command to execute",
    )
    parser.add_argument(
        "--execute", action="store_true", help="actually execute remote command"
    )
    args = parser.parse_args()

    if not Path(args.servers_file).is_file():
        print(f"[ERROR] servers file not found: {args.servers_file}")
        return 2

    entries, warnings = parse_servers(Path(args.servers_file).resolve())
    host_re = re.compile(args.host_regex)
    exclude = {item.strip() for item in args.exclude_hosts.split(",") if item.strip()}
    selected = [
        entry
        for entry in entries
        if host_re.search(entry.host) and entry.host not in exclude
    ]
    selected = dedupe(selected)
    selected.sort(key=lambda item: (item.host, item.port, item.user))
    if args.limit > 0:
        selected = selected[: args.limit]

    print(f"servers file: {args.servers_file}")
    print(f"parsed entries: {len(entries)}")
    print(f"matched entries: {len(selected)} (host_regex={args.host_regex})")
    if warnings:
        print(f"parse warnings: {len(warnings)}")
        for warning in warnings[:5]:
            print(f"  - {warning}")
    for entry in selected:
        print(f"  - {entry.host}:{entry.port} ({entry.user}) line={entry.line_no}")

    if not selected:
        print("no matched server entries")
        return 1

    if not args.execute:
        print("dry-run mode. add --execute to run remote command.")
        return 0

    if (
        subprocess.run(
            ["which", "sshpass"], capture_output=True, text=True, check=False
        ).returncode
        != 0
    ):
        print("[ERROR] sshpass is required for --execute")
        return 2

    ok = 0
    fail = 0
    print("starting remote operations...")
    with concurrent.futures.ThreadPoolExecutor(
        max_workers=max(1, args.max_workers)
    ) as executor:
        futures = [
            executor.submit(run_remote, entry, args.remote_command, args.timeout)
            for entry in selected
        ]
        for future in concurrent.futures.as_completed(futures):
            entry, success, detail = future.result()
            if success:
                ok += 1
                state = detail.splitlines()[-1] if detail else "ok"
                print(f"[OK] {entry.host}:{entry.port} -> {state}")
            else:
                fail += 1
                print(f"[FAIL] {entry.host}:{entry.port} -> {detail}")

    print(f"done. success={ok} failed={fail}")
    return 0 if fail == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
