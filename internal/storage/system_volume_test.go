package storage

import (
	"os"
	"path/filepath"
	"testing"

	"nas-os/pkg/btrfs"
)

func TestSystemRootIsExcludedFromDataPools(t *testing.T) {
	info := "20 1 0:20 / / rw - btrfs /dev/vda2 rw\n21 20 0:21 / /mnt/data rw - btrfs /dev/sdb1 rw\n"
	root := rootBtrfsDevice(info)
	if root != "/dev/vda2" {
		t.Fatalf("root device = %q", root)
	}
	if !isSystemVolume(btrfs.VolumeInfo{Name: "arbitrary-os-label", Devices: []string{"/dev/vda2", "/dev/vdb2"}}, root) {
		t.Fatal("multi-device root filesystem entered the data pool inventory")
	}
	if isSystemVolume(btrfs.VolumeInfo{Name: "NASOS-ROOT", Devices: []string{"/dev/sdb1"}}, root) {
		t.Fatal("data filesystem was excluded by label instead of device identity")
	}
}

func TestNonBtrfsRootDoesNotExcludeDataPools(t *testing.T) {
	for _, info := range []string{
		"20 1 8:2 / / rw - ext4 /dev/sda2 rw\n",
		"20 1 0:20 / / rw - overlay overlay rw\n",
		"malformed line\n",
	} {
		if root := rootBtrfsDevice(info); root != "" {
			t.Fatalf("unexpected btrfs root %q", root)
		}
	}
}

func TestSystemRootMatchesDeviceAliases(t *testing.T) {
	dir := t.TempDir()
	device := filepath.Join(dir, "device")
	alias := filepath.Join(dir, "root-alias")
	if err := os.WriteFile(device, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(device, alias); err != nil {
		t.Fatal(err)
	}
	if !isSystemVolume(btrfs.VolumeInfo{Devices: []string{device}}, alias) {
		t.Fatal("root device alias bypassed system filesystem exclusion")
	}
}
