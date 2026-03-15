package web

import (
	"testing"
)

// ========== 通用响应结构测试 ==========

func TestResponse_Struct(t *testing.T) {
	resp := Response{
		Code:    0,
		Message: "success",
		Data:    map[string]string{"key": "value"},
	}

	if resp.Code != 0 {
		t.Errorf("Expected Code=0, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("Expected Message=success, got %s", resp.Message)
	}
}

func TestErrorResponse_Struct(t *testing.T) {
	resp := ErrorResponse{
		Code:    400,
		Message: "请求参数错误",
	}

	if resp.Code != 400 {
		t.Errorf("Expected Code=400, got %d", resp.Code)
	}
}

// ========== 卷管理 API 模型测试 ==========

func TestVolumeCreateRequest_Validation(t *testing.T) {
	req := VolumeCreateRequest{
		Name:    "data-vol",
		Devices: []string{"/dev/sda", "/dev/sdb"},
		Profile: "raid1",
	}

	if req.Name != "data-vol" {
		t.Errorf("Expected Name=data-vol, got %s", req.Name)
	}
	if len(req.Devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(req.Devices))
	}
}

func TestVolume_Struct(t *testing.T) {
	vol := Volume{
		Name:       "data",
		UUID:       "uuid-123",
		Mounted:    true,
		MountPoint: "/mnt/data",
		TotalSize:  107374182400,
		UsedSize:   21474836480,
		FreeSize:   85899345920,
		Profile:    "raid1",
		Devices:    []string{"/dev/sda", "/dev/sdb"},
		SubVolumes: []string{"documents", "photos"},
	}

	if vol.Name != "data" {
		t.Errorf("Expected Name=data, got %s", vol.Name)
	}
	if !vol.Mounted {
		t.Error("Expected Mounted=true")
	}
	if vol.TotalSize != 107374182400 {
		t.Errorf("Expected TotalSize=107374182400, got %d", vol.TotalSize)
	}
}

func TestVolumeUsage_Struct(t *testing.T) {
	usage := VolumeUsage{
		Total: 107374182400,
		Used:  21474836480,
		Free:  85899345920,
	}

	if usage.Total != 107374182400 {
		t.Errorf("Expected Total=107374182400, got %d", usage.Total)
	}
	if usage.Used+usage.Free != usage.Total {
		t.Error("Used + Free should equal Total")
	}
}

func TestDeviceAddRequest_Struct(t *testing.T) {
	req := DeviceAddRequest{
		Device: "/dev/sdc",
	}

	if req.Device != "/dev/sdc" {
		t.Errorf("Expected Device=/dev/sdc, got %s", req.Device)
	}
}

// ========== 子卷管理 API 模型测试 ==========

func TestSubVolumeCreateRequest_Struct(t *testing.T) {
	req := SubVolumeCreateRequest{
		Name: "documents",
	}

	if req.Name != "documents" {
		t.Errorf("Expected Name=documents, got %s", req.Name)
	}
}

func TestSubVolumeReadOnlyRequest_Struct(t *testing.T) {
	req := SubVolumeReadOnlyRequest{
		ReadOnly: true,
	}

	if !req.ReadOnly {
		t.Error("Expected ReadOnly=true")
	}
}

// ========== 快照管理 API 模型测试 ==========

func TestSnapshotCreateRequest_Struct(t *testing.T) {
	req := SnapshotCreateRequest{
		SubVolumeName: "documents",
		Name:          "backup-2024-01-01",
		ReadOnly:      true,
	}

	if req.SubVolumeName != "documents" {
		t.Errorf("Expected SubVolumeName=documents, got %s", req.SubVolumeName)
	}
	if !req.ReadOnly {
		t.Error("Expected ReadOnly=true")
	}
}

func TestSnapshotRestoreRequest_Struct(t *testing.T) {
	req := SnapshotRestoreRequest{
		TargetName: "documents-restored",
	}

	if req.TargetName != "documents-restored" {
		t.Errorf("Expected TargetName=documents-restored, got %s", req.TargetName)
	}
}

// ========== RAID 配置 API 模型测试 ==========

func TestRAIDConvertRequest_Struct(t *testing.T) {
	req := RAIDConvertRequest{
		DataProfile: "raid1",
		MetaProfile: "raid1",
	}

	if req.DataProfile != "raid1" {
		t.Errorf("Expected DataProfile=raid1, got %s", req.DataProfile)
	}
}

// ========== 用户管理 API 模型测试 ==========

func TestUserInput_Struct(t *testing.T) {
	input := UserInput{
		Username: "john",
		Password: "secret123",
		Shell:    "/bin/bash",
		HomeDir:  "/home/john",
		Role:     "user",
	}

	if input.Username != "john" {
		t.Errorf("Expected Username=john, got %s", input.Username)
	}
	if input.Role != "user" {
		t.Errorf("Expected Role=user, got %s", input.Role)
	}
}

func TestUser_Struct(t *testing.T) {
	user := User{
		Username: "admin",
		UID:      1000,
		GID:      1000,
		HomeDir:  "/home/admin",
		Shell:    "/bin/bash",
		Disabled: false,
		Role:     "admin",
		Groups:   []string{"wheel", "docker"},
	}

	if user.Username != "admin" {
		t.Errorf("Expected Username=admin, got %s", user.Username)
	}
	if user.UID != 1000 {
		t.Errorf("Expected UID=1000, got %d", user.UID)
	}
	if len(user.Groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(user.Groups))
	}
}

func TestLoginRequest_Struct(t *testing.T) {
	req := LoginRequest{
		Username: "admin",
		Password: "password",
	}

	if req.Username != "admin" {
		t.Errorf("Expected Username=admin, got %s", req.Username)
	}
}

func TestLoginResponse_Struct(t *testing.T) {
	resp := LoginResponse{
		Token:     "eyJhbGciOiJIUzI1NiIs...",
		ExpiresAt: "2024-01-02T15:04:05Z",
		User: &User{
			Username: "admin",
			Role:     "admin",
		},
	}

	if resp.Token == "" {
		t.Error("Token should not be empty")
	}
	if resp.User == nil {
		t.Error("User should not be nil")
	}
}

func TestChangePasswordRequest_Struct(t *testing.T) {
	req := ChangePasswordRequest{
		OldPassword: "oldpass",
		NewPassword: "newpass",
	}

	if req.OldPassword != "oldpass" {
		t.Errorf("Expected OldPassword=oldpass, got %s", req.OldPassword)
	}
}

func TestResetPasswordRequest_Struct(t *testing.T) {
	req := ResetPasswordRequest{
		NewPassword: "newpass",
	}

	if req.NewPassword != "newpass" {
		t.Errorf("Expected NewPassword=newpass, got %s", req.NewPassword)
	}
}

func TestSetRoleRequest_Struct(t *testing.T) {
	req := SetRoleRequest{
		Role: "admin",
	}

	if req.Role != "admin" {
		t.Errorf("Expected Role=admin, got %s", req.Role)
	}
}

func TestGroupInput_Struct(t *testing.T) {
	input := GroupInput{
		Name: "developers",
		GID:  1001,
	}

	if input.Name != "developers" {
		t.Errorf("Expected Name=developers, got %s", input.Name)
	}
	if input.GID != 1001 {
		t.Errorf("Expected GID=1001, got %d", input.GID)
	}
}

// ========== 共享管理 API 模型测试 ==========

func TestShareOverview_Struct(t *testing.T) {
	overview := ShareOverview{
		Type: "smb",
	}

	if overview.Type != "smb" {
		t.Errorf("Expected Type=smb, got %s", overview.Type)
	}
}

// ========== 响应构建测试 ==========

func TestResponse_Success(t *testing.T) {
	resp := Response{
		Code:    0,
		Message: "success",
		Data:    nil,
	}

	if resp.Code != 0 {
		t.Errorf("Success response should have Code=0, got %d", resp.Code)
	}
}

func TestResponse_Error(t *testing.T) {
	resp := ErrorResponse{
		Code:    500,
		Message: "内部服务器错误",
	}

	if resp.Code < 400 {
		t.Errorf("Error response should have Code >= 400, got %d", resp.Code)
	}
}

// ========== 用户角色测试 ==========

func TestUser_Roles(t *testing.T) {
	roles := []string{"admin", "user", "guest"}

	for _, role := range roles {
		user := User{Role: role}
		if user.Role != role {
			t.Errorf("Expected Role=%s, got %s", role, user.Role)
		}
	}
}

// ========== 卷状态测试 ==========

func TestVolume_Mounted(t *testing.T) {
	vol := Volume{
		Name:    "data",
		Mounted: true,
	}

	if !vol.Mounted {
		t.Error("Expected Mounted=true")
	}
}

func TestVolume_Unmounted(t *testing.T) {
	vol := Volume{
		Name:    "backup",
		Mounted: false,
	}

	if vol.Mounted {
		t.Error("Expected Mounted=false")
	}
}

// ========== RAID 配置验证测试 ==========

func TestRAIDConvertRequest_Profiles(t *testing.T) {
	profiles := []string{"single", "raid0", "raid1", "raid5", "raid6", "raid10"}

	for _, profile := range profiles {
		req := RAIDConvertRequest{DataProfile: profile}
		if req.DataProfile != profile {
			t.Errorf("Expected DataProfile=%s, got %s", profile, req.DataProfile)
		}
	}
}

// ========== 快照只读测试 ==========

func TestSnapshotCreateRequest_ReadOnly(t *testing.T) {
	// 只读快照
	req := SnapshotCreateRequest{
		Name:     "readonly-snap",
		ReadOnly: true,
	}

	if !req.ReadOnly {
		t.Error("Expected ReadOnly=true")
	}

	// 可写快照
	req2 := SnapshotCreateRequest{
		Name:     "writable-snap",
		ReadOnly: false,
	}

	if req2.ReadOnly {
		t.Error("Expected ReadOnly=false")
	}
}

// ========== 用户禁用状态测试 ==========

func TestUser_Disabled(t *testing.T) {
	user := User{
		Username: "disabled-user",
		Disabled: true,
	}

	if !user.Disabled {
		t.Error("Expected Disabled=true")
	}
}

// ========== 容量计算测试 ==========

func TestVolumeUsage_Calculation(t *testing.T) {
	usage := VolumeUsage{
		Total: 100000000000, // 100 GB
		Used:  30000000000,  // 30 GB
		Free:  70000000000,  // 70 GB
	}

	if usage.Used+usage.Free != usage.Total {
		t.Errorf("Used (%d) + Free (%d) should equal Total (%d)",
			usage.Used, usage.Free, usage.Total)
	}

	usedPercent := float64(usage.Used) / float64(usage.Total) * 100
	if usedPercent != 30.0 {
		t.Errorf("Expected usedPercent=30.0, got %.1f", usedPercent)
	}
}
