#!/usr/bin/env python3

import argparse
import json
from pathlib import Path
import sys
import urllib.error
import urllib.request


def load_config(config_path: Path) -> dict:
    with config_path.open("r", encoding="utf-8") as f:
        return json.load(f)


def parse_fs_addr(fs_addr: str) -> tuple[str, str]:
    parts = fs_addr.strip().split(":")
    if len(parts) != 2 or not parts[0] or not parts[1]:
        raise ValueError("fs_addr must be in ip:port format")
    return parts[0], parts[1]


def parse_http_url(listen_addr: str) -> str:
    if not listen_addr.strip():
        raise ValueError("http.listen_addr must not be empty")
    return f"http://127.0.0.1:{listen_addr.rsplit(':', 1)[1]}/calls"


def build_parser() -> argparse.ArgumentParser:
    root_dir = Path(__file__).resolve().parent.parent
    parser = argparse.ArgumentParser(description="Create a call through micro-uac HTTP API")
    parser.add_argument(
        "--config",
        default=str(root_dir / "config.json"),
        help="path to config.json",
    )
    parser.add_argument(
        "--from-number",
        default="1001",
        help="caller number used in the SIP From header",
    )
    parser.add_argument(
        "--to-number",
        default="1012",
        help="callee number used in the SIP To header and request URI",
    )
    parser.add_argument(
        "--fs-addr",
        help="FreeSWITCH address in ip:port format; overrides config.json fs_addr",
    )
    parser.add_argument(
        "--line-addr",
        help="line address in ip:port format; used to build To and request_uri",
    )
    parser.add_argument(
        "--target-number",
        default="1012",
        help="destination number used in X-Sip-Client-Target-Uri",
    )
    return parser


def main() -> int:
    args = build_parser().parse_args()
    config = load_config(Path(args.config))
    external_ip = config["sip"]["external_ip"]
    fs_addr = args.fs_addr or config.get("fs_addr", "10.103.2.34:5060")
    line_addr = args.line_addr or fs_addr
    line_host, line_port = parse_fs_addr(line_addr)
    url = parse_http_url(config["http"]["listen_addr"])

    payload = {
        "from": f"<sip:{args.from_number}@{external_ip}>",
        "to": f"<sip:{args.to_number}@{parse_fs_addr(fs_addr)[0]}>",
        "request_uri": f"sip:{args.to_number}@{line_host}:{line_port}",
        "fs_addr": fs_addr,
        "line_addr": line_addr,
        "target_uri": f"sip:{args.target_number}@{line_host}:{line_port}",
        "audio_file": "audio/demo.wav",
        "codec": "pcmu",
        "frame_ms": 20,
    }

    body = json.dumps(payload, ensure_ascii=True, indent=2).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    print("request url:")
    print(url)
    print("request body:")
    print(body.decode("utf-8"))

    try:
        with urllib.request.urlopen(request) as response:
            response_body = response.read().decode("utf-8")
            print("response status:")
            print(response.status)
            print("response body:")
            print(response_body)
            return 0
    except urllib.error.HTTPError as exc:
        response_body = exc.read().decode("utf-8", errors="replace")
        print("response status:")
        print(exc.code)
        print("response body:")
        print(response_body)
        return 1
    except urllib.error.URLError as exc:
        print(f"request failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
