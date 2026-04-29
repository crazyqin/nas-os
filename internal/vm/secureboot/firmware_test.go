package secureboot

import (
	"crypto/sha256"
	"testing"
)

func TestNewFirmwareVerifier(t *testing.T) {
	km := NewKeyManager(nil)
	fv := NewFirmwareVerifier(km, nil)

	if fv == nil {
		t.Fatal("NewFirmwareVerifier 返回 nil")
	}
	if fv.knownHashes == nil {
		t.Error("knownHashes 应已初始化")
	}
}

func TestRegisterKnownFirmware(t *testing.T) {
	km := NewKeyManager(nil)
	fv := NewFirmwareVerifier(km, nil)

	data := []byte("firmware content")
	hash := sha256.Sum256(data)

	fv.RegisterKnownFirmware("test-uefi", hash)

	fv.mu.RLock()
	stored, ok := fv.knownHashes["test-uefi"]
	fv.mu.RUnlock()

	if !ok {
		t.Fatal("固件应已注册")
	}
	if stored != hash {
		t.Error("哈希不匹配")
	}
}

func TestVerifyFirmwareEmptyData(t *testing.T) {
	km := NewKeyManager(nil)
	fv := NewFirmwareVerifier(km, nil)

	fw := &FirmwareImage{
		Name: "test",
		Data: nil,
	}

	result := fv.VerifyFirmware(fw)
	if result.Valid {
		t.Error("空数据固件不应验证通过")
	}
	if result.Reason != "固件数据为空" {
		t.Errorf("原因不匹配：%s", result.Reason)
	}
}

func TestVerifyFirmwareHashMismatch(t *testing.T) {
	km := NewKeyManager(nil)
	fv := NewFirmwareVerifier(km, nil)

	// 注册一个已知哈希
	correctData := []byte("correct firmware")
	correctHash := sha256.Sum256(correctData)
	fv.RegisterKnownFirmware("uefi-1.0", correctHash)

	// 使用错误数据验证
	wrongData := []byte("tampered firmware")
	fw := &FirmwareImage{
		Name: "uefi-1.0",
		Data: wrongData,
	}

	result := fv.VerifyFirmware(fw)
	if result.Valid {
		t.Error("篡改固件不应验证通过")
	}
	if result.HashValid {
		t.Error("HashValid 应为 false")
	}
}

func TestVerifyFirmwareHashMatch(t *testing.T) {
	km := NewKeyManager(nil)
	fv := NewFirmwareVerifier(km, nil)

	data := []byte("firmware v1.0")
	hash := sha256.Sum256(data)
	fv.RegisterKnownFirmware("uefi-1.0", hash)

	fw := &FirmwareImage{
		Name:    "uefi-1.0",
		Version: "1.0",
		Data:    data,
	}

	result := fv.VerifyFirmware(fw)
	if !result.Valid {
		t.Errorf("正确固件应验证通过，原因：%s", result.Reason)
	}
	if !result.HashValid {
		t.Error("HashValid 应为 true")
	}
	if result.FirmwareName != "uefi-1.0" {
		t.Errorf("固件名不匹配：%s", result.FirmwareName)
	}
}

func TestVerifyFirmwareNotInWhitelist(t *testing.T) {
	km := NewKeyManager(nil)
	fv := NewFirmwareVerifier(km, nil)

	fw := &FirmwareImage{
		Name: "unknown-uefi",
		Data: []byte("some firmware"),
	}

	result := fv.VerifyFirmware(fw)
	// 不在白名单但数据非空 — 审计模式下应允许
	if !result.Valid {
		t.Errorf("不在白名单的固件在审计模式下应允许，原因：%s", result.Reason)
	}
}

func TestVerifyFirmwareWithSignature(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()
	_ = km.InitDefaultKeys()

	fv := NewFirmwareVerifier(km, nil)

	// 注册 db 中的签名者
	cert := generateTestCert(t, "Firmware Signer")
	_ = km.AddDBEntry(cert)

	fw := &FirmwareImage{
		Name:     "signed-uefi",
		Data:     []byte("signed firmware"),
		SignedBy: "Firmware Signer",
	}

	result := fv.VerifyFirmware(fw)
	if !result.Valid {
		t.Errorf("签名固件应验证通过，原因：%s", result.Reason)
	}
	if !result.SignatureValid {
		t.Error("SignatureValid 应为 true")
	}
}

func TestVerifyFirmwareUnknownSigner(t *testing.T) {
	km := NewKeyManager(nil)
	_ = km.GeneratePlatformCA()

	fv := NewFirmwareVerifier(km, nil)

	fw := &FirmwareImage{
		Name:     "bad-signed",
		Data:     []byte("firmware"),
		SignedBy: "Unknown Signer",
	}

	result := fv.VerifyFirmware(fw)
	if result.Valid {
		t.Error("未知签名者不应验证通过")
	}
	if result.Reason == "" {
		t.Error("应有失败原因")
	}
}

func TestVerifyBootChainEmpty(t *testing.T) {
	km := NewKeyManager(nil)
	fv := NewFirmwareVerifier(km, nil)

	result := fv.VerifyBootChain(nil)
	if result.Valid {
		t.Error("空启动链不应验证通过")
	}
	if result.OverallOK {
		t.Error("OverallOK 应为 false")
	}
}

func TestVerifyBootChainValid(t *testing.T) {
	km := NewKeyManager(nil)
	fv := NewFirmwareVerifier(km, nil)

	data1 := []byte("uefi firmware")
	data2 := []byte("grub bootloader")
	data3 := []byte("linux kernel")

	hash1 := sha256.Sum256(data1)
	hash2 := sha256.Sum256(data2)
	hash3 := sha256.Sum256(data3)

	components := []*BootComponent{
		{Name: "UEFI", Data: data1, Hash: hash1},
		{Name: "GRUB", Data: data2, Hash: hash2},
		{Name: "Kernel", Data: data3, Hash: hash3},
	}

	result := fv.VerifyBootChain(components)
	if !result.Valid {
		t.Error("有效启动链应验证通过")
	}
	if !result.OverallOK {
		t.Error("OverallOK 应为 true")
	}
	if len(result.Components) != 3 {
		t.Errorf("应有 3 个组件结果，实际 %d", len(result.Components))
	}
}

func TestVerifyBootChainOneFailed(t *testing.T) {
	km := NewKeyManager(nil)
	fv := NewFirmwareVerifier(km, nil)

	components := []*BootComponent{
		{Name: "UEFI", Data: []byte("uefi"), Hash: sha256.Sum256([]byte("uefi"))},
		{Name: "Bad", Data: []byte("tampered"), Hash: [32]byte{0xff}},
	}

	result := fv.VerifyBootChain(components)
	if result.Valid {
		t.Error("含失败组件的启动链不应验证通过")
	}
	if result.OverallOK {
		t.Error("OverallOK 应为 false")
	}
}

// === MemoryVariableStore 测试 ===

func TestMemoryVariableStoreSetGet(t *testing.T) {
	store := NewMemoryVariableStore()

	v := &UEFIVariable{
		Name:       "TestVar",
		GUID:       UEFIGlobalGUID,
		Attributes: 0x07,
		Data:       []byte{1, 2, 3},
	}

	err := store.SetVariable(v)
	if err != nil {
		t.Fatalf("SetVariable 失败：%v", err)
	}

	got, err := store.GetVariable("TestVar", UEFIGlobalGUID)
	if err != nil {
		t.Fatalf("GetVariable 失败：%v", err)
	}
	if got.Name != "TestVar" {
		t.Errorf("名称不匹配：%s", got.Name)
	}
	if len(got.Data) != 3 {
		t.Errorf("数据长度应为 3，实际 %d", len(got.Data))
	}
}

func TestMemoryVariableStoreGetNotFound(t *testing.T) {
	store := NewMemoryVariableStore()

	_, err := store.GetVariable("NonExistent", UEFIGlobalGUID)
	if err == nil {
		t.Error("获取不存在的变量应返回错误")
	}
}

func TestMemoryVariableStoreSetNil(t *testing.T) {
	store := NewMemoryVariableStore()

	err := store.SetVariable(nil)
	if err == nil {
		t.Error("设置 nil 变量应返回错误")
	}
}

func TestMemoryVariableStoreDelete(t *testing.T) {
	store := NewMemoryVariableStore()

	_ = store.SetVariable(&UEFIVariable{
		Name: "ToDelete",
		GUID: UEFIGlobalGUID,
		Data: []byte{1},
	})

	err := store.DeleteVariable("ToDelete", UEFIGlobalGUID)
	if err != nil {
		t.Fatalf("DeleteVariable 失败：%v", err)
	}

	_, err = store.GetVariable("ToDelete", UEFIGlobalGUID)
	if err == nil {
		t.Error("删除后获取应返回错误")
	}
}

func TestMemoryVariableStoreDeleteNotFound(t *testing.T) {
	store := NewMemoryVariableStore()

	err := store.DeleteVariable("NonExistent", UEFIGlobalGUID)
	if err == nil {
		t.Error("删除不存在的变量应返回错误")
	}
}

func TestMemoryVariableStoreList(t *testing.T) {
	store := NewMemoryVariableStore()

	_ = store.SetVariable(&UEFIVariable{Name: "A", GUID: "g1", Data: []byte{1}})
	_ = store.SetVariable(&UEFIVariable{Name: "B", GUID: "g2", Data: []byte{2}})
	_ = store.SetVariable(&UEFIVariable{Name: "C", GUID: "g1", Data: []byte{3}})

	vars, err := store.ListVariables()
	if err != nil {
		t.Fatalf("ListVariables 失败：%v", err)
	}
	if len(vars) != 3 {
		t.Errorf("应有 3 个变量，实际 %d", len(vars))
	}
}

func TestMemoryVariableStoreCount(t *testing.T) {
	store := NewMemoryVariableStore()
	if store.Count() != 0 {
		t.Error("初始应为 0")
	}

	_ = store.SetVariable(&UEFIVariable{Name: "X", GUID: "g", Data: []byte{1}})
	_ = store.SetVariable(&UEFIVariable{Name: "Y", GUID: "g", Data: []byte{2}})

	if store.Count() != 2 {
		t.Errorf("应为 2，实际 %d", store.Count())
	}
}

func TestMemoryVariableStoreOverwrite(t *testing.T) {
	store := NewMemoryVariableStore()

	_ = store.SetVariable(&UEFIVariable{Name: "V", GUID: "g", Data: []byte{1}})
	_ = store.SetVariable(&UEFIVariable{Name: "V", GUID: "g", Data: []byte{2, 3}})

	got, _ := store.GetVariable("V", "g")
	if len(got.Data) != 2 {
		t.Errorf("覆盖后数据长度应为 2，实际 %d", len(got.Data))
	}
}

func TestUEFIVariableNames(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{UEFIVarPK, "PK"},
		{UEFIVarKEK, "KEK"},
		{UEFIVarDB, "db"},
		{UEFIVarDBX, "dbx"},
		{UEFIVarSecureBoot, "SecureBoot"},
		{UEFIVarSetupMode, "SetupMode"},
		{UEFIVarDeployedMode, "DeployedMode"},
	}
	for _, tt := range tests {
		if tt.name != tt.want {
			t.Errorf("变量名 %q 应为 %q", tt.name, tt.want)
		}
	}
}
