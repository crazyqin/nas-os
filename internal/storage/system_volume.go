package storage

import (
	"path/filepath"
	"strings"

	"nas-os/pkg/btrfs"
)

// rootBtrfsDevice reads the root mount in the process's own mount namespace.
// Labels are not an identity: the OS filesystem must never enter the data pool
// inventory, where it would be treated as /mnt/<label> and offered for deletion.
func rootBtrfsDevice(mountInfo string) string {
	var device string
	for _, line := range strings.Split(mountInfo, "\n") {
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) != 2 {
			continue
		}
		mount := strings.Fields(parts[0])
		fs := strings.Fields(parts[1])
		if len(mount) >= 5 && mount[4] == "/" && len(fs) >= 2 {
			device = ""
			if fs[0] == "btrfs" {
				device = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(fs[1])
			}
		}
	}
	return device
}

func isSystemVolume(volume btrfs.VolumeInfo, rootDevice string) bool {
	if rootDevice == "" {
		return false
	}
	canonical := func(device string) string {
		if resolved, err := filepath.EvalSymlinks(device); err == nil {
			return resolved
		}
		return device
	}
	rootDevice = canonical(rootDevice)
	for _, device := range volume.Devices {
		if canonical(device) == rootDevice {
			return true
		}
	}
	return false
}
