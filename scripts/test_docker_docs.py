"""Check the documented stable Docker path without Docker or network access.

Examples must pull and run the same verified release image, with matching
source/configuration and no implicit local builds or moving development tags.
"""

from pathlib import Path
import re
import shlex
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[1]
RELEASE_VERSION = "v3.25.0"
RELEASE_IMAGE = "ghcr.io/crazyqin/nas-os:" + RELEASE_VERSION


def section(document, heading):
    return document.split(heading + "\n", 1)[1].split("\n### ", 1)[0]


def commands(document):
    for block in re.findall(r"```bash\n(.*?)```", document, re.S):
        for line in block.replace("\\\n", " ").splitlines():
            tokens = shlex.split(line, comments=True)
            if tokens and tokens[0] == "docker":
                yield tokens


def check_examples(quick, deployment):
    assert "export NAS_OS_IMAGE=" + RELEASE_IMAGE in deployment, \
        "Stable deployment must override inherited image names from the host or .env"
    assert "git checkout --detach " + RELEASE_VERSION in deployment, \
        "Compose source/configuration must use the same release tag as the image"
    quick_commands = list(commands(quick))
    pulls = [c for c in quick_commands if c[1] == "pull"]
    assert pulls == [["docker", "pull", RELEASE_IMAGE]], \
        "Pull must use the verified stable release image"
    pull = pulls[0]
    run = next(c for c in quick_commands if c[1] == "run")
    assert run[-1] == RELEASE_IMAGE and quick_commands.index(pull) < quick_commands.index(run), \
        "Run must use the same stable image explicitly pulled first"
    assert "--pull=never" in run, "Run must use the explicitly pulled image"
    assert "127.0.0.1:8080:8080" in run and "NAS_OS_LISTEN_HOST=0.0.0.0" in run
    assert "nas-os-config:/etc/nas-os" in run and "nas-os-data:/var/lib/nas-os" in run
    compose_commands = list(commands(deployment))
    compose_pulls = [c for c in compose_commands if c[1] == "compose" and "pull" in c]
    assert compose_pulls == [["docker", "compose", "pull", "nas-os"]], \
        "Compose must explicitly pull the selected release image"
    compose_up = [c for c in compose_commands if c[1] == "compose" and "up" in c]
    assert len(compose_up) == 3, "Check default, secure and privileged Compose examples"
    for command in compose_up:
        assert "--no-build" in command and "--build" not in command, \
            "Stable deployment must not replace the released image with a local build"
        assert compose_commands.index(compose_pulls[0]) < compose_commands.index(command), \
            "Compose must pull before starting the stable image"
        assert command[command.index("--pull") + 1] == "never", \
            "Compose must use the explicitly pulled release image"


class DockerDocsTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.readme = (ROOT / "README.md").read_text()
        cls.quick = section(cls.readme, "### 方式二：Docker 部署")
        cls.deployment = section(cls.readme, "### Docker 部署")

    def test_stable_release_examples_and_shell_syntax(self):
        check_examples(self.quick, self.deployment)
        for block in re.findall(r"```bash\n(.*?)```", self.quick + self.deployment, re.S):
            result = subprocess.run(["bash", "-n"], input=block, text=True,
                                    capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_only_verified_stable_remote_image_commands(self):
        # Scan all executable README examples, including snippets outside Docker headings.
        code = "\n".join(re.findall(r"```(?:bash|sh)\n(.*?)```", self.readme, re.S))
        images = re.findall(r"ghcr\.io/crazyqin/nas-os(?::|@)[^\s\"']+", code)
        self.assertTrue(images, "Stable image commands must be documented")
        self.assertEqual(set(images), {RELEASE_IMAGE})

    def test_compose_uses_local_default_with_release_override(self):
        compose = (ROOT / "docker-compose.yml").read_text()
        image = re.search(r"^    image: (.+)$", compose, re.M).group(1)
        self.assertEqual(image, "${NAS_OS_IMAGE:-nas-os:local}")
        self.assertRegex(compose, r"build:\n      context: \.\n      dockerfile: Dockerfile")

    def test_regression_rejects_version_drift_and_implicit_image_reuse(self):
        for image in ("ghcr.io/crazyqin/nas-os:v3.24.6",
                      "ghcr.io/crazyqin/nas-os:latest",
                      "ghcr.io/crazyqin/nas-os:master",
                      "ghcr.io/crazyqin/nas-os:pr-54",
                      "ghcr.io/crazyqin/nas-os:v99.0.0"):
            with self.subTest(image=image), self.assertRaises(AssertionError):
                check_examples(self.quick.replace("  " + RELEASE_IMAGE + "\n", "  " + image + "\n"),
                               self.deployment)
        with self.assertRaises(AssertionError):
            check_examples(self.quick.replace("--pull=never", ""), self.deployment)
        with self.assertRaises(AssertionError):
            check_examples(self.quick, self.deployment.replace("export NAS_OS_IMAGE=" + RELEASE_IMAGE, ""))
        with self.assertRaises(AssertionError):
            check_examples(self.quick, self.deployment.replace("git checkout --detach " + RELEASE_VERSION, ""))
        with self.assertRaises(AssertionError):
            check_examples(self.quick.replace("docker pull " + RELEASE_IMAGE, ""), self.deployment)
        with self.assertRaises(AssertionError):
            check_examples(self.quick, self.deployment.replace("docker compose pull nas-os", ""))
        for option in (" --no-build", " --pull never"):
            with self.subTest(option=option), self.assertRaises((AssertionError, ValueError)):
                check_examples(self.quick, self.deployment.replace(option, ""))


if __name__ == "__main__":
    unittest.main()
