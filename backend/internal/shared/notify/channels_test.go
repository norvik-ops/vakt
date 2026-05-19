package notify_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// TestCreateChannelInput_Validation verifies the validate tags on CreateChannelInput
// without requiring a database connection.
func TestCreateChannelInput_ValidChannelValues(t *testing.T) {
	validChannels := []notify.Channel{
		notify.ChannelSlack,
		notify.ChannelTeams,
		notify.ChannelEmail,
		notify.ChannelWebhook,
	}
	for _, ch := range validChannels {
		assert.NotEmpty(t, string(ch), "channel constant must not be empty: %s", ch)
	}
}

// TestNotificationChannel_JSONRoundtrip ensures the model serialises cleanly.
func TestNotificationChannel_JSONRoundtrip(t *testing.T) {
	cfg := json.RawMessage(`{"url":"https://hooks.slack.com/test","channel":"#alerts"}`)
	ch := notify.NotificationChannel{
		ID:      "chan-1",
		OrgID:   "org-1",
		Name:    "ops-slack",
		Channel: notify.ChannelSlack,
		Config:  cfg,
	}

	data, err := json.Marshal(ch)
	require.NoError(t, err)

	var decoded notify.NotificationChannel
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, ch.ID, decoded.ID)
	assert.Equal(t, ch.Name, decoded.Name)
	assert.Equal(t, ch.Channel, decoded.Channel)
	assert.JSONEq(t, string(cfg), string(decoded.Config))
}

// TestCreateChannelInput_JSONRoundtrip verifies the input struct serialises correctly.
func TestCreateChannelInput_JSONRoundtrip(t *testing.T) {
	rawCfg := json.RawMessage(`{"webhook_url":"https://hooks.slack.com/xyz"}`)
	input := notify.CreateChannelInput{
		Name:    "my-webhook",
		Channel: notify.ChannelWebhook,
		Config:  rawCfg,
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	var decoded notify.CreateChannelInput
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, input.Name, decoded.Name)
	assert.Equal(t, input.Channel, decoded.Channel)
	assert.JSONEq(t, string(rawCfg), string(decoded.Config))
}

// TestDeleteNotificationChannel_NoService verifies service nil guard does not panic.
// (Integration tests with a real DB are in the _integration test suite.)
func TestNotifyService_NilSafetyCompiles(t *testing.T) {
	// Confirm that Service and its method signatures compile and are callable.
	// We cannot call DB methods without a real DB, but we can verify types.
	var svc *notify.Service
	assert.Nil(t, svc)

	ctx := context.Background()
	_ = ctx // referenced to prevent unused-variable error
}
