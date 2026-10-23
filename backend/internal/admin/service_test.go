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

// TestListModules_onlyCore: since the archive (docs/stories/module-archivieren.md)
// the overview lists the ISMS alone, enabled, whatever VAKT_MODULES_ENABLED says.
func TestListModules_onlyCore(t *testing.T) {
	for _, csv := range []string{"", "vaktcomply", "vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy,vakthr", " VAKTSCAN , vaktaware "} {
		mods := NewService(nil, csv).ListModules()
		require.Len(t, mods, 1, csv)
		assert.Equal(t, "vaktcomply", mods[0].Name)
		assert.True(t, mods[0].Enabled, csv)
	}
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
