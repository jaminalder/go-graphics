"""Verify actual container boundaries and gallery survival when rendering stops."""
import json
import html
import http.cookiejar
import os
import re
import subprocess
import time
import urllib.error
import urllib.request
import urllib.parse

COMPOSE = ["docker", "compose", "-p", os.environ["ART_COMPOSE_PROJECT"], "--env-file", "deploy/local.env", "-f", "deploy/compose.yaml"]


def compose(*args, check=True):
    return subprocess.run(COMPOSE + list(args), check=check, capture_output=True, text=True)


def request(path, headers=None):
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(urllib.request.Request(os.environ["ART_ORIGIN"] + path, headers=headers or {}), timeout=10) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as error:
        return error.code, error.read()


def main():
    assert request("/")[0] == 200
    assert request("/", {"Host": "attacker.invalid"})[0] != 200
    for path in ("/metrics", "/ready", "/health/live", "/generation/off"):
        assert request(path)[0] == 404, path
    # Visitor-supplied proxy headers do not alter routing or canonical origin.
    assert request("/", {"X-Art-Client": "192.0.2.4", "X-Forwarded-Host": "attacker.invalid"})[0] == 200
    ids = compose("ps", "-q").stdout.split()
    containers = json.loads(subprocess.check_output(["docker", "inspect", *ids]))
    services = {c["Config"]["Labels"]["com.docker.compose.service"]: c for c in containers}
    for name, memory, cpu in (("web", 384 * 1024**2, 500000000), ("renderer", 2 * 1024**3, 1500000000)):
        host = services[name]["HostConfig"]
        assert host["Memory"] == host["MemorySwap"] == memory
        assert host["NanoCpus"] == cpu and host["PidsLimit"] == 64
        assert host["ReadonlyRootfs"] and "ALL" in host["CapDrop"]
        assert "no-new-privileges:true" in host["SecurityOpt"]
        assert not host["PortBindings"]
        assert services[name]["Config"]["User"] != "0"
    assert services["renderer"]["HostConfig"]["NetworkMode"] == "none"
    assert not any(m["Destination"] == "/var/cache/art" for m in services["renderer"]["Mounts"])
    assert all(m["Destination"] != "/var/run/docker.sock" for c in containers for m in c["Mounts"])
    socket = next(m for m in services["web"]["Mounts"] if m["Destination"] == "/run/art")
    assert not socket["RW"]
    compose("exec", "-T", "web", "/app/artctl", "ready")
    # Real form -> bounded queue -> socket -> child -> cached PNG, through Caddy.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    origin = os.environ["ART_ORIGIN"]
    with opener.open(origin + "/art/iris", timeout=10) as response:
        page = response.read().decode()
    form = dict((name, html.unescape(value)) for name, value in re.findall(r'type="hidden" name="([^"]+)" value="([^"]*)"', page))
    form.update(style="", colour="")
    req = urllib.request.Request(origin + "/explorations", data=urllib.parse.urlencode(form).encode(), headers={"Origin": origin})
    with opener.open(req, timeout=10) as response:
        exploration = response.url
    deadline = time.monotonic() + 70
    images = []
    while time.monotonic() < deadline:
        with opener.open(exploration, timeout=10) as response:
            images = re.findall(r'src="(/images/[^"]+)"', response.read().decode())
        if len(images) == 4:
            break
        time.sleep(2)
    assert len(images) == 4, "four real render results did not arrive"
    for path in images:
        with opener.open(origin + path, timeout=10) as response:
            assert response.read().startswith(b"\x89PNG\r\n\x1a\n")
    compose("stop", "renderer")
    assert request("/")[0] == 200
    assert compose("exec", "-T", "web", "/app/artctl", "ready", check=False).returncode != 0
    compose("up", "-d", "--no-build", "--wait", "--wait-timeout", "60", "renderer")
    compose("exec", "-T", "web", "/app/artctl", "ready")
    # Recreate the renderer against an existing socket volume: stale socket recovery.
    compose("up", "-d", "--no-build", "--force-recreate", "--wait", "--wait-timeout", "60", "renderer")
    compose("exec", "-T", "web", "/app/artctl", "ready")
    print("Compose boundaries, private paths, four real PNGs, renderer failure and recreation passed")


if __name__ == "__main__":
    main()
