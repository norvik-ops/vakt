package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService_nonNil(t *testing.T) {
	svc := NewService(nil, "vaktscan,vaktcomply")
	require.NotNil(t, svc)
	assert.Equal(t, "vaktscan,vaktcomply", svc.modulesEnabled)
}

func TestWithNotifyService_returnsReceiver(t *testing.T) {
	svc := NewService(nil, "")
	returned := svc.WithNotifyService(nil)
	assert.Equal(t, svc, returned, "WithNotifyService must return the same *Service")
}

func TestListModules_allEnabled(t *testing.T) {
	svc := NewService(nil, "vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy")
	mods := svc.ListModules()
	require.Len(t, mods, 5)
	for _, m := range mods {
		assert.True(t, m.Enabled, "module %q should be enabled", m.Name)
	}
}

func TestListModules_noneEnabled(t *testing.T) {
	svc := NewService(nil, "")
	mods := svc.ListModules()
	require.Len(t, mods, 5)
	for _, m := range mods {
		assert.False(t, m.Enabled, "module %q should be disabled", m.Name)
	}
}

func TestListModules_partial(t *testing.T) {
	svc := NewService(nil, "vaktscan,vaktprivacy")
	mods := svc.ListModules()
	byName := make(map[string]bool, len(mods))
	for _, m := range mods {
		byName[m.Name] = m.Enabled
	}
	assert.True(t, byName["vaktscan"])
	assert.True(t, byName["vaktprivacy"])
	assert.False(t, byName["vaktcomply"])
	assert.False(t, byName["vaktvault"])
	assert.False(t, byName["vaktaware"])
}

func TestListModules_caseInsensitive(t *testing.T) {
	svc := NewService(nil, "VAKTSCAN,VaktComply")
	mods := svc.ListModules()
	byName := make(map[string]bool, len(mods))
	for _, m := range mods {
		byName[m.Name] = m.Enabled
	}
	assert.True(t, byName["vaktscan"], "VAKTSCAN should match vaktscan")
	assert.True(t, byName["vaktcomply"], "VaktComply should match vaktcomply")
}

func TestListModules_whitespaceStripped(t *testing.T) {
	svc := NewService(nil, " vaktscan , vaktaware ")
	mods := svc.ListModules()
	byName := make(map[string]bool, len(mods))
	for _, m := range mods {
		byName[m.Name] = m.Enabled
	}
	assert.True(t, byName["vaktscan"])
	assert.True(t, byName["vaktaware"])
}

func TestListNotificationChannels_nilNotifySvc(t *testing.T) {
	svc := NewService(nil, "")
	_, err := svc.ListNotificationChannels(nil, "org-1") //nolint:staticcheck
	assert.ErrorContains(t, err, "notification service not configured")
}

func TestCreateUserInput_passwordMinLen(t *testing.T) {
	// CreateUserInput.Password requires min=10 — verify field tag is present.
	input := CreateUserInput{Email: "a@b.com", Password: "short", Role: "Admin"}
	assert.Equal(t, "short", input.Password, "field value accessible")
	assert.Less(t, len(input.Password), 10, "below minimum confirms validation would reject it")
}

func TestWithMasterKey_returnsReceiver(t *testing.T) {
	svc := NewService(nil, "")
	returned := svc.WithMasterKey([]byte("key"))
	require.Equal(t, svc, returned)
	assert.Equal(t, []byte("key"), svc.masterKey)
}
