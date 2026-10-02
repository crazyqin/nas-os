"""Offline release orchestration regressions; no GitHub writes or Docker needed."""

import os
from pathlib import Path
import re
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[1]


def job(source, name):
    return re.split(r"\n  [a-z][a-z0-9-]*:\n", source.split("  " + name + ":\n", 1)[1], 1)[0]


def step(source, name):
    return source.split("      - name: " + name + "\n", 1)[1].split("\n      - name:", 1)[0]


def run_script(source, name):
    body = re.search(r"(?m)^        run: \|\n((?:^          [^\n]*\n|^[ \t]*\n)*)",
                     step(source, name)).group(1)
    return textwrap.dedent(body)


def condition(source):
    return re.search(r"^    if: (.+)$", source, re.M).group(1)


def evaluate(expression, values):
    expression = expression.strip().removeprefix("${{").removesuffix("}}")
    for key, value in sorted(values.items(), key=lambda item: -len(item[0])):
        if key == "cancelled":
            continue
        expression = expression.replace(key, repr(value))
    expression = expression.replace("&&", " and ").replace("||", " or ")
    expression = re.sub(r"!(?!=)", " not ", expression)
    return bool(eval(expression, {"__builtins__": {}}, {
        "startsWith": lambda value, prefix: value.startswith(prefix),
        "always": lambda: True, "cancelled": lambda: values.get("cancelled", False),
    }))


class ReleasePipelineTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.release = (ROOT / ".github/workflows/release.yml").read_text()
        cls.docker = (ROOT / ".github/workflows/docker-publish.yml").read_text()
        cls.staged = (ROOT / ".github/workflows/staged-release.yml").read_text()

    def test_release_is_draft_until_all_required_jobs_succeed(self):
        create = job(self.release, "create-release")
        self.assertIn("          draft: true\n", create)
        self.assertIn("          make_latest: false\n", create)
        trigger = job(self.release, "trigger-docker")
        self.assertIn("needs: [prepare-release, verify-release]", trigger)
        self.assertIn("uses: ./.github/workflows/docker-publish.yml", trigger)
        self.assertIn("version: ${{ needs.prepare-release.outputs.version }}", trigger)
        self.assertIn("secrets: inherit", trigger)
        self.assertNotIn("benc-uk/workflow-dispatch", self.release)
        publish = job(self.release, "publish-release")
        self.assertIn("needs: [prepare-release, verify-release, trigger-docker]", publish)
        for assets in ("success", "failure", "cancelled", "skipped"):
            for docker in ("success", "failure", "cancelled", "skipped"):
                for draft in ("true", "false"):
                    for cancelled in (True, False):
                        values = {
                            "needs.verify-release.result": assets,
                            "needs.trigger-docker.result": docker,
                            "needs.prepare-release.outputs.is-prerelease": "false",
                            "github.event.inputs.draft": draft, "cancelled": cancelled,
                        }
                        with self.subTest(**values):
                            expected = assets == docker == "success" and draft == "false" and not cancelled
                            self.assertEqual(evaluate(condition(publish), values), expected)

    def test_versioned_images_are_not_skipped_by_empty_path_diff(self):
        output = re.search(r"^      docker-changed: (.+)$", self.docker, re.M).group(1)
        values = {"github.ref": "refs/tags/v3.24.7", "github.event_name": "push",
                  "steps.filter.outputs.docker": "false"}
        self.assertTrue(evaluate(output, values))
        values["needs.changes.outputs.docker-changed"] = "true"
        for name in ("build-platform", "merge-manifest", "sign-image", "docker-scan"):
            with self.subTest(job=name):
                self.assertTrue(evaluate(condition(job(self.docker, name)), values))
        values["github.ref"] = "refs/heads/master"
        self.assertFalse(evaluate(output, values))
        values["github.event_name"] = "workflow_dispatch"
        self.assertTrue(evaluate(output, values))

    def test_formal_architecture_and_security_execution_failures_are_required(self):
        platform = job(self.docker, "build-platform")
        expr = re.search(r"^    continue-on-error: (.+)$", platform, re.M).group(1)
        self.assertFalse(evaluate(expr, {"matrix.platform": "armv7", "github.ref": "refs/tags/v3.24.7"}))
        self.assertTrue(evaluate(expr, {"matrix.platform": "armv7", "github.ref": "refs/heads/master"}))
        for name in ("签名镜像 (Keyless)", "运行 Trivy 扫描"):
            expr = re.search(r"^        continue-on-error: (.+)$", step(self.docker, name), re.M).group(1)
            self.assertFalse(evaluate(expr, {"github.ref": "refs/tags/v3.24.7"}))

    def test_exact_tag_alias_and_single_automatic_release_path(self):
        self.assertIn("type=raw,value=${{ github.ref_name }},enable=${{ startsWith(github.ref, 'refs/tags/') }}", self.docker)
        self.assertIn("  workflow_call:\n", self.docker)
        self.assertNotIn("tags:", self.docker.split("on:\n", 1)[1].split("  pull_request:", 1)[0])
        self.assertNotIn("  push:", self.staged.split("on:\n", 1)[1].split("\nenv:", 1)[0])
        self.assertIn("ref: ${{ needs.prepare-release.outputs.version }}", job(self.release, "verify-release"))

    def test_release_targets_are_explicit_and_verified_before_upload(self):
        build = step(self.release, "构建二进制文件")
        self.assertIn("GOOS: ${{ matrix.os }}", build)
        self.assertIn("GOARCH: ${{ matrix.arch }}", build)
        self.assertIn('verify-release-binaries.py --os "$GOOS" --arch "$GOARCH" "$BINARY_NAME" "$CTL_NAME"', build)
        assets = (ROOT / "scripts/verify-release-assets.sh").read_text()
        self.assertIn('verify-release-binaries.py" --os linux --arch "${asset#nasd-linux-}" "$asset"', assets)

    def test_formal_image_verification_uses_release_alias_digest_and_revision(self):
        verify = job(self.docker, "verify-release-image")
        self.assertTrue(evaluate(condition(verify), {"github.ref": "refs/tags/v3.24.7", "github.event_name": "push"}))
        self.assertFalse(evaluate(condition(verify), {"github.ref": "refs/heads/master", "github.event_name": "push"}))
        self.assertIn("needs: [merge-manifest]", verify)
        self.assertIn("RELEASE_IMAGE: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ github.ref_name }}", verify)
        self.assertIn("EXPECTED_DIGEST: ${{ needs.merge-manifest.outputs.digest }}", verify)
        script = run_script(verify, "验证已发布的精确版本镜像")
        self.assertIn('"$EXPECTED_DIGEST"', script)
        self.assertIn('contains(["amd64", "arm64", "arm/v7"])', script)
        self.assertIn('"$GITHUB_SHA"', script)
        self.assertIn('"$GITHUB_REF_NAME"', script)

    def test_real_git_source_guards_reject_branch_wrong_tag_and_stale_sha(self):
        scripts = [run_script(self.docker, "校验发布 tag 与输入版本"),
                   run_script(self.release, "确定版本")]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            def git(*args):
                return subprocess.run(["git", *args], cwd=root, text=True, check=True,
                                      capture_output=True).stdout.strip()
            git("init", "--quiet")
            for message in ("release", "later"):
                git("-c", "user.name=Regression", "-c", "user.email=regression@localhost",
                    "-c", "commit.gpgsign=false", "commit", "--allow-empty", "--quiet", "-m", message)
                git("tag", "v3.24.7" if message == "release" else "v3.24.8")
            first, second = git("rev-parse", "v3.24.7"), git("rev-parse", "v3.24.8")
            git("tag", "not-semver", first)
            cases = [
                ("push", "refs/tags/v3.24.7", "", first, first, True),
                ("workflow_dispatch", "refs/tags/v3.24.7", "v3.24.7", first, first, True),
                ("workflow_dispatch", "refs/heads/master", "v3.24.7", first, first, False),
                ("workflow_dispatch", "refs/tags/v3.24.8", "v3.24.7", first, first, False),
                ("push", "refs/tags/v3.24.7", "", second, second, False),
                ("push", "refs/tags/v3.24.7", "", first, second, False),
                ("push", "refs/tags/not-semver", "", first, first, False),
            ]
            for event, ref, requested, head, sha, expected in cases:
                git("checkout", "--quiet", "--detach", head)
                for script in scripts:
                    with self.subTest(event=event, ref=ref, requested=requested, head=head, sha=sha):
                        script = script.replace("${{ github.event_name }}", event)
                        result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", script], cwd=root,
                                                env={**os.environ, "GITHUB_REF": ref, "GITHUB_SHA": sha,
                                                     "REQUESTED_VERSION": requested, "REQUESTED_PRERELEASE": "false",
                                                     "GITHUB_OUTPUT": str(root / "output")},
                                                capture_output=True, text=True)
                        self.assertEqual(result.returncode == 0, expected, result.stdout + result.stderr)

    def test_promotion_commands_never_mark_prerelease_latest(self):
        script = run_script(self.release, "发布已验证的草稿")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "gh").write_text('#!/bin/bash\nprintf "%s\\n" "$@" > "$CALLS"\n')
            (root / "gh").chmod(0o755)
            for prerelease in ("true", "false"):
                result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", script],
                                        env={**os.environ, "PATH": str(root) + ":" + os.environ["PATH"],
                                             "CALLS": str(root / "calls"), "IS_PRERELEASE": prerelease,
                                             "RELEASE_VERSION": "v3.24.7", "RELEASE_REPOSITORY": "crazyqin/nas-os"},
                                        capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                args = (root / "calls").read_text().splitlines()
                self.assertIn("--draft=false", args)
                self.assertIn("--latest=false" if prerelease == "true" else "--latest", args)

    def test_all_embedded_shell_blocks_parse(self):
        for source in (self.release, self.docker, self.staged):
            for block in re.findall(r"(?m)^        run: \|\n((?:^          [^\n]*\n|^[ \t]*\n)*)", source):
                script = re.sub(r"\$\{\{.*?\}\}", "placeholder", textwrap.dedent(block))
                result = subprocess.run(["bash", "-n"], input=script, text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
