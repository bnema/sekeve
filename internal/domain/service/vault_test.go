package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/sekeve/internal/port/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/bnema/sekeve/internal/domain/service"
)

const testKeyID = "test-key-id"

func newService(t *testing.T, crypto *mocks.MockCryptoPort, sync *mocks.MockSyncPort) *service.VaultService {
	t.Helper()
	return service.NewVaultService(crypto, sync, testKeyID)
}

// ---------- AddEntry ----------

func TestAddEntry(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(crypto *mocks.MockCryptoPort, syncP *mocks.MockSyncPort, env *entity.Envelope)
		envelope  *entity.Envelope
		wantErr   error
	}{
		{
			name: "success",
			envelope: &entity.Envelope{
				Name:    "my-secret",
				Type:    entity.EntryTypeSecret,
				Payload: []byte("plaintext"),
			},
			setupMock: func(crypto *mocks.MockCryptoPort, syncP *mocks.MockSyncPort, env *entity.Envelope) {
				encrypted := []byte("encrypted")
				crypto.EXPECT().Encrypt(mock.Anything, env.Payload, testKeyID).Return(encrypted, nil)
				syncP.EXPECT().CreateEntry(mock.Anything, mock.AnythingOfType("*entity.Envelope")).Return("generated-id", nil)
			},
		},
		{
			name: "duplicate name error",
			envelope: &entity.Envelope{
				Name:    "existing-secret",
				Type:    entity.EntryTypeSecret,
				Payload: []byte("plaintext"),
			},
			setupMock: func(crypto *mocks.MockCryptoPort, syncP *mocks.MockSyncPort, env *entity.Envelope) {
				encrypted := []byte("encrypted")
				crypto.EXPECT().Encrypt(mock.Anything, env.Payload, testKeyID).Return(encrypted, nil)
				syncP.EXPECT().CreateEntry(mock.Anything, mock.AnythingOfType("*entity.Envelope")).Return("", port.ErrAlreadyExists)
			},
			wantErr: port.ErrAlreadyExists,
		},
		{
			name: "encrypt error",
			envelope: &entity.Envelope{
				Name:    "some-secret",
				Type:    entity.EntryTypeSecret,
				Payload: []byte("plaintext"),
			},
			setupMock: func(crypto *mocks.MockCryptoPort, _ *mocks.MockSyncPort, env *entity.Envelope) {
				crypto.EXPECT().Encrypt(mock.Anything, env.Payload, testKeyID).Return(nil, errors.New("gpg error"))
			},
			wantErr: errors.New("gpg error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cryptoMock := mocks.NewMockCryptoPort(t)
			syncMock := mocks.NewMockSyncPort(t)
			tc.setupMock(cryptoMock, syncMock, tc.envelope)

			svc := newService(t, cryptoMock, syncMock)
			err := svc.AddEntry(context.Background(), tc.envelope)

			if tc.wantErr != nil {
				assert.Error(t, err)
				if errors.Is(tc.wantErr, port.ErrAlreadyExists) || errors.Is(tc.wantErr, port.ErrNotFound) {
					assert.ErrorIs(t, err, tc.wantErr)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "generated-id", tc.envelope.ID)
			}
		})
	}
}

// ---------- GetEntry ----------

func TestGetEntry(t *testing.T) {
	tests := []struct {
		name      string
		entryName string
		setupMock func(syncP *mocks.MockSyncPort)
		wantErr   error
	}{
		{
			name:      "success",
			entryName: "my-secret",
			setupMock: func(syncP *mocks.MockSyncPort) {
				env := &entity.Envelope{ID: "abc", Name: "my-secret", Payload: []byte("cipher")}
				syncP.EXPECT().GetEntry(mock.Anything, "my-secret").Return(env, nil)
			},
		},
		{
			name:      "not found",
			entryName: "missing-secret",
			setupMock: func(syncP *mocks.MockSyncPort) {
				syncP.EXPECT().GetEntry(mock.Anything, "missing-secret").Return(nil, port.ErrNotFound)
			},
			wantErr: port.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cryptoMock := mocks.NewMockCryptoPort(t)
			syncMock := mocks.NewMockSyncPort(t)
			tc.setupMock(syncMock)

			svc := newService(t, cryptoMock, syncMock)
			env, err := svc.GetEntry(context.Background(), tc.entryName)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, env)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, env)
				assert.Equal(t, tc.entryName, env.Name)
			}
		})
	}
}

// ---------- ListEntries ----------

func TestListEntries(t *testing.T) {
	tests := []struct {
		name      string
		entryType entity.EntryType
		setupMock func(syncP *mocks.MockSyncPort)
		wantCount int
		wantErr   error
	}{
		{
			name:      "list all",
			entryType: entity.EntryTypeUnspecified,
			setupMock: func(syncP *mocks.MockSyncPort) {
				entries := []*entity.Envelope{
					{ID: "1", Name: "a", Type: entity.EntryTypeSecret},
					{ID: "2", Name: "b", Type: entity.EntryTypeLogin},
				}
				syncP.EXPECT().ListEntries(mock.Anything, entity.EntryTypeUnspecified).Return(entries, nil)
			},
			wantCount: 2,
		},
		{
			name:      "list by type",
			entryType: entity.EntryTypeLogin,
			setupMock: func(syncP *mocks.MockSyncPort) {
				entries := []*entity.Envelope{
					{ID: "2", Name: "b", Type: entity.EntryTypeLogin},
				}
				syncP.EXPECT().ListEntries(mock.Anything, entity.EntryTypeLogin).Return(entries, nil)
			},
			wantCount: 1,
		},
		{
			name:      "list error",
			entryType: entity.EntryTypeUnspecified,
			setupMock: func(syncP *mocks.MockSyncPort) {
				syncP.EXPECT().ListEntries(mock.Anything, entity.EntryTypeUnspecified).Return(nil, errors.New("remote error"))
			},
			wantErr: errors.New("remote error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cryptoMock := mocks.NewMockCryptoPort(t)
			syncMock := mocks.NewMockSyncPort(t)
			tc.setupMock(syncMock)

			svc := newService(t, cryptoMock, syncMock)
			entries, err := svc.ListEntries(context.Background(), tc.entryType)

			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.Nil(t, entries)
			} else {
				assert.NoError(t, err)
				assert.Len(t, entries, tc.wantCount)
			}
		})
	}
}

// ---------- DeleteEntry ----------

func TestDeleteEntry(t *testing.T) {
	tests := []struct {
		name      string
		entryName string
		setupMock func(syncP *mocks.MockSyncPort)
		wantErr   error
	}{
		{
			name:      "success",
			entryName: "my-secret",
			setupMock: func(syncP *mocks.MockSyncPort) {
				syncP.EXPECT().DeleteEntry(mock.Anything, "my-secret").Return(nil)
			},
		},
		{
			name:      "not found",
			entryName: "ghost-secret",
			setupMock: func(syncP *mocks.MockSyncPort) {
				syncP.EXPECT().DeleteEntry(mock.Anything, "ghost-secret").Return(port.ErrNotFound)
			},
			wantErr: port.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cryptoMock := mocks.NewMockCryptoPort(t)
			syncMock := mocks.NewMockSyncPort(t)
			tc.setupMock(syncMock)

			svc := newService(t, cryptoMock, syncMock)
			err := svc.DeleteEntry(context.Background(), tc.entryName)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------- UpdateEntry ----------

func TestUpdateEntry(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(crypto *mocks.MockCryptoPort, syncP *mocks.MockSyncPort, env *entity.Envelope)
		envelope  *entity.Envelope
		wantErr   error
	}{
		{
			name: "success",
			envelope: &entity.Envelope{
				ID:      "abc",
				Name:    "my-secret",
				Type:    entity.EntryTypeSecret,
				Payload: []byte("updated-plaintext"),
			},
			setupMock: func(crypto *mocks.MockCryptoPort, syncP *mocks.MockSyncPort, env *entity.Envelope) {
				encrypted := []byte("updated-encrypted")
				crypto.EXPECT().Encrypt(mock.Anything, env.Payload, testKeyID).Return(encrypted, nil)
				syncP.EXPECT().UpdateEntry(mock.Anything, mock.AnythingOfType("*entity.Envelope")).Return(nil)
			},
		},
		{
			name: "not found",
			envelope: &entity.Envelope{
				ID:      "xyz",
				Name:    "gone-secret",
				Type:    entity.EntryTypeSecret,
				Payload: []byte("plaintext"),
			},
			setupMock: func(crypto *mocks.MockCryptoPort, syncP *mocks.MockSyncPort, env *entity.Envelope) {
				encrypted := []byte("encrypted")
				crypto.EXPECT().Encrypt(mock.Anything, env.Payload, testKeyID).Return(encrypted, nil)
				syncP.EXPECT().UpdateEntry(mock.Anything, mock.AnythingOfType("*entity.Envelope")).Return(port.ErrNotFound)
			},
			wantErr: port.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cryptoMock := mocks.NewMockCryptoPort(t)
			syncMock := mocks.NewMockSyncPort(t)
			tc.setupMock(cryptoMock, syncMock, tc.envelope)

			svc := newService(t, cryptoMock, syncMock)
			err := svc.UpdateEntry(context.Background(), tc.envelope)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------- Authenticate ----------

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(crypto *mocks.MockCryptoPort, syncP *mocks.MockSyncPort)
		wantToken string
		wantErr   bool
	}{
		{
			name: "success",
			setupMock: func(crypto *mocks.MockCryptoPort, syncP *mocks.MockSyncPort) {
				syncP.EXPECT().Authenticate(mock.Anything, testKeyID, crypto).Return("session-token", nil)
			},
			wantToken: "session-token",
		},
		{
			name: "failure",
			setupMock: func(crypto *mocks.MockCryptoPort, syncP *mocks.MockSyncPort) {
				syncP.EXPECT().Authenticate(mock.Anything, testKeyID, crypto).Return("", errors.New("auth failed"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cryptoMock := mocks.NewMockCryptoPort(t)
			syncMock := mocks.NewMockSyncPort(t)
			tc.setupMock(cryptoMock, syncMock)

			svc := newService(t, cryptoMock, syncMock)
			token, err := svc.Authenticate(context.Background())

			if tc.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantToken, token)
			}
		})
	}
}

// ---------- DecryptAndUse ----------

func TestDecryptAndUse(t *testing.T) {
	t.Run("delegates to crypto port", func(t *testing.T) {
		cryptoMock := mocks.NewMockCryptoPort(t)
		syncMock := mocks.NewMockSyncPort(t)

		ciphertext := []byte("encrypted-data")
		var capturedPlain []byte
		fn := func(plain []byte) { capturedPlain = plain }

		cryptoMock.EXPECT().
			DecryptAndUse(mock.Anything, ciphertext, mock.AnythingOfType("func([]uint8)")).
			Return(nil)

		svc := newService(t, cryptoMock, syncMock)
		err := svc.DecryptAndUse(context.Background(), ciphertext, fn)

		assert.NoError(t, err)
		_ = capturedPlain // fn not actually called by mock, just verifying delegation
	})
}
