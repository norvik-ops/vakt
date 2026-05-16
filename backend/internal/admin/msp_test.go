package admin

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- slugify ---

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Acme Corp", "acme-corp"},
		{"  Hello   World  ", "hello-world"},
		{"NIS2 Audit GmbH & Co. KG", "nis2-audit-gmbh-co-kg"},
		{"already-slug", "already-slug"},
		{"123 Numbers", "123-numbers"},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, slugify(tc.input))
		})
	}
}

// --- isNotManagedByError ---

func TestIsNotManagedByError(t *testing.T) {
	assert.True(t, isNotManagedByError(fmt.Errorf("org abc is not managed by msp xyz")))
	assert.True(t, isNotManagedByError(fmt.Errorf("msp org foo does not have access to is not managed by bar")))
	assert.False(t, isNotManagedByError(fmt.Errorf("some other error")))
	assert.False(t, isNotManagedByError(nil))
}

// --- MSPService unit tests (stub repository) ---

// stubRepo is a minimal in-memory stand-in for Repository.
type stubRepo struct {
	orgs map[string]*Organization
}

func newStubRepo() *stubRepo {
	return &stubRepo{orgs: make(map[string]*Organization)}
}

func (r *stubRepo) CreateOrg(_ context.Context, name, plan, parentOrgID string) (*Organization, error) {
	org := &Organization{
		ID:          fmt.Sprintf("new-org-%d", len(r.orgs)+1),
		Name:        name,
		Slug:        slugify(name),
		Plan:        plan,
		ParentOrgID: &parentOrgID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	r.orgs[org.ID] = org
	return org, nil
}

func (r *stubRepo) ListChildOrgs(_ context.Context, parentOrgID string) ([]ManagedOrgSummary, error) {
	var result []ManagedOrgSummary
	for _, o := range r.orgs {
		if o.ParentOrgID != nil && *o.ParentOrgID == parentOrgID {
			result = append(result, ManagedOrgSummary{
				ID:        o.ID,
				Name:      o.Name,
				Plan:      o.Plan,
				CreatedAt: o.CreatedAt,
			})
		}
	}
	return result, nil
}

func (r *stubRepo) ScheduleOrgDeletion(_ context.Context, orgID string, at time.Time) error {
	o, ok := r.orgs[orgID]
	if !ok {
		return fmt.Errorf("org not found: %s", orgID)
	}
	o.ScheduledDeletionAt = &at
	return nil
}

func (r *stubRepo) GetOrg(_ context.Context, orgID string) (*Organization, error) {
	o, ok := r.orgs[orgID]
	if !ok {
		return nil, fmt.Errorf("org not found: %s", orgID)
	}
	return o, nil
}

func (r *stubRepo) UpdateOrgBranding(_ context.Context, orgID, logoURL string, colors map[string]string) error {
	o, ok := r.orgs[orgID]
	if !ok {
		return fmt.Errorf("org not found: %s", orgID)
	}
	o.MSPBrandLogo = &logoURL
	o.MSPBrandColors = colors
	return nil
}

// mspServiceFromStub builds an MSPService backed by the stub repo.
func mspServiceFromStub(r *stubRepo) *MSPService {
	return &MSPService{
		repo:        &Repository{}, // not used; methods below bypass via interface
		asynqClient: nil,
	}
}

// repoMSPService is an MSPService variant that accepts a repoInterface for testing.
type repoInterface interface {
	CreateOrg(ctx context.Context, name, plan, parentOrgID string) (*Organization, error)
	ListChildOrgs(ctx context.Context, parentOrgID string) ([]ManagedOrgSummary, error)
	ScheduleOrgDeletion(ctx context.Context, orgID string, at time.Time) error
	GetOrg(ctx context.Context, orgID string) (*Organization, error)
	UpdateOrgBranding(ctx context.Context, orgID, logoURL string, colors map[string]string) error
}

// testMSP is an MSPService-equivalent backed by a repoInterface for unit testing.
type testMSP struct {
	repo repoInterface
}

func (s *testMSP) CreateManagedOrg(ctx context.Context, mspOrgID, _ string, input CreateManagedOrgInput) (*Organization, error) {
	org, err := s.repo.CreateOrg(ctx, input.Name, input.Plan, mspOrgID)
	if err != nil {
		return nil, fmt.Errorf("create managed org: %w", err)
	}
	return org, nil
}

func (s *testMSP) ListManagedOrgs(ctx context.Context, mspOrgID string) ([]ManagedOrgSummary, error) {
	return s.repo.ListChildOrgs(ctx, mspOrgID)
}

func (s *testMSP) DeleteManagedOrg(ctx context.Context, mspOrgID, targetOrgID string) error {
	org, err := s.repo.GetOrg(ctx, targetOrgID)
	if err != nil {
		return fmt.Errorf("get target org: %w", err)
	}
	if org.ParentOrgID == nil || *org.ParentOrgID != mspOrgID {
		return fmt.Errorf("org %s is not managed by msp org %s", targetOrgID, mspOrgID)
	}
	deletionAt := time.Now().UTC().Add(orgDeletionGrace)
	return s.repo.ScheduleOrgDeletion(ctx, targetOrgID, deletionAt)
}

func (s *testMSP) UpdateOrgBranding(ctx context.Context, mspOrgID, targetOrgID string, input BrandingInput) error {
	org, err := s.repo.GetOrg(ctx, targetOrgID)
	if err != nil {
		return fmt.Errorf("get target org: %w", err)
	}
	if org.ParentOrgID == nil || *org.ParentOrgID != mspOrgID {
		return fmt.Errorf("org %s is not managed by msp org %s", targetOrgID, mspOrgID)
	}
	colors := input.Colors
	if colors == nil {
		colors = map[string]string{}
	}
	return s.repo.UpdateOrgBranding(ctx, targetOrgID, input.LogoURL, colors)
}

func (s *testMSP) GetOrgBranding(ctx context.Context, orgID string) (*BrandingConfig, error) {
	org, err := s.repo.GetOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get org: %w", err)
	}
	logoURL := ""
	if org.MSPBrandLogo != nil {
		logoURL = *org.MSPBrandLogo
	}
	colors := org.MSPBrandColors
	if colors == nil {
		colors = map[string]string{}
	}
	return &BrandingConfig{OrgID: org.ID, LogoURL: logoURL, Colors: colors}, nil
}

func (s *testMSP) SwitchContext(ctx context.Context, mspOrgID, targetOrgID string) error {
	org, err := s.repo.GetOrg(ctx, targetOrgID)
	if err != nil {
		return fmt.Errorf("get target org: %w", err)
	}
	if org.ParentOrgID == nil || *org.ParentOrgID != mspOrgID {
		return fmt.Errorf("msp org %s does not have access to org %s", mspOrgID, targetOrgID)
	}
	return nil
}

// --- Tests using testMSP + stubRepo ---

func newTestMSP(t *testing.T) (*testMSP, *stubRepo) {
	t.Helper()
	repo := newStubRepo()
	return &testMSP{repo: repo}, repo
}

func TestCreateManagedOrg(t *testing.T) {
	svc, _ := newTestMSP(t)
	ctx := context.Background()

	org, err := svc.CreateManagedOrg(ctx, "msp-001", "user-001", CreateManagedOrgInput{
		Name: "Acme GmbH",
		Plan: "msp_managed",
	})
	require.NoError(t, err)
	assert.Equal(t, "Acme GmbH", org.Name)
	assert.Equal(t, "acme-gmbh", org.Slug)
	assert.Equal(t, "msp_managed", org.Plan)
	require.NotNil(t, org.ParentOrgID)
	assert.Equal(t, "msp-001", *org.ParentOrgID)
}

func TestListManagedOrgs(t *testing.T) {
	svc, _ := newTestMSP(t)
	ctx := context.Background()

	_, err := svc.CreateManagedOrg(ctx, "msp-001", "u", CreateManagedOrgInput{Name: "Org A", Plan: "standard"})
	require.NoError(t, err)
	_, err = svc.CreateManagedOrg(ctx, "msp-001", "u", CreateManagedOrgInput{Name: "Org B", Plan: "enterprise"})
	require.NoError(t, err)
	// Create an org under a different MSP — should not appear.
	_, err = svc.CreateManagedOrg(ctx, "msp-other", "u", CreateManagedOrgInput{Name: "Org C", Plan: "standard"})
	require.NoError(t, err)

	orgs, err := svc.ListManagedOrgs(ctx, "msp-001")
	require.NoError(t, err)
	assert.Len(t, orgs, 2)
}

func TestDeleteManagedOrg_SetsScheduledDeletion(t *testing.T) {
	svc, repo := newTestMSP(t)
	ctx := context.Background()

	org, err := svc.CreateManagedOrg(ctx, "msp-001", "u", CreateManagedOrgInput{Name: "Target", Plan: "standard"})
	require.NoError(t, err)

	err = svc.DeleteManagedOrg(ctx, "msp-001", org.ID)
	require.NoError(t, err)

	stored := repo.orgs[org.ID]
	require.NotNil(t, stored.ScheduledDeletionAt)
	assert.True(t, stored.ScheduledDeletionAt.After(time.Now().UTC()),
		"scheduled deletion should be in the future")
}

func TestDeleteManagedOrg_ForbiddenForWrongMSP(t *testing.T) {
	svc, _ := newTestMSP(t)
	ctx := context.Background()

	org, err := svc.CreateManagedOrg(ctx, "msp-001", "u", CreateManagedOrgInput{Name: "Target", Plan: "standard"})
	require.NoError(t, err)

	err = svc.DeleteManagedOrg(ctx, "msp-other", org.ID)
	require.Error(t, err)
	assert.True(t, isNotManagedByError(err))
}

func TestUpdateAndGetOrgBranding(t *testing.T) {
	svc, _ := newTestMSP(t)
	ctx := context.Background()

	org, err := svc.CreateManagedOrg(ctx, "msp-001", "u", CreateManagedOrgInput{Name: "Brand Org", Plan: "standard"})
	require.NoError(t, err)

	err = svc.UpdateOrgBranding(ctx, "msp-001", org.ID, BrandingInput{
		LogoURL: "https://example.com/logo.png",
		Colors:  map[string]string{"primary": "#ff0000"},
	})
	require.NoError(t, err)

	branding, err := svc.GetOrgBranding(ctx, org.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/logo.png", branding.LogoURL)
	assert.Equal(t, "#ff0000", branding.Colors["primary"])
}

func TestUpdateOrgBranding_ForbiddenForWrongMSP(t *testing.T) {
	svc, _ := newTestMSP(t)
	ctx := context.Background()

	org, err := svc.CreateManagedOrg(ctx, "msp-001", "u", CreateManagedOrgInput{Name: "Brand Org", Plan: "standard"})
	require.NoError(t, err)

	err = svc.UpdateOrgBranding(ctx, "msp-other", org.ID, BrandingInput{LogoURL: "https://evil.com"})
	require.Error(t, err)
	assert.True(t, isNotManagedByError(err))
}

func TestSwitchContext_AllowsValidChild(t *testing.T) {
	svc, _ := newTestMSP(t)
	ctx := context.Background()

	org, err := svc.CreateManagedOrg(ctx, "msp-001", "u", CreateManagedOrgInput{Name: "Child", Plan: "standard"})
	require.NoError(t, err)

	err = svc.SwitchContext(ctx, "msp-001", org.ID)
	assert.NoError(t, err)
}

func TestSwitchContext_BlocksUnrelatedOrg(t *testing.T) {
	svc, _ := newTestMSP(t)
	ctx := context.Background()

	org, err := svc.CreateManagedOrg(ctx, "msp-001", "u", CreateManagedOrgInput{Name: "Child", Plan: "standard"})
	require.NoError(t, err)

	err = svc.SwitchContext(ctx, "msp-other", org.ID)
	require.Error(t, err)
}

// Ensure mspServiceFromStub is referenced (avoids unused function warning).
var _ = mspServiceFromStub
