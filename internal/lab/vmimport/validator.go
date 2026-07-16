// Package vmimport 提供虚拟机镜像导入导出功能
package vmimport

import (
	"encoding/binary"
	"fmt"
	"os"
)

// Magic Bytes 定义.
var (
	// QCOW2 魔数: QFI\xfb.
	qcow2Magic = []byte{0x51, 0x46, 0x49, 0xfb}
	// QED 魔数.
	qedMagic = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	// VDI 魔数: <<< Oracle VM VirtualBox Disk Image >>>.
	vdiMagic = []byte("<<< Oracle VM VirtualBox Disk Image >>>")
	// VHDX 魔数: vhdxfile.
	vhdxMagic = []byte{0x76, 0x68, 0x64, 0x78, 0x66, 0x69, 0x6c, 0x65}
	// VMDK 骨数: KDMV.
	vmdkMagic = []byte{0x4b, 0x44, 0x4d, 0x56}
)

// DetectFormat 检测虚拟磁盘镜像格式.
// 通过读取文件头部的 magic bytes 来判断格式.
func DetectFormat(path string) (DiskFormat, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	// 读取前512字节用于格式检测.
	header := make([]byte, 512)
	n, err := f.Read(header)
	if err != nil && n == 0 {
		return "", fmt.Errorf("读取文件头失败: %w", err)
	}
	header = header[:n]

	if len(header) < 4 {
		return FormatRAW, nil // 文件太小，默认为RAW.
	}

	// 检测 QCOW2.
	if len(header) >= 4 && matchBytes(header[:4], qcow2Magic) {
		return FormatQCOW2, nil
	}

	// 检测 VMDK.
	if len(header) >= 4 && matchBytes(header[:4], vmdkMagic) {
		return FormatVMDK, nil
	}

	// 检测 VDI.
	// VDI 文件在偏移 0x40 处有魔数.
	if len(header) >= 0x40+len(vdiMagic) {
		if matchBytes(header[0x40:0x40+len(vdiMagic)], vdiMagic) {
			return FormatVDI, nil
		}
	}

	// 检测 VHDX.
	// VHDX 文件在偏移 0 处有魔数 "vhdxfile".
	if len(header) >= len(vhdxMagic) && matchBytes(header[:len(vhdxMagic)], vhdxMagic) {
		return FormatVHDX, nil
	}

	// 检测 QED.
	// QED 格式特殊，前16字节通常为0，需要更多特征.
	// 简单检测：如果前16字节全为0且文件大小 > 64KB，可能是QED.
	if len(header) >= 20 {
		allZero := true
		for i := 0; i < 16; i++ {
			if header[i] != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			// 检查 cluster_size 字段（偏移 20-23）.
			if len(header) >= 24 {
				clusterSize := binary.LittleEndian.Uint32(header[20:24])
				// QED 的 cluster_size 通常是 64KB (65536) 或其倍数.
				if clusterSize == 65536 || clusterSize == 131072 || clusterSize == 262144 || clusterSize == 524288 {
					return FormatQED, nil
				}
			}
		}
	}

	// 默认为 RAW 格式.
	return FormatRAW, nil
}

// ValidateImage 验证镜像文件.
// 检查文件是否为有效的虚拟磁盘镜像.
func ValidateImage(path string) (*ValidateResult, error) {
	// 检查文件是否存在.
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ValidateResult{
				Valid:        false,
				ErrorMessage: "文件不存在",
			}, nil
		}
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 检查文件大小.
	if info.Size() == 0 {
		return &ValidateResult{
			Valid:        false,
			ErrorMessage: "文件为空",
		}, nil
	}

	// 检测格式.
	format, err := DetectFormat(path)
	if err != nil {
		return &ValidateResult{
			Valid:        false,
			ErrorMessage: fmt.Sprintf("格式检测失败: %v", err),
		}, nil
	}

	// 获取虚拟大小.
	virtualSize := info.Size()
	if format != FormatRAW {
		if vs, vsErr := GetVirtualSize(path); vsErr == nil && vs > 0 {
			virtualSize = vs
		}
	}

	return &ValidateResult{
		Valid:       true,
		Format:      format,
		VirtualSize: virtualSize,
		FileSize:    info.Size(),
	}, nil
}

// matchBytes 比较两个字节切片是否匹配.
func matchBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
