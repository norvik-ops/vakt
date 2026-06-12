package scim

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── mockRevoker ─────────────────────────────────────────────────────────────

// mockRevoker tracks calls to RevokeAllSessions.
type mockRevoker struct {
	called []string
	err    error
}

func (m *mockRevoker) RevokeAllSessions(_ context.Context, userID string) error {
	m.called = append(m.called, userID)
	return m.err
}

// compile-time check: mockRevoker satisfies SessionRevoker.
var _ SessionRevoker = (*mockRevoker)(nil)

// ─── mockTx ──────────────────────────────────────────────────────────────────

// mockTx implements pgx.Tx for DeactivateUser tests.
// Only Begin/Commit/Rollback/Exec are exercised; all others panic if called.
type mockTx struct {
	execErr    error
	commitErr  error
	execCalled int
}

func (m *mockTx) Begin(ctx context.Context) (pgx.Tx, error) { return m, nil }
func (m *mockTx) Commit(ctx context.Context) error          { return m.commitErr }
func (m *mockTx) Rollback(ctx context.Context) error        { return nil }
func (m *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalled++
	return pgconn.CommandTag{}, m.execErr
}
func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	panic("mockTx.Query not expected in this test")
}
func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	panic("mockTx.QueryRow not expected in this test")
}
func (m *mockTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	panic("mockTx.CopyFrom not expected")
}
func (m *mockTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults {
	panic("mockTx.SendBatch not expected")
}
func (m *mockTx) LargeObjects() pgx.LargeObjects { panic("mockTx.LargeObjects not expected") }
func (m *mockTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	panic("mockTx.Prepare not expected")
}
func (m *mockTx) Conn() *pgx.Conn { panic("mockTx.Conn not expected") }

var _ pgx.Tx = (*mockTx)(nil)

// ─── mockDB ──────────────────────────────────────────────────────────────────

// mockDB implements dbPool; only Begin is exercised by DeactivateUser.
type mockDB struct {
	tx  *mockTx
	err error
}

func (m *mockDB) Begin(_ context.Context) (pgx.Tx, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tx, nil
}
func (m *mockDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	panic("mockDB.Exec not expected")
}
func (m *mockDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	panic("mockDB.Query not expected")
}
func (m *mockDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("mockDB.QueryRow not expected")
}

var _ dbPool = (*mockDB)(nil)

// ─── Builder tests ────────────────────────────────────────────────────────────

func TestNewService_nonNil(t *testing.T) {
	svc := NewService(nil)
	require.NotNil(t, svc)
	assert.Nil(t, svc.sessionRevoker, "sessionRevoker must start nil")
}

func TestWithSessionRevoker_setsField(t *testing.T) {
	r := &mockRevoker{}
	svc := NewService(nil).WithSessionRevoker(r)
	require.NotNil(t, svc)
	assert.Equal(t, r, svc.sessionRevoker, "WithSessionRevoker must store the revoker")
}

func TestWithSessionRevoker_nilSafe(t *testing.T) {
	svc := NewService(nil).WithSessionRevoker(nil)
	require.NotNil(t, svc)
	assert.Nil(t, svc.sessionRevoker)
}

func TestWithSessionRevoker_returnsReceiver(t *testing.T) {
	r := &mockRevoker{}
	svc := NewService(nil)
	returned := svc.WithSessionRevoker(r)
	assert.Equal(t, svc, returned, "WithSessionRevoker must return the same *Service")
}

// ─── DeactivateUser + session revocation ──────────────────────────────────────

func TestDeactivateUser_callsRevoker(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}
	r := &mockRevoker{}
	svc := &Service{db: db, sessionRevoker: r}

	err := svc.DeactivateUser(context.Background(), "org-1", "user-42")
	require.NoError(t, err)
	assert.Equal(t, []string{"user-42"}, r.called, "RevokeAllSessions must be called with userID after tx commit")
	assert.Equal(t, 2, tx.execCalled, "expect 2 Exec calls: DELETE org_member + UPDATE users")
}

func TestDeactivateUser_nilRevoker_safe(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}
	svc := &Service{db: db, sessionRevoker: nil}

	// Must not panic when sessionRevoker is nil.
	err := svc.DeactivateUser(context.Background(), "org-1", "user-42")
	require.NoError(t, err)
}

func TestDeactivateUser_revokerSkippedOnTxError(t *testing.T) {
	tx := &mockTx{execErr: errors.New("db error")}
	db := &mockDB{tx: tx}
	r := &mockRevoker{}
	svc := &Service{db: db, sessionRevoker: r}

	err := svc.DeactivateUser(context.Background(), "org-1", "user-42")
	assert.Error(t, err)
	assert.Empty(t, r.called, "RevokeAllSessions must NOT be called when the transaction fails")
}

func TestDeactivateUser_revokerErrorIsNonFatal(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}
	r := &mockRevoker{err: errors.New("redis down")}
	svc := &Service{db: db, sessionRevoker: r}

	// Revocation failure must not propagate — deactivation itself succeeded.
	err := svc.DeactivateUser(context.Background(), "org-1", "user-42")
	require.NoError(t, err, "revocation error is non-fatal; DeactivateUser must return nil")
	assert.Equal(t, []string{"user-42"}, r.called)
}

// ─── parseEqFilter ────────────────────────────────────────────────────────────

func TestParseEqFilter_simple(t *testing.T) {
	tests := []struct {
		filter string
		attr   string
		want   string
	}{
		{`userName eq "alice"`, "userName", "alice"},
		{`displayName eq "Team X"`, "displayName", "Team X"},
		{`userName eq alice`, "userName", "alice"},
		{``, "userName", ""},
		{`email eq "x@y.com"`, "userName", ""},
		{`USERNAME EQ "bob"`, "userName", "bob"},
		{`displayName eq ""`, "displayName", ""},
	}
	for _, tc := range tests {
		got := parseEqFilter(tc.filter, tc.attr)
		assert.Equal(t, tc.want, got, "filter=%q attr=%q", tc.filter, tc.attr)
	}
}

// ─── parseMemberValues ────────────────────────────────────────────────────────

func TestParseMemberValues_sliceOfMaps(t *testing.T) {
	input := []any{
		map[string]any{"value": "uid-1"},
		map[string]any{"value": "uid-2"},
		map[string]any{"nope": "uid-3"},
	}
	got := parseMemberValues(input)
	assert.Equal(t, []string{"uid-1", "uid-2"}, got)
}

func TestParseMemberValues_singleMap(t *testing.T) {
	got := parseMemberValues(map[string]any{"value": "uid-42"})
	assert.Equal(t, []string{"uid-42"}, got)
}

func TestParseMemberValues_nil(t *testing.T) {
	assert.Empty(t, parseMemberValues(nil))
}

func TestParseMemberValues_emptySlice(t *testing.T) {
	assert.Empty(t, parseMemberValues([]any{}))
}
