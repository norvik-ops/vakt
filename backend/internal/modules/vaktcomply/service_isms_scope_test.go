package vaktcomply

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApproveISMSScopeRoleCheck verifies that only admin users may approve.
// The repo call is skipped because the role guard fires before it.
func TestApproveISMSScopeRoleCheck(t *testing.T) {
	svc := &Service{repo: nil} // repo is never reached for non-admin
	ctx := context.Background()

	_, err := svc.ApproveISMSScope(ctx, "org-1", "scope-1", "user-1", "analyst")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only admins")

	_, err = svc.ApproveISMSScope(ctx, "org-1", "scope-1", "user-1", "viewer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only admins")

	_, err = svc.ApproveISMSScope(ctx, "org-1", "scope-1", "user-1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only admins")
}
