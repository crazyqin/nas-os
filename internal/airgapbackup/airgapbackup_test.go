package airgapbackup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVaultStates(t *testing.T) {
	assert.Equal(t, "online", string(VaultStateOnline))
	assert.Equal(t, "airgapped", string(VaultStateAirGapped))
	assert.Equal(t, "disconnecting", string(VaultStateDisconnecting))
}

func TestWORMPolicies(t *testing.T) {
	assert.Equal(t, "disabled", string(WORMDisabled))
	assert.Equal(t, "compliance", string(WORMCompliance))
	assert.Equal(t, "governance", string(WORMGovernance))
}

func TestCreateVault(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	vault, err := mgr.CreateVault("primary", "/dev/sdb1", WORMCompliance, true)
	require.NoError(t, err)
	assert.Equal(t, "primary", vault.Name)
	assert.Equal(t, VaultStateOnline, vault.State)
	assert.Equal(t, WORMCompliance, vault.WORMPolicy)
}

func TestDisconnectConnect(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	vault, _ := mgr.CreateVault("primary", "/dev/sdb1", WORMDisabled, false)

	err := mgr.Disconnect(vault.ID)
	require.NoError(t, err)

	v, _ := mgr.GetVault(vault.ID)
	assert.Equal(t, VaultStateAirGapped, v.State)

	err = mgr.Connect(vault.ID)
	require.NoError(t, err)

	v, _ = mgr.GetVault(vault.ID)
	assert.Equal(t, VaultStateOnline, v.State)
}

func TestCreateBackup(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	vault, _ := mgr.CreateVault("primary", "/dev/sdb1", WORMDisabled, false)

	backup, err := mgr.CreateBackup(vault.ID, "daily-001", 1024*1024, 100, []byte("test data"))
	require.NoError(t, err)
	assert.Equal(t, BackupStatusCompleted, backup.Status)
	assert.NotEmpty(t, backup.Checksum)
	assert.NotEmpty(t, backup.ChainHash)
}

func TestWORMComplianceDelete(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	vault, _ := mgr.CreateVault("secure", "/dev/sdc1", WORMCompliance, true)
	backup, _ := mgr.CreateBackup(vault.ID, "immutable-001", 512, 10, []byte("immutable"))

	err := mgr.DeleteBackup(backup.ID)
	assert.ErrorIs(t, err, ErrWORMViolation)
}

func TestWORMGovernanceDelete(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	vault, _ := mgr.CreateVault("managed", "/dev/sdd1", WORMGovernance, false)
	backup, _ := mgr.CreateBackup(vault.ID, "managed-001", 512, 10, []byte("managed"))

	err := mgr.DeleteBackup(backup.ID)
	assert.NoError(t, err) // Governance mode allows admin delete
}

func TestVerifyBackup(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	vault, _ := mgr.CreateVault("primary", "/dev/sdb1", WORMDisabled, false)
	backup, _ := mgr.CreateBackup(vault.ID, "verify-001", 1024, 50, []byte("verify me"))

	err := mgr.VerifyBackup(backup.ID)
	require.NoError(t, err)

	b, _ := mgr.GetBackupByID(backup.ID)
	assert.Equal(t, BackupStatusVerified, b.Status)
}

func TestChainHashIntegrity(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	vault, _ := mgr.CreateVault("primary", "/dev/sdb1", WORMDisabled, false)

	b1, _ := mgr.CreateBackup(vault.ID, "chain-001", 100, 10, []byte("first"))
	b2, _ := mgr.CreateBackup(vault.ID, "chain-002", 200, 20, []byte("second"))

	assert.NotEmpty(t, b1.ChainHash)
	assert.NotEmpty(t, b2.ChainHash)
	assert.NotEqual(t, b1.ChainHash, b2.ChainHash)
}

func TestBackupOnAirGappedVault(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	vault, _ := mgr.CreateVault("offline", "/dev/sde1", WORMDisabled, false)
	_ = mgr.Disconnect(vault.ID)

	_, err := mgr.CreateBackup(vault.ID, "fail-001", 100, 10, []byte("data"))
	assert.ErrorIs(t, err, ErrConnectionNotActive)
}

func TestListBackups(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	vault, _ := mgr.CreateVault("primary", "/dev/sdb1", WORMDisabled, false)
	_, _ = mgr.CreateBackup(vault.ID, "list-001", 100, 10, []byte("a"))
	_, _ = mgr.CreateBackup(vault.ID, "list-002", 200, 20, []byte("b"))

	backups := mgr.ListBackups(vault.ID)
	assert.Len(t, backups, 2)
}

func TestManagerClosed(t *testing.T) {
	mgr := NewManager()
	mgr.Close()

	_, err := mgr.CreateVault("test", "/dev/sdX", WORMDisabled, false)
	assert.ErrorIs(t, err, ErrManagerClosed)
}
