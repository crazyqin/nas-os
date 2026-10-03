"""Offline release orchestration regressions; no GitHub writes or Docker needed."""

import os
import json
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
        cls.tag = (ROOT / ".github/workflows/create-release-tag.yml").read_text()

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

    def test_iso_acceptance_is_required_before_release_creation(self):
        build = job(self.release, "build-iso")
        self.assertIn("uses: ./.github/workflows/iso-build.yml", build)
        self.assertIn("source_commit: ${{ needs.prepare-release.outputs.source-commit }}", build)
        self.assertIn("release_version: ${{ needs.prepare-release.outputs.version }}", build)
        self.assertNotIn("include_arm64", build)
        create = job(self.release, "create-release")
        needs = re.search(r"needs: \[(.+)\]", create).group(1).split(", ")
        self.assertIn("build-iso", needs)
        self.assertLess(create.index("上传前验证 ISO"), create.index("softprops/action-gh-release"))
        self.assertIn("name: nas-os-iso-amd64", create)
        for suffix in ("", ".sha256", ".source.json"):
            self.assertIn("release-iso/nas-os-*-amd64.iso" + suffix + "\n", create)
        self.assertNotIn("release-iso/*.iso", create)
        verify = job(self.release, "verify-release")
        self.assertIn("RELEASE_SOURCE_COMMIT: ${{ needs.prepare-release.outputs.source-commit }}", verify)

    def test_iso_source_guard_rejects_wrong_version_tag_and_commit(self):
        iso = (ROOT / ".github/workflows/iso-build.yml").read_text()
        script = run_script(iso, "Check immutable ISO source")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            def git(*args):
                return subprocess.run(["git", *args], cwd=root, text=True, check=True,
                                      capture_output=True).stdout.strip()
            git("init", "--quiet")
            (root / "VERSION").write_text("v3.25.0\n")
            git("add", "VERSION")
            git("-c", "user.name=Regression", "-c", "user.email=regression@localhost",
                "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "ISO source")
            commit = git("rev-parse", "HEAD")
            git("tag", "v3.25.0")
            git("tag", "v3.24.8")
            cases = [("refs/tags/v3.25.0", "v3.25.0", commit, True),
                     ("refs/heads/master", "v3.25.0", commit, False),
                     ("refs/tags/v3.24.8", "v3.25.0", commit, False),
                     ("refs/tags/v3.24.8", "v3.24.8", commit, False),
                     ("refs/tags/v3.25.0", "v3.25.0", "b" * 40, False),
                     ("refs/heads/master", "", commit, True)]
            for ref, version, expected_source, expected in cases:
                with self.subTest(ref=ref, version=version, source=expected_source):
                    result = subprocess.run(["bash", "-c", script], cwd=root,
                        env={**os.environ, "GITHUB_REF": ref, "RELEASE_VERSION": version,
                             "EXPECTED_SOURCE": expected_source}, capture_output=True, text=True)
                    self.assertEqual(result.returncode == 0, expected, result.stderr)

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
                (root / "VERSION").write_text(ref.removeprefix("refs/tags/") + "\n")
                for script in scripts:
                    with self.subTest(event=event, ref=ref, requested=requested, head=head, sha=sha):
                        script = script.replace("${{ github.event_name }}", event)
                        result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", script], cwd=root,
                                                env={**os.environ, "GITHUB_REF": ref, "GITHUB_SHA": sha,
                                                     "REQUESTED_VERSION": requested, "REQUESTED_PRERELEASE": "false",
                                                     "GITHUB_OUTPUT": str(root / "output")},
                                                capture_output=True, text=True)
                        self.assertEqual(result.returncode == 0, expected, result.stdout + result.stderr)

            git("checkout", "--quiet", "--detach", first)
            (root / "VERSION").write_text("v3.24.8\n")
            script = scripts[1].replace("${{ github.event_name }}", "push")
            result = subprocess.run(["bash", "-c", script], cwd=root,
                env={**os.environ, "GITHUB_REF": "refs/tags/v3.24.7", "GITHUB_SHA": first,
                     "REQUESTED_VERSION": "", "REQUESTED_PRERELEASE": "false",
                     "GITHUB_OUTPUT": str(root / "output")}, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0, "A tag must match the source VERSION")

    def test_public_download_verification_runs_only_after_publication(self):
        public = job(self.release, "verify-public-release")
        self.assertIn("needs: [prepare-release, publish-release]", public)
        self.assertNotIn("always()", public)
        self.assertIn("REQUIRE_PUBLIC_RELEASE: 'true'", public)
        self.assertIn("ref: ${{ needs.prepare-release.outputs.source-commit }}", public)
        self.assertIn("bash scripts/verify-release-assets.sh", public)

    def test_generated_notes_preserve_current_source_changelog_section(self):
        script = run_script(self.release, "生成变更日志")
        script = script.replace("${{ needs.prepare-release.outputs.version }}", "v3.25.0")
        script = script.replace("${{ needs.prepare-release.outputs.previous-tag }}", "")
        script = script.replace("${{ github.repository }}", "crazyqin/nas-os")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            original = "# Changelog\n\n## v3.25.0 - 2026-10-03\n\n### Added\n- ISO current change\n\n## v3.24.8 - older\n- Old change\n"
            (root / "CHANGELOG.md").write_text(original)
            result = subprocess.run(["bash", "-c", script], cwd=root,
                env={**os.environ, "GITHUB_OUTPUT": str(root / "output")},
                capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            notes = (root / "CHANGELOG.md").read_text()
            self.assertIn("ISO current change", notes)
            self.assertNotIn("Old change", notes)
            self.assertEqual((root / "SOURCE_CHANGELOG.md").read_text(), original)

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
        for source in (self.release, self.docker, self.staged, self.tag):
            for block in re.findall(r"(?m)^        run: \|\n((?:^          [^\n]*\n|^[ \t]*\n)*)", source):
                script = re.sub(r"\$\{\{.*?\}\}", "placeholder", textwrap.dedent(block))
                result = subprocess.run(["bash", "-n"], input=script, text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)

    def test_tag_creation_rejects_wrong_source_and_never_moves_existing_tags(self):
        script = run_script(self.tag, "Verify source and create immutable tag")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            def git(*args):
                return subprocess.run(["git", *args], cwd=root, text=True, check=True,
                                      capture_output=True).stdout.strip()
            git("init", "--quiet")
            (root / "VERSION").write_text("v3.25.0\n")
            git("add", "VERSION")
            git("-c", "user.name=Regression", "-c", "user.email=regression@localhost",
                "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "release")
            source = git("rev-parse", "HEAD")
            mock = root / "gh"
            mock.write_text('''#!/usr/bin/env python3
import json, os, sys
with open(os.environ["CALLS"], "a") as stream:
    stream.write(" ".join(sys.argv[1:]) + "\\n")
if "matching-refs" in sys.argv[2]:
    print(os.environ["TAG_MATCHES"])
elif "--method" in sys.argv:
    print("{}")
else:
    print(os.environ["RELEASE_SOURCE_COMMIT"])
''')
            mock.chmod(0o755)
            ref = "refs/tags/v3.25.0"
            matching = [{"ref": ref, "object": {"type": "commit", "sha": source}}]
            conflicting = [{"ref": ref, "object": {"type": "commit", "sha": "b" * 40}}]
            prefix = [{"ref": ref + "-rc.1", "object": {"type": "commit", "sha": "b" * 40}}]
            cases = [
                ("refs/heads/master", "v3.25.0", source, [], True, True),
                ("refs/heads/master", "v3.25.0", source, matching, True, False),
                ("refs/heads/master", "v3.25.0", source, prefix, True, True),
                ("refs/heads/master", "v3.25.0", source, conflicting, False, False),
                ("refs/heads/other", "v3.25.0", source, [], False, False),
                ("refs/heads/master", "v3.24.8", source, [], False, False),
                ("refs/heads/master", "v3.25.0", "b" * 40, [], False, False),
                ("refs/heads/master", "v3.25.0", source[:7], [], False, False),
                ("refs/heads/master", "v3.25.0; echo invalid", source, [], False, False),
            ]
            calls = root / "calls"
            for github_ref, version, requested, matches, success, created in cases:
                calls.unlink(missing_ok=True)
                with self.subTest(ref=github_ref, version=version, source=requested, matches=matches):
                    result = subprocess.run(["bash", "-c", script], cwd=root,
                        env={**os.environ, "PATH": str(root) + ":" + os.environ["PATH"],
                             "CALLS": str(calls), "TAG_MATCHES": json.dumps(matches),
                             "GITHUB_REF": github_ref, "GITHUB_SHA": source,
                             "RELEASE_VERSION": version, "RELEASE_SOURCE_COMMIT": requested,
                             "RELEASE_REPOSITORY": "crazyqin/nas-os",
                             "GITHUB_STEP_SUMMARY": str(root / "summary")},
                        capture_output=True, text=True)
                    self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
                    commands = calls.read_text() if calls.exists() else ""
                    self.assertEqual("--method POST" in commands, created)
                    self.assertNotIn("--method PATCH", commands)
                    self.assertNotIn("--method DELETE", commands)


if __name__ == "__main__":
    unittest.main()
