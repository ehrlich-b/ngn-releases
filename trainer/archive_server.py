"""Serve fixed campaign files by byte range, with optional one-time uploads.

    Run inside the campaign's CPU/memory scope. The file manifest supplies
    verified SHA-256s, sizes and mtime_ns, generated once at conversion. Only
    listed names are served; uploads write new files into this server's own directory.
"""

import argparse
import ctypes
import json
import os
from pathlib import Path
import re
import select
import socket
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def make_server(root, token, address, uploads=False):
    root = Path(root)

    class Handler(BaseHTTPRequestHandler):
        def setup(self):
            # Descriptor requests can wait for hours across a remote host's NAT.
            # Keep that connection present until the corpus is published.
            self.request.setsockopt(socket.SOL_SOCKET, socket.SO_KEEPALIVE, 1)
            self.request.setsockopt(socket.IPPROTO_TCP, socket.TCP_KEEPIDLE, 60)
            self.request.setsockopt(socket.IPPROTO_TCP, socket.TCP_KEEPINTVL, 30)
            self.request.setsockopt(socket.IPPROTO_TCP, socket.TCP_KEEPCNT, 6)
            super().setup()

        def log_message(self, *_args):
            pass  # URLs contain the private campaign access token.

        def name(self):
            prefix = "/" + token + "/"
            name = self.path[len(prefix):] if self.path.startswith(prefix) else ""
            return name if re.fullmatch(r"[a-zA-Z0-9_.-]+", name) else None

        def do_PUT(self):
            name = self.name()
            if not uploads or not name:
                self.send_error(403)
                return
            path = root / name
            temporary = root / (name + ".upload")
            size = int(self.headers.get("Content-Length", -1))
            if size < 0 or size > 25 * 1024**3 or path.exists() or temporary.exists():
                self.send_error(409)
                return
            remaining = size
            with temporary.open("xb") as output:
                while remaining:
                    chunk = self.rfile.read(min(1024**2, remaining))
                    if not chunk:
                        raise ConnectionError("Incomplete upload")
                    output.write(chunk)
                    remaining -= len(chunk)
                output.flush()
                os.fsync(output.fileno())
            temporary.rename(path)
            self.send_response(201)
            self.send_header("Content-Length", "0")
            self.end_headers()

        def do_GET(self):
            name = self.name()
            if name == "descriptor.json":
                # One interruptible kernel wait, allowing a detached client
                # continuation to awaken when the data gather publishes.
                descriptor = root / "t74.archive.json"
                libc = ctypes.CDLL(None, use_errno=True)
                fd = libc.inotify_init1(os.O_CLOEXEC)
                if fd < 0 or libc.inotify_add_watch(fd, os.fsencode(root), 0x80) < 0:
                    if fd >= 0:
                        os.close(fd)
                    self.send_error(500)
                    return
                poll = select.poll()
                poll.register(fd, select.POLLIN)
                try:
                    while not descriptor.exists():
                        if not poll.poll(12 * 3600 * 1000):
                            self.send_error(504)
                            return
                        os.read(fd, 65536)
                finally:
                    os.close(fd)
                raw = descriptor.read_bytes()
                self.send_response(200)
                self.send_header("Content-Length", str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)
                return
            manifest = root / "files.json"
            files = json.loads(manifest.read_text()) if manifest.exists() else {}
            if not name or name not in files:
                self.send_error(404)
                return
            pin = files[name]
            path = root / name
            stat = path.stat()
            etag = '"' + pin["sha256"] + '"'
            if (stat.st_size != pin["bytes"] or stat.st_mtime_ns != pin["mtime_ns"]
                    or self.headers.get("If-Match") != etag):
                self.send_error(412)
                return
            match = re.fullmatch(r"bytes=(\d+)-(\d+)", self.headers.get("Range", ""))
            if not match:
                self.send_error(416)
                return
            first, last = map(int, match.groups())
            if not 0 <= first <= last < stat.st_size:
                self.send_error(416)
                return
            self.send_response(206)
            self.send_header("Content-Length", str(last - first + 1))
            self.send_header("Content-Range", f"bytes {first}-{last}/{stat.st_size}")
            self.send_header("ETag", etag)
            self.end_headers()
            with path.open("rb") as source:
                source.seek(first)
                remaining = last - first + 1
                while remaining:
                    chunk = source.read(min(1024**2, remaining))
                    if not chunk:
                        raise IOError("Archive truncated during read")
                    self.wfile.write(chunk)
                    remaining -= len(chunk)

    return ThreadingHTTPServer(address, Handler)


if __name__ == "__main__":
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("--root", type=Path, required=True)
    cli.add_argument("--token-file", type=Path, required=True)
    cli.add_argument("--bind", default="127.0.0.1")
    cli.add_argument("--port", type=int, default=8766)
    cli.add_argument("--uploads", action="store_true")
    args = cli.parse_args()
    make_server(args.root, args.token_file.read_text().strip(),
                (args.bind, args.port), args.uploads).serve_forever()
