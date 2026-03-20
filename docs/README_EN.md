# NAS-OS 🖥️

[中文](../README.md) | **English**

A Go-based home NAS system with btrfs storage management, SMB/NFS sharing, and web management interface.

> **Latest Version**: v2.253.75 Stable (2026-03-20)
> **CI/CD**: [![CI/CD](https://github.com/crazyqin/nas-os/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/crazyqin/nas-os/actions)
> **Docker**: [![Docker](https://img.shields.io/docker/v/ghcr.io/crazyqin/nas-os/v2.253.75?label=docker)](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

## Features

### Core Functions ✅

| Module | Description | Status |
|--------|-------------|--------|
| 💾 btrfs Storage | Volume/Subvolume/Snapshot/RAID | ✅ Complete |
| 🌐 Web Interface | Responsive design/Mobile support | ✅ Complete |
| 📁 File Sharing | SMB/NFS/RBAC | ✅ Complete |
| 👥 User Permissions | RBAC/MFA/Audit | ✅ Complete |
| 📊 Monitoring & Alerts | Real-time metrics/Multi-channel notifications | ✅ Complete |
| 🔒 Security & Auth | JWT/RBAC/Encryption | ✅ Complete |
| 🐳 Docker | Multi-architecture images | ✅ Complete |
| ⚡ Performance Optimization | LRU cache/GC tuning/Worker pool | ✅ Complete |
| 🛡️ Cluster Support | Multi-node/Load balancing | ✅ Complete |

## Quick Start

### Option 1: Download Binary (Recommended)

```bash
# Download (choose your architecture)
# AMD64 (x86_64)
wget https://github.com/crazyqin/nas-os/releases/download/v2.253.69/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/crazyqin/nas-os/releases/download/v2.253.69/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7 (Raspberry Pi 3, older ARM)
wget https://github.com/crazyqin/nas-os/releases/download/v2.253.69/nasd-linux-armv7
chmod +x nasd-linux-armv7
sudo mv nasd-linux-armv7 /usr/local/bin/nasd

# Verify installation
nasd --version
```

### Option 2: Docker Deployment

```bash
# Pull image
docker pull ghcr.io/crazyqin/nas-os:v2.253.69

# Run container
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
ghcr.io/crazyqin/nas-os:v2.253.69

# View logs
docker logs -f nasd
```

### Option 3: Build from Source

#### Prerequisites

```bash
# Install Go 1.26.1+
# Install btrfs tools
sudo apt install btrfs-progs

# Install Samba (for SMB sharing)
sudo apt install samba

# Install NFS (for NFS sharing)
sudo apt install nfs-kernel-server
```

#### Build

```bash
cd nas-os
go mod tidy
go build -o nasd ./cmd/nasd
go build -o nasctl ./cmd/nasctl
```

### Run

```bash
# Requires root privileges (to access disk devices)
sudo nasd
```

Access http://localhost:8080

**Default Credentials**:
- Username: `admin`
- Password: `admin123`

⚠️ **Please change the default password immediately after first login!**

## API Reference

### Storage Management
| Method | Path | Description |
|--------|------|-------------|
| GET | /api/v1/volumes | List volumes |
| POST | /api/v1/volumes | Create volume |
| GET | /api/v1/volumes/:name | Get volume details |
| DELETE | /api/v1/volumes/:name | Delete volume |
| POST | /api/v1/volumes/:name/subvolumes | Create subvolume |
| POST | /api/v1/volumes/:name/snapshots | Create snapshot |
| POST | /api/v1/volumes/:name/balance | Balance data |
| POST | /api/v1/volumes/:name/scrub | Data scrubbing |

### Share Management
| Method | Path | Description |
|--------|------|-------------|
| GET | /api/v1/shares | List shares |
| POST | /api/v1/shares/smb | Create SMB share |
| POST | /api/v1/shares/nfs | Create NFS share |
| DELETE | /api/v1/shares/:id | Delete share |
| PUT | /api/v1/shares/:id | Update share config |

### Configuration Management
| Method | Path | Description |
|--------|------|-------------|
| GET | /api/v1/config | Get configuration |
| PUT | /api/v1/config | Update configuration |
| POST | /api/v1/config/reload | Reload configuration |

For complete API documentation, see [docs/API_GUIDE.md](API_GUIDE.md)

## Project Structure

```
nas-os/
├── cmd/           # Executable entry points
├── internal/      # Internal modules
│   ├── storage/   # Storage management
│   ├── web/       # Web service
│   ├── smb/       # SMB sharing
│   ├── nfs/       # NFS sharing
│   └── users/     # User management
├── pkg/           # Public libraries
├── webui/         # Frontend interface
└── configs/       # Configuration files
```

## Development Roadmap

See [MILESTONES.md](../MILESTONES.md) for detailed milestones.

### Version Roadmap

| Version | Type | Release Date | Core Features | Status |
|---------|------|--------------|---------------|--------|
| v2.76.0 | Stable | 2026-03-16 | Six ministries collaboration/Documentation/Security audit | ✅ Released |
| v2.71.0 | Stable | 2026-03-16 | Test type fixes/Version sync | ✅ Released |
| v2.70.0 | Stable | 2026-03-16 | Brand upgrade/Documentation system | ✅ Released |
| v2.68.0 | Stable | 2026-03-16 | Test coverage/API stability/CI/CD enhancement | ✅ Released |
| v2.27.0 | Stable | 2026-03-16 | Media service/Quota auto-expand/Monitoring enhancement | ✅ Released |
| v2.26.0 | Stable | 2026-03-16 | Network diagnostics/Docker enhancement/Automation | ✅ Released |
| v2.3.0 | Stable | 2026-03-28 | Storage tiering/FTP-SFTP/Compression/File tags | ✅ Released |
| v2.2.0 | Stable | 2026-03-21 | iSCSI/Snapshot policy/Dashboard enhancement | ✅ Released |

## Deployment

### Docker Deployment
```bash
# Quick start (development/testing)
docker-compose up -d

# View logs
docker-compose logs -f
```

### Bare Metal Installation
```bash
# One-click installation script
curl -fsSL https://raw.githubusercontent.com/your-org/nas-os/main/scripts/install.sh | sudo bash

# Or manual installation
sudo ./scripts/install.sh
```

### System Service
```bash
systemctl status nas-os
systemctl restart nas-os
journalctl -u nas-os -f
```

## Quick Usage

### 1. Create Storage Volume
```bash
sudo nasctl volume create mydata --path /dev/sda1
```

### 2. Create SMB Share
```bash
sudo nasctl share create smb public --path /data/public --guest
```

### 3. Create NFS Share
```bash
sudo nasctl share create nfs backup --path /data/backup --network 192.168.1.0/24
```

### 4. Access from Client
- **Windows**: `\\<server-ip>\public`
- **macOS**: `smb://<server-ip>/public`
- **Linux (NFS)**: `sudo mount <server-ip>:/backup /mnt/local_backup`

## Getting Help

- 📖 **Documentation**: [docs/](./) directory
- 🐛 **Report Issues**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- 💬 **Community Discussions**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)
- 📦 **Docker Images**: [GHCR](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines.

## License

MIT