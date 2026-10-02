#!/usr/bin/env bash
# Validate the assets consumed by scripts/install.sh, including ARMv7 (arm).
set -euo pipefail
: "${RELEASE_REPOSITORY:?required}"
: "${RELEASE_VERSION:?required}"
: "${RELEASE_SOURCE_COMMIT:?required}"
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT
cd "$work_dir"

# Authenticated API supports draft releases too; public downloads are checked
# for published releases so redirect-only success cannot hide a missing asset.
gh release view "$RELEASE_VERSION" --repo "$RELEASE_REPOSITORY" \
    --json isDraft > release.json
assets=(nasd-linux-amd64 nasd-linux-arm64 nasd-linux-arm
        checksums.txt webui.tar.gz webui.tar.gz.sha256 install.sh install.sh.sha256)
iso="nas-os-$RELEASE_VERSION-amd64.iso"
assets+=("$iso" "$iso.sha256" "$iso.source.json")
for asset in "${assets[@]}"; do
    if jq -e '.isDraft' release.json >/dev/null; then
        gh release download "$RELEASE_VERSION" --repo "$RELEASE_REPOSITORY" \
            --pattern "$asset" --dir .
    else
        curl --fail --location --retry 3 --connect-timeout 10 --max-time 180 \
            --output "$asset" \
            "https://github.com/$RELEASE_REPOSITORY/releases/download/$RELEASE_VERSION/$asset"
    fi
    test -s "$asset"
done

# Select only the expected files; never trust checksum-provided paths.
for asset in nasd-linux-amd64 nasd-linux-arm64 nasd-linux-arm; do
    awk -v name="$asset" '$2 == name {print}' checksums.txt > selected.sha256
    test "$(wc -l < selected.sha256)" -eq 1
    sha256sum --check selected.sha256
    python3 "$script_dir/verify-release-binaries.py" --os linux --arch "${asset#nasd-linux-}" "$asset"
done
for asset in webui.tar.gz install.sh; do
    awk -v name="$asset" '$2 == name {print}' "$asset.sha256" > selected.sha256
    test "$(wc -l < selected.sha256)" -eq 1
    sha256sum --check selected.sha256
done
bash -n install.sh
grep -Fx "NAS_OS_RELEASE_VERSION=\"$RELEASE_VERSION\"" install.sh
tar tzf webui.tar.gz > archive-files.txt
grep -Fx 'webui/index.html' archive-files.txt
grep -Fx 'webui/pages/login.html' archive-files.txt
python3 "$script_dir/verify-release-iso.py" --directory . --version "$RELEASE_VERSION" \
    --source-commit "$RELEASE_SOURCE_COMMIT"
echo "Release installation assets verified: $RELEASE_VERSION"
