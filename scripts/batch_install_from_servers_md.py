#!/usr/bin/env python3
import argparse
import concurrent.futures
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


DEFAULT_ADMIN_USER = "byx"
DEFAULT_PANEL_PORT = 29154

KV_RE = re.compile(r"(\w+)=(\"[^\"]*\"|\S+)")
DOT2_IP_RE = re.compile(r"^\d+\.\d+\.\d+\.2$")


@dataclass(frozen=True)
class ServerEntry:
    line_no: int
    host: str
    port: int
    user: str
    password: str
    title: str

    @property
    def key(self) -> tuple[str, int, str]:
        return (self.host, self.port, self.user)


def parse_servers(path: Path) -> tuple[list[ServerEntry], list[str]]:
    if not path.is_file():
        raise FileNotFoundError(f"servers file not found: {path}")

    entries: list[ServerEntry] = []
    warnings: list[str] = []
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()

    for idx, raw in enumerate(lines, start=1):
        text = raw.strip()
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
        user = kv.get("user", "root").strip() or "root"
        password = kv.get("password", "")
        title = kv.get("title", host)
        port_text = kv.get("port", "22").strip()

        if not host:
            warnings.append(f"line {idx}: host missing")
            continue
        if not password:
            warnings.append(f"line {idx}: password missing for {host}")
            continue
        try:
            port = int(port_text)
        except ValueError:
            warnings.append(f"line {idx}: invalid port '{port_text}'")
            continue
        if port <= 0 or port > 65535:
            warnings.append(f"line {idx}: invalid port {port}")
            continue

        entries.append(
            ServerEntry(
                line_no=idx,
                host=host,
                port=port,
                user=user,
                password=password,
                title=title,
            )
        )

    return entries, warnings


def dedupe_entries(entries: Iterable[ServerEntry]) -> list[ServerEntry]:
    out: list[ServerEntry] = []
    seen: set[tuple[str, int, str]] = set()
    for entry in entries:
        if entry.key in seen:
            continue
        seen.add(entry.key)
        out.append(entry)
    return out


def run_remote_install(
    entry: ServerEntry, remote_command: str, timeout: int
) -> tuple[ServerEntry, bool, str]:
    target = f"{entry.user}@{entry.host}"
    command = [
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
        "-p",
        str(entry.port),
        target,
        "bash",
        "-lc",
        remote_command,
    ]
    try:
        result = subprocess.run(
            command, capture_output=True, text=True, timeout=timeout, check=False
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

    return entry, True, "ok"


def resolve_default_servers_file() -> Path:
    return (Path(__file__).resolve().parent.parent.parent / "servers.md").resolve()


def build_remote_command(admin_user: str, admin_pass: str, panel_port: int) -> str:
    return (
        "curl -fsSL "
        "https://raw.githubusercontent.com/helloandworlder/xraytool-managed-service/refs/heads/main/deploy/public-install.sh "
        f"| sudo bash -s -- --non-interactive --xray-api-port random --port {panel_port} "
        f"--admin-user {admin_user} --admin-pass '{admin_pass}'"
    )


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Batch install xraytool from servers.md"
    )
    parser.add_argument(
        "--servers-file",
        default=str(resolve_default_servers_file()),
        help="servers.md path",
    )
    parser.add_argument(
        "--remote-command", default="", help="full remote install command"
    )
    parser.add_argument(
        "--admin-user",
        default=DEFAULT_ADMIN_USER,
        help="admin user for generated command",
    )
    parser.add_argument(
        "--admin-pass",
        default="",
        help="admin password for generated command (required unless --remote-command set)",
    )
    parser.add_argument(
        "--panel-port",
        type=int,
        default=DEFAULT_PANEL_PORT,
        help="panel port for generated command",
    )
    parser.add_argument(
        "--host-regex", default=DOT2_IP_RE.pattern, help="host filter regex"
    )
    parser.add_argument("--max-workers", type=int, default=6, help="parallel workers")
    parser.add_argument(
        "--timeout", type=int, default=900, help="per-host timeout seconds"
    )
    parser.add_argument(
        "--limit", type=int, default=0, help="run first N hosts after filtering"
    )
    parser.add_argument(
        "--execute", action="store_true", help="actually execute remote install"
    )
    args = parser.parse_args()

    host_re = re.compile(args.host_regex)
    all_entries, warnings = parse_servers(
        Path(args.servers_file).expanduser().resolve()
    )

    selected = [entry for entry in all_entries if host_re.search(entry.host)]
    selected = dedupe_entries(selected)
    selected.sort(key=lambda entry: (entry.host, entry.port, entry.user))
    if args.limit > 0:
        selected = selected[: args.limit]

    print(f"servers file: {args.servers_file}")
    print(f"parsed entries: {len(all_entries)}")
    print(f"matched entries: {len(selected)} (host_regex={args.host_regex})")
    if warnings:
        print(f"parse warnings: {len(warnings)}")
        for warning in warnings[:5]:
            print(f"  - {warning}")

    if not selected:
        print("no matched server entries")
        return 1

    for entry in selected:
        print(
            f"  - {entry.host}:{entry.port} ({entry.user}) line={entry.line_no} title={entry.title}"
        )

    if not args.execute:
        print("dry-run mode. add --execute to run remote installer.")
        return 0

    remote_command = args.remote_command.strip()
    if not remote_command:
        if not args.admin_pass:
            print(
                "[ERROR] --admin-pass is required when --remote-command is not provided"
            )
            return 2
        remote_command = build_remote_command(
            admin_user=args.admin_user.strip() or DEFAULT_ADMIN_USER,
            admin_pass=args.admin_pass,
            panel_port=args.panel_port,
        )

    if (
        subprocess.run(
            ["which", "sshpass"], capture_output=True, text=True, check=False
        ).returncode
        != 0
    ):
        print("[ERROR] sshpass is required for --execute")
        return 2

    print("starting remote installs...")
    ok_count = 0
    fail_count = 0

    with concurrent.futures.ThreadPoolExecutor(
        max_workers=max(1, args.max_workers)
    ) as executor:
        futures = [
            executor.submit(run_remote_install, entry, remote_command, args.timeout)
            for entry in selected
        ]
        for future in concurrent.futures.as_completed(futures):
            entry, ok, detail = future.result()
            if ok:
                ok_count += 1
                print(f"[OK] {entry.host}:{entry.port}")
            else:
                fail_count += 1
                print(f"[FAIL] {entry.host}:{entry.port} -> {detail}")

    print(f"done. success={ok_count} failed={fail_count}")
    return 0 if fail_count == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
