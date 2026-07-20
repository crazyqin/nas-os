# Hello Host — community package example

Minimal **third-party** package for NAS-OS Package Surface.

## Rules

- Implement / run against the **public Host SDK only**: [`pkg/hostapi`](../../../pkg/hostapi).
- Do **not** import `nas-os/internal/*` business packages.
- `trust` must be `community` or `local` (never `system`).
- Do **not** declare `http.admin` — admin HTTP mount is system-only.
- Default `entry` is `host-sdk`: the host runs an in-process Host SDK lifecycle adapter (no Go `.so` required for this path).

## Layout

```text
hello-host/
  manifest.json   # required — discovered by packages.community_dir
  README.md
```

## Install / enable

1. Copy this directory under your community root, e.g.:

   ```bash
   mkdir -p /var/lib/nas-os/community-packages
   cp -a examples/community-packages/hello-host \
     /var/lib/nas-os/community-packages/hello-host
   ```

2. Configure nasd:

   ```yaml
   packages:
     community_dir: /var/lib/nas-os/community-packages
     enabled:
       - com.example.hello-host
   ```

3. Restart `nasd`. Discovery registers the package; `enabled` starts lifecycle.

4. Check:

   - `GET /api/v1/packages` → `community_discovered` / `loaded`
   - Marker file: `$data_dir/community-packages/com.example.hello-host/started`

## Building a richer package

Target `nas-os/pkg/hostapi`:

```go
package myplugin

import (
    "context"
    "nas-os/pkg/hostapi"
)

// Your type implements hostapi.Package and is loaded by a future
// entry type or host adapter. Today entry=host-sdk uses the host's
// built-in HostSDKPackage which only needs manifest.json.
type P struct{}

func (P) Meta() hostapi.Meta {
    return hostapi.Meta{
        ID: "com.example.mine", Trust: hostapi.TrustLocal,
        Capabilities: []hostapi.Capability{hostapi.CapHostSDK},
    }
}
func (P) Init(ctx context.Context, h hostapi.Host) error {
    h.Logf("init host_api=%s", h.APIVersion())
    return nil
}
func (P) Start(context.Context) error  { return nil }
func (P) Stop(context.Context) error   { return nil }
func (P) Health(context.Context) error { return nil }
```

See [docs/STRUCTURE.md](../../../docs/STRUCTURE.md) § Package Surface / community.
