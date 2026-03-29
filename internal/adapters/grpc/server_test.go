package grpc_test

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	sekevev1 "github.com/bnema/sekeve/gen/proto/sekeve/v1"
	grpcadapter "github.com/bnema/sekeve/internal/adapters/grpc"
	"github.com/bnema/sekeve/internal/adapters/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func setupTestServerWithAuth(t *testing.T) (sekevev1.SekeveClient, *grpcadapter.AuthManager, func()) {
	t.Helper()

	ctx := context.Background()
	store, err := storage.NewBboltStore(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)

	auth := grpcadapter.NewAuthManager([]byte("test-public-key"))
	auth.SetTestToken("test-token")

	srv := grpcadapter.NewServer(ctx, store, auth)

	lis := bufconn.Listen(1024 * 1024)
	go func() {
		_ = srv.ServeListener(ctx, lis)
	}()

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := sekevev1.NewSekeveClient(conn)
	return client, auth, func() {
		require.NoError(t, conn.Close())
		require.NoError(t, store.Close(ctx))
	}
}

func setupTestServer(t *testing.T) (sekevev1.SekeveClient, func()) {
	client, _, cleanup := setupTestServerWithAuth(t)
	return client, cleanup
}

// authedCtx returns a context with "authorization: test-token" in outgoing metadata.
func authedCtx() context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "test-token"))
}

func TestCreateAndGet(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name    string
		entry   *sekevev1.Entry
		wantErr codes.Code
	}{
		{
			name: "valid entry",
			entry: &sekevev1.Entry{
				Name:    "my-secret",
				Type:    sekevev1.EntryType_ENTRY_TYPE_SECRET,
				Payload: []byte("hunter2"),
				Meta:    map[string]string{"env": "prod"},
			},
		},
		{
			name:    "nil entry fails",
			entry:   nil,
			wantErr: codes.InvalidArgument,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := authedCtx()
			resp, err := client.CreateEntry(ctx, &sekevev1.CreateEntryRequest{Entry: tc.entry})

			if tc.wantErr != codes.OK {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tc.wantErr, st.Code())
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, resp.Id)

			// Retrieve the entry by ID.
			got, err := client.GetEntry(ctx, &sekevev1.GetEntryRequest{Id: resp.Id})
			require.NoError(t, err)
			assert.Equal(t, tc.entry.Name, got.Name)
			assert.Equal(t, tc.entry.Type, got.Type)
			assert.Equal(t, tc.entry.Payload, got.Payload)
		})
	}
}

func TestListEntries(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := authedCtx()

	// Seed some entries.
	entries := []*sekevev1.Entry{
		{Name: "login1", Type: sekevev1.EntryType_ENTRY_TYPE_LOGIN, Payload: []byte("pass1")},
		{Name: "login2", Type: sekevev1.EntryType_ENTRY_TYPE_LOGIN, Payload: []byte("pass2")},
		{Name: "secret1", Type: sekevev1.EntryType_ENTRY_TYPE_SECRET, Payload: []byte("s3cr3t")},
	}
	for _, e := range entries {
		_, err := client.CreateEntry(ctx, &sekevev1.CreateEntryRequest{Entry: e})
		require.NoError(t, err)
	}

	t.Run("list all", func(t *testing.T) {
		resp, err := client.ListEntries(ctx, &sekevev1.ListEntriesRequest{Type: sekevev1.EntryType_ENTRY_TYPE_UNSPECIFIED})
		require.NoError(t, err)
		assert.Len(t, resp.Entries, 3)
	})

	t.Run("list by type login", func(t *testing.T) {
		resp, err := client.ListEntries(ctx, &sekevev1.ListEntriesRequest{Type: sekevev1.EntryType_ENTRY_TYPE_LOGIN})
		require.NoError(t, err)
		assert.Len(t, resp.Entries, 2)
	})

	t.Run("list empty type note", func(t *testing.T) {
		resp, err := client.ListEntries(ctx, &sekevev1.ListEntriesRequest{Type: sekevev1.EntryType_ENTRY_TYPE_NOTE})
		require.NoError(t, err)
		assert.Empty(t, resp.Entries)
	})
}

func TestDeleteEntry(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := authedCtx()

	// Create entry to delete.
	resp, err := client.CreateEntry(ctx, &sekevev1.CreateEntryRequest{
		Entry: &sekevev1.Entry{
			Name:    "to-delete",
			Type:    sekevev1.EntryType_ENTRY_TYPE_NOTE,
			Payload: []byte("bye"),
		},
	})
	require.NoError(t, err)

	t.Run("delete existing", func(t *testing.T) {
		_, err := client.DeleteEntry(ctx, &sekevev1.DeleteEntryRequest{Id: resp.Id})
		require.NoError(t, err)
	})

	t.Run("delete non-existent returns NotFound", func(t *testing.T) {
		_, err := client.DeleteEntry(ctx, &sekevev1.DeleteEntryRequest{Id: "does-not-exist"})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestCreateDuplicateNameAllowed(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := authedCtx()

	entry := &sekevev1.Entry{
		Name:    "duplicate",
		Type:    sekevev1.EntryType_ENTRY_TYPE_SECRET,
		Payload: []byte("data"),
	}

	resp1, err := client.CreateEntry(ctx, &sekevev1.CreateEntryRequest{Entry: entry})
	require.NoError(t, err)

	resp2, err := client.CreateEntry(ctx, &sekevev1.CreateEntryRequest{Entry: entry})
	require.NoError(t, err)

	assert.NotEqual(t, resp1.Id, resp2.Id)
}

// --- HasPIN tests ---

func TestHasPIN_NoPIN(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := client.HasPIN(context.Background(), &sekevev1.HasPINRequest{})
	require.NoError(t, err)
	assert.False(t, resp.HasPin)
}

func TestHasPIN_AfterSetPIN(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := authedCtx()
	_, err := client.SetPIN(ctx, &sekevev1.SetPINRequest{NewPin: "1234"})
	require.NoError(t, err)

	resp, err := client.HasPIN(context.Background(), &sekevev1.HasPINRequest{})
	require.NoError(t, err)
	assert.True(t, resp.HasPin)
}

// --- SetPIN tests ---

func TestSetPIN_FirstTime(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := authedCtx()
	_, err := client.SetPIN(ctx, &sekevev1.SetPINRequest{NewPin: "1234"})
	require.NoError(t, err)

	resp, err := client.HasPIN(context.Background(), &sekevev1.HasPINRequest{})
	require.NoError(t, err)
	assert.True(t, resp.HasPin)
}

func TestSetPIN_ChangePIN(t *testing.T) {
	client, auth, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	ctx := authedCtx()
	// Set initial PIN.
	_, err := client.SetPIN(ctx, &sekevev1.SetPINRequest{NewPin: "1234"})
	require.NoError(t, err)

	// SetPIN invalidates all sessions; re-authenticate for the next call.
	auth.SetTestToken("test-token")

	// Change PIN — must supply correct current PIN.
	_, err = client.SetPIN(ctx, &sekevev1.SetPINRequest{CurrentPin: "1234", NewPin: "5678"})
	require.NoError(t, err)
}

func TestSetPIN_WrongCurrentPIN(t *testing.T) {
	client, auth, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	ctx := authedCtx()
	_, err := client.SetPIN(ctx, &sekevev1.SetPINRequest{NewPin: "1234"})
	require.NoError(t, err)

	// SetPIN invalidates all sessions; re-authenticate for the next call.
	auth.SetTestToken("test-token")

	_, err = client.SetPIN(ctx, &sekevev1.SetPINRequest{CurrentPin: "wrong", NewPin: "5678"})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestSetPIN_TooShort(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := authedCtx()
	_, err := client.SetPIN(ctx, &sekevev1.SetPINRequest{NewPin: "12"})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestSetPIN_NonDigitRejected(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := authedCtx()
	_, err := client.SetPIN(ctx, &sekevev1.SetPINRequest{NewPin: "abcd"})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- Unlock tests ---

func TestUnlock_ValidTicketAndPIN(t *testing.T) {
	client, auth, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	ctx := authedCtx()
	// Set PIN first.
	_, err := client.SetPIN(ctx, &sekevev1.SetPINRequest{NewPin: "5678"})
	require.NoError(t, err)

	// Generate a challenge nonce and verify it to receive an unlock ticket.
	nonce, err := auth.GenerateChallenge(context.Background())
	require.NoError(t, err)
	result, err := auth.VerifyNonce(context.Background(), nonce)
	require.NoError(t, err)
	require.True(t, result.RequiresPIN)

	// Unlock with valid ticket and correct PIN.
	resp, err := client.Unlock(context.Background(), &sekevev1.UnlockRequest{
		UnlockTicket: result.UnlockTicket,
		Pin:          "5678",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.NotZero(t, resp.ExpiresAt)
}

func TestUnlock_WrongPIN(t *testing.T) {
	client, auth, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	ctx := authedCtx()
	_, err := client.SetPIN(ctx, &sekevev1.SetPINRequest{NewPin: "5678"})
	require.NoError(t, err)

	nonce, err := auth.GenerateChallenge(context.Background())
	require.NoError(t, err)
	result, err := auth.VerifyNonce(context.Background(), nonce)
	require.NoError(t, err)

	_, err = client.Unlock(context.Background(), &sekevev1.UnlockRequest{
		UnlockTicket: result.UnlockTicket,
		Pin:          "0000",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestCreateEntry_OversizedPayload(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := authedCtx()
	bigPayload := make([]byte, 2*1024*1024) // 2MB, over the 1MB limit
	_, err := client.CreateEntry(ctx, &sekevev1.CreateEntryRequest{
		Entry: &sekevev1.Entry{
			Name:    "big",
			Type:    sekevev1.EntryType_ENTRY_TYPE_NOTE,
			Payload: bigPayload,
		},
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestUnlock_NoPINConfigured(t *testing.T) {
	client, auth, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	// Force pinConfigured so VerifyNonce returns an unlock ticket, but no PIN is stored.
	auth.SetPINConfigured(true)

	nonce, err := auth.GenerateChallenge(context.Background())
	require.NoError(t, err)
	result, err := auth.VerifyNonce(context.Background(), nonce)
	require.NoError(t, err)

	_, err = client.Unlock(context.Background(), &sekevev1.UnlockRequest{
		UnlockTicket: result.UnlockTicket,
		Pin:          "1234",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestUnlock_WrongPIN_ConsumesTicket(t *testing.T) {
	client, auth, cleanup := setupTestServerWithAuth(t)
	defer cleanup()

	ctx := authedCtx()
	_, err := client.SetPIN(ctx, &sekevev1.SetPINRequest{NewPin: "5678"})
	require.NoError(t, err)

	nonce, err := auth.GenerateChallenge(context.Background())
	require.NoError(t, err)
	result, err := auth.VerifyNonce(context.Background(), nonce)
	require.NoError(t, err)

	// First attempt: wrong PIN — should fail and consume the ticket.
	_, err = client.Unlock(context.Background(), &sekevev1.UnlockRequest{
		UnlockTicket: result.UnlockTicket,
		Pin:          "0000",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())

	// Second attempt: correct PIN, same ticket — should fail (ticket consumed).
	auth.ResetPINFailures() // clear rate limit for test clarity
	_, err = client.Unlock(context.Background(), &sekevev1.UnlockRequest{
		UnlockTicket: result.UnlockTicket,
		Pin:          "5678",
	})
	require.Error(t, err)
	st, _ = status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}
