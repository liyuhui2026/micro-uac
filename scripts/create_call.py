#!/usr/bin/env python3

import json
from pathlib import Path
import sys
import urllib.error
import urllib.request


def load_config() -> dict:
    config_path = Path(__file__).resolve().parent.parent / "config.json"
    with config_path.open("r", encoding="utf-8") as f:
        return json.load(f)


def parse_fs_addr(fs_addr: str) -> tuple[str, str]:
    parts = fs_addr.strip().split(":")
    if len(parts) != 2 or not parts[0] or not parts[1]:
        raise ValueError("fs_addr must be in ip:port format")
    return parts[0], parts[1]


def main() -> int:
    config = load_config()
    external_ip = config["sip"]["external_ip"]
    fs_addr = config.get("fs_addr", "10.103.2.34:5060")
    fs_host, fs_port = parse_fs_addr(fs_addr)

    url = "http://127.0.0.1:8090/calls"
    payload = {
        "from": f"<sip:1001@{external_ip}>",
        "to": f"<sip:1012@{fs_host}>",
        "request_uri": f"sip:1012@{fs_host}:{fs_port}",
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
