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

func setupTestServer(t *testing.T) (sekevev1.SekeveClient, func()) {
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
	return client, func() {
		conn.Close()
		store.Close(ctx)
	}
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
			name: "empty name fails",
			entry: &sekevev1.Entry{
				Name:    "",
				Type:    sekevev1.EntryType_ENTRY_TYPE_NOTE,
				Payload: []byte("note"),
			},
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

			// Retrieve the entry.
			got, err := client.GetEntry(ctx, &sekevev1.GetEntryRequest{Name: tc.entry.Name})
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
	_, err := client.CreateEntry(ctx, &sekevev1.CreateEntryRequest{
		Entry: &sekevev1.Entry{
			Name:    "to-delete",
			Type:    sekevev1.EntryType_ENTRY_TYPE_NOTE,
			Payload: []byte("bye"),
		},
	})
	require.NoError(t, err)

	t.Run("delete existing", func(t *testing.T) {
		_, err := client.DeleteEntry(ctx, &sekevev1.DeleteEntryRequest{Name: "to-delete"})
		require.NoError(t, err)
	})

	t.Run("delete non-existent returns NotFound", func(t *testing.T) {
		_, err := client.DeleteEntry(ctx, &sekevev1.DeleteEntryRequest{Name: "does-not-exist"})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestCreateDuplicate(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := authedCtx()

	entry := &sekevev1.CreateEntryRequest{
		Entry: &sekevev1.Entry{
			Name:    "duplicate",
			Type:    sekevev1.EntryType_ENTRY_TYPE_SECRET,
			Payload: []byte("data"),
		},
	}

	_, err := client.CreateEntry(ctx, entry)
	require.NoError(t, err)

	_, err = client.CreateEntry(ctx, entry)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, st.Code())
}
