"""Exercise default Compose from empty config/log directories on a Docker host.

The isolation overlay changes names, build context, published port and the host
data directory; it preserves bind-volume semantics, devices, config mounts,
capabilities and application settings.
No bootstrap passwords, hashes or session tokens are printed or uploaded.
"""

import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import stat
import subprocess
import tempfile
import urllib.request
import uuid


ROOT = Path(__file__).resolve().parents[1]


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def main():
    require(shutil.which("docker"), "Docker and Compose 2.24.4+ are required")
    project = "nas-os-smoke-" + uuid.uuid4().hex[:12]
    image = "nas-os-smoke:" + project
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]

    # Clear application overrides inherited from a developer's host.
    env = {k: v for k, v in os.environ.items()
           if not k.startswith(("NAS_OS_", "COMPOSE_", "NAS_CSRF_"))}
    env.update(NAS_OS_ENV="production", NAS_CSRF_KEY=secrets.token_hex(32))
    with tempfile.TemporaryDirectory(prefix=project + "-") as temp:
        fixture = Path(temp)
        (fixture / "configs").mkdir()
        (fixture / "logs").mkdir()
        (fixture / "data").mkdir()
        shutil.copyfile(ROOT / "docker-compose.yml", fixture / "compose.yml")
        # YAML !override ensures the default published port is not also bound.
        (fixture / "isolation.yml").write_text(
            "services:\n  nas-os:\n"
            f"    container_name: {project}\n    image: {image}\n"
            f"    build:\n      context: {json.dumps(str(ROOT))}\n"
            f'    ports: !override ["127.0.0.1:{port}:8080"]\n'
            f"networks:\n  default:\n    name: {project}-network\n"
            "volumes:\n  nas-os-data:\n    driver_opts:\n"
            f"      device: {json.dumps(str(fixture / 'data'))}\n")
        command = ["docker", "compose", "--project-name", project,
                   "--file", str(fixture / "compose.yml"),
                   "--file", str(fixture / "isolation.yml")]

        def compose(*args, capture=False, check=True):
            return subprocess.run(command + list(args), cwd=fixture, env=env,
                                  check=check, text=True, capture_output=capture)

        def request(path, payload=None, token=None):
            headers = {}
            data = None
            if payload is not None:
                data = json.dumps(payload).encode()
                headers["Content-Type"] = "application/json"
            if token:
                headers["Authorization"] = "Bearer " + token
            req = urllib.request.Request(f"http://127.0.0.1:{port}" + path,
                                         data=data, headers=headers)
            # Test the local service directly, regardless of host proxy settings.
            opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
            with opener.open(req, timeout=10) as response:
                body = response.read()
                if path.startswith("/api/"):
                    result = json.loads(body)
                    require(result.get("code") == 0, f"API failed: {path}")
                    return result["data"]
                require(body, f"Empty WebUI response: {path}")
                if path in ("/", "/login", "/webui/pages/login.html"):
                    require("text/html" in response.headers.get("Content-Type", ""),
                            f"WebUI is not HTML: {path}")

        def health_and_ui():
            require(request("/api/v1/system/health")["status"] == "healthy",
                    "Core modules are unhealthy")
            for path in ("/", "/login", "/webui/pages/login.html",
                         "/js/api.js", "/css/design-system.css",
                         "/webui/js/api.js", "/webui/css/design-system.css"):
                request(path)

        def login(password, must_change):
            data = request("/api/v1/auth/login",
                           {"username": "admin", "password": password})
            require(data.get("must_change_password", False) is must_change,
                    "Unexpected first-login password policy")
            require(data.get("token"), "Login did not return a session")
            return data["token"]

        built = False
        try:
            plan = json.loads(compose("config", "--format", "json", capture=True).stdout)
            service = plan["services"]["nas-os"]
            mounts = [v for v in service["volumes"] if v["target"] == "/etc/nas-os"]
            require(len(mounts) == 1 and not mounts[0].get("read_only", False),
                    "Default config/state mount must be writable")
            require(not service.get("devices"), "Default deployment requires host devices")
            options = plan["volumes"]["nas-os-data"]["driver_opts"]
            require(options.get("type") == "none" and options.get("o") == "bind"
                    and options.get("device") == str(fixture / "data"),
                    "Data volume must retain bind semantics in an isolated directory")
            compose("build", "nas-os")
            built = True

            # Reproduce the previous readonly bootstrap failure with the real image.
            readonly = subprocess.run(
                ["docker", "run", "--rm", "--name", project + "-readonly",
                 "--network", "none", "--mount",
                 f"type=bind,src={fixture / 'configs'},dst=/etc/nas-os,readonly", image],
                env=env, text=True, capture_output=True, timeout=30)
            require(readonly.returncode != 0 and "read-only file system" in
                    readonly.stdout + readonly.stderr,
                    "Readonly config did not fail closed during bootstrap")
            print("PASS: readonly bootstrap failure reproduced", flush=True)

            compose("up", "--detach", "--wait", "--wait-timeout", "120", "nas-os")
            health_and_ui()
            password_file = fixture / "configs/.admin_password"
            # Docker creates root-owned secret files; docker cp makes a private
            # readable copy for the invoking user even when CI is not root.
            secret_copy = fixture / "bootstrap-password"
            subprocess.run(["docker", "cp", project + ":/etc/nas-os/.admin_password",
                            str(secret_copy)], check=True, env=env, capture_output=True)
            require(stat.S_IMODE(password_file.stat().st_mode) == 0o600,
                    "Bootstrap password permissions are not 0600")
            token = login(secret_copy.read_text().strip(), must_change=True)
            new_password = "Smoke1!" + secrets.token_hex(16)
            request("/api/v1/me/password",
                    {"old_password": secret_copy.read_text().strip(),
                     "new_password": new_password}, token=token)
            token = login(new_password, must_change=False)
            admin_id = request("/api/v1/me", token=token)["id"]
            print("PASS: health, WebUI assets, bootstrap and password rotation", flush=True)

            compose("up", "--detach", "--force-recreate", "--wait",
                    "--wait-timeout", "120", "nas-os")
            health_and_ui()
            token = login(new_password, must_change=False)
            require(request("/api/v1/me", token=token)["id"] == admin_id,
                    "Admin identity changed after container recreation")
            print("PASS: rotated password and identity survive container recreation", flush=True)
        except Exception:
            compose("ps", "--all", check=False)
            compose("logs", "--no-color", "--tail", "100", "nas-os", check=False)
            raise
        finally:
            subprocess.run(["docker", "rm", "--force", project + "-readonly"],
                           env=env, check=False, capture_output=True)
            compose("down", "--volumes", "--remove-orphans", check=False)
            if built:
                # Root-owned private TLS directories in the bind data volume
                # cannot be removed by a non-root CI user. Use this same image
                # to remove only our four disposable paths after stopping nasd;
                # do not relax production file modes or require host sudo.
                subprocess.run(
                    ["docker", "run", "--rm", "--pull", "never",
                     "--network", "none", "--read-only", "--mount",
                     f"type=bind,src={fixture},dst=/fixture",
                     "--entrypoint", "/bin/rm", image, "-rf", "--",
                     "/fixture/configs", "/fixture/data", "/fixture/logs",
                     "/fixture/bootstrap-password"],
                    env=env, check=True, capture_output=True, timeout=30)
            subprocess.run(["docker", "image", "rm", image], env=env,
                           check=False, capture_output=True)


if __name__ == "__main__":
    main()
