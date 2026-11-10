"""Test web server for test-install.sh: serves a folder with HTTP Range support,
logs each GET (path and Range header) to <root>/../requests.log, and when
<root>/../cut holds a number N sends at most N bytes of a .zip and then drops
the connection, like a flaky network."""
import http.server
import os
import sys

ROOT, PORT = sys.argv[1], int(sys.argv[2])
LOG = os.path.join(ROOT, "..", "requests.log")
CUT = os.path.join(ROOT, "..", "cut")


class H(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_HEAD(self):
        self.serve(head=True)

    def do_GET(self):
        with open(LOG, "a") as f:
            f.write(f"{self.path} {self.headers.get('Range', '-')}\n")
        self.serve(head=False)

    def serve(self, head):
        p = os.path.join(ROOT, self.path.lstrip("/"))
        if not os.path.isfile(p):
            self.send_error(404)
            return
        data = open(p, "rb").read()
        size, start, status = len(data), 0, 200
        rng = self.headers.get("Range")
        if rng and rng.startswith("bytes="):
            start = int(rng[6:].split("-")[0] or 0)
            if start >= size:
                self.send_response(416)
                self.send_header("Content-Range", f"bytes */{size}")
                self.send_header("Content-Length", "0")
                self.end_headers()
                return
            status = 206
        body = data[start:]
        self.send_response(status)
        self.send_header("Accept-Ranges", "bytes")
        self.send_header("Content-Length", str(len(body)))
        if status == 206:
            self.send_header("Content-Range", f"bytes {start}-{size - 1}/{size}")
        self.end_headers()
        if head:
            return
        if p.endswith(".zip") and os.path.exists(CUT):
            self.wfile.write(body[: int(open(CUT).read())])
            self.wfile.flush()
            self.close_connection = True
            return
        self.wfile.write(body)


http.server.ThreadingHTTPServer(("127.0.0.1", PORT), H).serve_forever()
