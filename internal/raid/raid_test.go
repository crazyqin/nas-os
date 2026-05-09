package raid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)
	assert.Empty(t, m.ListArrays())
}

func TestCreateArray(t *testing.T) {
	m := NewManager()

	// 正常创建
	err := m.CreateArray("test1", RAID1, []string{"/dev/sda", "/dev/sdb"}, nil, "64K")
	require.NoError(t, err)

	arr, err := m.GetArray("test1")
	require.NoError(t, err)
	assert.Equal(t, "test1", arr.Name)
	assert.Equal(t, RAID1, arr.Level)
	assert.Equal(t, "active", arr.Status)
	assert.Len(t, arr.Devices, 2)
}

func TestCreateArrayDuplicate(t *testing.T) {
	m := NewManager()

	err := m.CreateArray("dup", RAID0, []string{"/dev/sda", "/dev/sdb"}, nil, "64K")
	require.NoError(t, err)

	err = m.CreateArray("dup", RAID0, []string{"/dev/sdc", "/dev/sdd"}, nil, "64K")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "阵列已存在")
}

func TestCreateArrayInvalidLevel(t *testing.T) {
	m := NewManager()

	err := m.CreateArray("bad", "RAID99", []string{"/dev/sda", "/dev/sdb"}, nil, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的 RAID 级别")
}

func TestCreateArrayInsufficientDevices(t *testing.T) {
	m := NewManager()

	err := m.CreateArray("few", RAID1, []string{"/dev/sda"}, nil, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "至少需要 2 个设备")
}

func TestDeleteArray(t *testing.T) {
	m := NewManager()

	err := m.CreateArray("del", RAID1, []string{"/dev/sda", "/dev/sdb"}, nil, "64K")
	require.NoError(t, err)

	err = m.DeleteArray("del")
	require.NoError(t, err)

	_, err = m.GetArray("del")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "阵列不存在")
}

func TestDeleteArrayNotFound(t *testing.T) {
	m := NewManager()

	err := m.DeleteArray("nonexistent")
	assert.Error(t, err)
}

func TestListArrays(t *testing.T) {
	m := NewManager()

	_ = m.CreateArray("a1", RAID1, []string{"/dev/sda", "/dev/sdb"}, nil, "")
	_ = m.CreateArray("a2", RAID5, []string{"/dev/sdc", "/dev/sdd", "/dev/sde"}, nil, "")

	arrays := m.ListArrays()
	assert.Len(t, arrays, 2)
}

func TestAddAndRemoveSpare(t *testing.T) {
	m := NewManager()

	_ = m.CreateArray("sp", RAID5, []string{"/dev/sda", "/dev/sdb", "/dev/sdc"}, nil, "")

	err := m.AddSpare("sp", "/dev/sdd")
	require.NoError(t, err)

	arr, _ := m.GetArray("sp")
	assert.Len(t, arr.SpareDevices, 1)
	assert.Equal(t, "/dev/sdd", arr.SpareDevices[0])

	// 重复添加
	err = m.AddSpare("sp", "/dev/sdd")
	assert.Error(t, err)

	// 移除
	err = m.RemoveSpare("sp", "/dev/sdd")
	require.NoError(t, err)

	arr, _ = m.GetArray("sp")
	assert.Empty(t, arr.SpareDevices)

	// 移除不存在的
	err = m.RemoveSpare("sp", "/dev/sdd")
	assert.Error(t, err)
}

func TestRebuildArray(t *testing.T) {
	m := NewManager()

	_ = m.CreateArray("rb", RAID1, []string{"/dev/sda", "/dev/sdb"}, nil, "")

	err := m.RebuildArray("rb")
	require.NoError(t, err)

	arr, _ := m.GetArray("rb")
	assert.Equal(t, "rebuilding", arr.Status)
}

func TestExpandArray(t *testing.T) {
	m := NewManager()

	_ = m.CreateArray("ex", RAID5, []string{"/dev/sda", "/dev/sdb", "/dev/sdc"}, nil, "")

	err := m.ExpandArray("ex", []string{"/dev/sdd"})
	require.NoError(t, err)

	arr, _ := m.GetArray("ex")
	assert.Len(t, arr.Devices, 4)

	// 扩展重复设备
	err = m.ExpandArray("ex", []string{"/dev/sda"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "设备已在阵列中")
}

func TestGetArrayStatus(t *testing.T) {
	m := NewManager()

	_ = m.CreateArray("st", RAID6, []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd"}, []string{"/dev/sde"}, "128K")

	status, err := m.GetArrayStatus("st")
	require.NoError(t, err)
	assert.Equal(t, "st", status["name"])
	assert.Equal(t, RAID6, status["level"])
	assert.Equal(t, "active", status["status"])
	assert.Equal(t, 4, status["device_count"])
	assert.Equal(t, 1, status["spare_count"])
}

func TestGetArrayStatusNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetArrayStatus("nonexistent")
	assert.Error(t, err)
}

func TestAllRAIDLevels(t *testing.T) {
	m := NewManager()
	levels := []string{RAID0, RAID1, RAID5, RAID6, RAID10, RAIDZ1, RAIDZ2, RAIDZ3, DRAID1, DRAID2, DRAID3}

	for _, level := range levels {
		name := "arr_" + level
		err := m.CreateArray(name, level, []string{"/dev/sda", "/dev/sdb"}, nil, "")
		assert.NoError(t, err, "level %s should be valid", level)
	}

	assert.Len(t, m.ListArrays(), len(levels))
}
