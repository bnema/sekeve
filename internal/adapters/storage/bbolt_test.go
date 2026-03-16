package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bnema/sekeve/internal/adapters/storage"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/domain/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *storage.BboltStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewBboltStore(context.Background(), dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return store
}

func TestBboltStore_Create(t *testing.T) {
	tests := []struct {
		name     string
		envelope *entity.Envelope
		setup    func(*storage.BboltStore)
		wantErr  error
	}{
		{
			name:     "create new entry succeeds",
			envelope: &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("encrypted-data")},
		},
		{
			name:     "create duplicate name fails",
			envelope: &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("encrypted-data")},
			setup: func(s *storage.BboltStore) {
				_ = s.Create(context.Background(), &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("existing")})
			},
			wantErr: port.ErrAlreadyExists,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			if tt.setup != nil {
				tt.setup(store)
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

func TestBboltStore_Get(t *testing.T) {
	tests := []struct {
		name      string
		entryName string
		setup     func(*storage.BboltStore)
		wantErr   error
	}{
		{
			name: "get existing entry", entryName: "github",
			setup: func(s *storage.BboltStore) {
				_ = s.Create(context.Background(), &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("encrypted")})
			},
		},
		{name: "get non-existent entry", entryName: "missing", wantErr: port.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			if tt.setup != nil {
				tt.setup(store)
			}
			envelope, err := store.Get(context.Background(), tt.entryName)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, envelope)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.entryName, envelope.Name)
			}
		})
	}
}

func TestBboltStore_List(t *testing.T) {
	tests := []struct {
		name      string
		entryType entity.EntryType
		setup     func(*storage.BboltStore)
		wantCount int
	}{
		{name: "list all from empty store", entryType: entity.EntryTypeUnspecified, wantCount: 0},
		{
			name: "list all entries", entryType: entity.EntryTypeUnspecified,
			setup: func(s *storage.BboltStore) {
				_ = s.Create(context.Background(), &entity.Envelope{Name: "gh", Type: entity.EntryTypeLogin, Payload: []byte("a")})
				_ = s.Create(context.Background(), &entity.Envelope{Name: "key", Type: entity.EntryTypeSecret, Payload: []byte("b")})
			},
			wantCount: 2,
		},
		{
			name: "list filtered by type", entryType: entity.EntryTypeLogin,
			setup: func(s *storage.BboltStore) {
				_ = s.Create(context.Background(), &entity.Envelope{Name: "gh", Type: entity.EntryTypeLogin, Payload: []byte("a")})
				_ = s.Create(context.Background(), &entity.Envelope{Name: "key", Type: entity.EntryTypeSecret, Payload: []byte("b")})
			},
			wantCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			if tt.setup != nil {
				tt.setup(store)
			}
			entries, err := store.List(context.Background(), tt.entryType)
			assert.NoError(t, err)
			assert.Len(t, entries, tt.wantCount)
		})
	}
}

func TestBboltStore_Delete(t *testing.T) {
	tests := []struct {
		name      string
		entryName string
		setup     func(*storage.BboltStore)
		wantErr   error
	}{
		{
			name: "delete existing entry", entryName: "github",
			setup: func(s *storage.BboltStore) {
				_ = s.Create(context.Background(), &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("a")})
			},
		},
		{name: "delete non-existent entry", entryName: "missing", wantErr: port.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			if tt.setup != nil {
				tt.setup(store)
			}
			err := store.Delete(context.Background(), tt.entryName)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				_, err := store.Get(context.Background(), tt.entryName)
				assert.ErrorIs(t, err, port.ErrNotFound)
			}
		})
	}
}

func TestBboltStore_Update(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*storage.BboltStore) *entity.Envelope
		update  func(*entity.Envelope)
		wantErr error
	}{
		{
			name: "update existing entry",
			setup: func(s *storage.BboltStore) *entity.Envelope {
				env := &entity.Envelope{Name: "github", Type: entity.EntryTypeLogin, Payload: []byte("old")}
				_ = s.Create(context.Background(), env)
				return env
			},
			update: func(env *entity.Envelope) { env.Payload = []byte("new") },
		},
		{
			name: "update non-existent entry",
			setup: func(_ *storage.BboltStore) *entity.Envelope {
				return &entity.Envelope{ID: "fake-id", Name: "missing", Type: entity.EntryTypeLogin}
			},
			wantErr: port.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			env := tt.setup(store)
			if tt.update != nil {
				tt.update(env)
			}
			err := store.Update(context.Background(), env)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				got, err := store.Get(context.Background(), env.Name)
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

	// No key stored yet
	_, err := store.GetAuthKey(ctx)
	assert.ErrorIs(t, err, port.ErrNotFound)

	// Store a key
	pubKey := []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\ntest\n-----END PGP PUBLIC KEY BLOCK-----")
	err = store.StoreAuthKey(ctx, pubKey)
	assert.NoError(t, err)

	// Retrieve it
	got, err := store.GetAuthKey(ctx)
	assert.NoError(t, err)
	assert.Equal(t, pubKey, got)
}
