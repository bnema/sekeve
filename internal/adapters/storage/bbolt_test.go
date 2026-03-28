package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bnema/sekeve/internal/adapters/storage"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *storage.BboltStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewBboltStore(context.Background(), dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := store.Close(context.Background()); err != nil {
			t.Errorf("failed to close test store: %v", err)
		}
	})
	return store
}

func mustCreate(t testing.TB, s *storage.BboltStore, env *entity.Envelope) {
	t.Helper()
	require.NoError(t, s.Create(context.Background(), env))
}

func TestBboltStore_Create(t *testing.T) {
	tests := []struct {
		name     string
		envelope *entity.Envelope
		setup    func(testing.TB, *storage.BboltStore)
		wantErr  error
	}{
		{
			name:     "create new entry succeeds",
			envelope: &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("encrypted-data")},
		},
		{
			name:     "create duplicate name succeeds (names are not unique)",
			envelope: &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("encrypted-data-2")},
			setup: func(t testing.TB, s *storage.BboltStore) {
				mustCreate(t, s, &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("existing")})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			if tt.setup != nil {
				tt.setup(t, store)
			}
			err := store.Create(context.Background(), tt.envelope)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, tt.envelope.ID)
				assert.False(t, tt.envelope.CreatedAt.IsZero())
			}
		})
	}
}

func TestBboltStore_GetByID(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(testing.TB, *storage.BboltStore) string
		wantErr error
	}{
		{
			name: "get existing entry",
			setup: func(t testing.TB, s *storage.BboltStore) string {
				env := &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("encrypted")}
				mustCreate(t, s, env)
				return env.ID
			},
		},
		{name: "get non-existent entry", setup: func(_ testing.TB, _ *storage.BboltStore) string { return "non-existent-id" }, wantErr: port.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			id := tt.setup(t, store)
			envelope, err := store.GetByID(context.Background(), id)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, envelope)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, id, envelope.ID)
			}
		})
	}
}

func TestBboltStore_List(t *testing.T) {
	tests := []struct {
		name      string
		entryType entity.EntryType
		setup     func(testing.TB, *storage.BboltStore)
		wantCount int
	}{
		{name: "list all from empty store", entryType: entity.EntryTypeUnspecified, wantCount: 0},
		{
			name: "list all entries", entryType: entity.EntryTypeUnspecified,
			setup: func(t testing.TB, s *storage.BboltStore) {
				mustCreate(t, s, &entity.Envelope{Name: "gh", Type: entity.EntryTypeLogin, Payload: []byte("a")})
				mustCreate(t, s, &entity.Envelope{Name: "key", Type: entity.EntryTypeSecret, Payload: []byte("b")})
			},
			wantCount: 2,
		},
		{
			name: "list filtered by type", entryType: entity.EntryTypeLogin,
			setup: func(t testing.TB, s *storage.BboltStore) {
				mustCreate(t, s, &entity.Envelope{Name: "gh", Type: entity.EntryTypeLogin, Payload: []byte("a")})
				mustCreate(t, s, &entity.Envelope{Name: "key", Type: entity.EntryTypeSecret, Payload: []byte("b")})
			},
			wantCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			if tt.setup != nil {
				tt.setup(t, store)
			}
			entries, err := store.List(context.Background(), tt.entryType)
			assert.NoError(t, err)
			assert.Len(t, entries, tt.wantCount)
		})
	}
}

func TestBboltStore_DeleteByID(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(testing.TB, *storage.BboltStore) string
		wantErr error
	}{
		{
			name: "delete existing entry",
			setup: func(t testing.TB, s *storage.BboltStore) string {
				env := &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("a")}
				mustCreate(t, s, env)
				return env.ID
			},
		},
		{name: "delete non-existent entry", setup: func(_ testing.TB, _ *storage.BboltStore) string { return "missing-id" }, wantErr: port.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			id := tt.setup(t, store)
			err := store.DeleteByID(context.Background(), id)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				_, err := store.GetByID(context.Background(), id)
				assert.ErrorIs(t, err, port.ErrNotFound)
			}
		})
	}
}

func TestBboltStore_Update(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(testing.TB, *storage.BboltStore) *entity.Envelope
		update  func(*entity.Envelope)
		wantErr error
	}{
		{
			name: "update existing entry",
			setup: func(t testing.TB, s *storage.BboltStore) *entity.Envelope {
				env := &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("old")}
				mustCreate(t, s, env)
				return env
			},
			update: func(env *entity.Envelope) { env.Payload = []byte("new") },
		},
		{
			name: "update non-existent entry",
			setup: func(_ testing.TB, _ *storage.BboltStore) *entity.Envelope {
				return &entity.Envelope{ID: "fake-id", Name: "missing", Type: entity.EntryTypeLogin}
			},
			wantErr: port.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			env := tt.setup(t, store)
			if tt.update != nil {
				tt.update(env)
			}
			err := store.Update(context.Background(), env)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				got, err := store.GetByID(context.Background(), env.ID)
				require.NoError(t, err)
				assert.Equal(t, []byte("new"), got.Payload)
				assert.True(t, got.UpdatedAt.After(got.CreatedAt))
			}
		})
	}
}

func TestBboltStore_AuthKey(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetAuthKey(ctx)
	assert.ErrorIs(t, err, port.ErrNotFound)

	pubKey := []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\ntest\n-----END PGP PUBLIC KEY BLOCK-----")
	err = store.StoreAuthKey(ctx, pubKey)
	assert.NoError(t, err)

	got, err := store.GetAuthKey(ctx)
	assert.NoError(t, err)
	assert.Equal(t, pubKey, got)
}

func TestPINHash_StoreAndRetrieve(t *testing.T) {
	store := newTestStore(t)

	hash := []byte("testhash")
	salt := []byte("testsalt")

	err := store.StorePINHash(context.Background(), hash, salt)
	if err != nil {
		t.Fatal(err)
	}

	gotHash, gotSalt, err := store.GetPINHash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(gotHash) != string(hash) {
		t.Errorf("hash = %q, want %q", gotHash, hash)
	}
	if string(gotSalt) != string(salt) {
		t.Errorf("salt = %q, want %q", gotSalt, salt)
	}
}

func TestPINHash_NotSet(t *testing.T) {
	store := newTestStore(t)

	_, _, err := store.GetPINHash(context.Background())
	if err == nil {
		t.Fatal("expected error when PIN hash not set")
	}
}

func TestPINHash_Delete(t *testing.T) {
	store := newTestStore(t)

	_ = store.StorePINHash(context.Background(), []byte("h"), []byte("s"))
	err := store.DeletePINHash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.GetPINHash(context.Background())
	if err == nil {
		t.Fatal("expected error after delete")
	}
}
