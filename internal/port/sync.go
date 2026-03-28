package port

import (
	"context"
	"time"

	"github.com/bnema/sekeve/internal/domain/entity"
)

// AuthResult holds the outcome of a client Authenticate call.
type AuthResult struct {
	Token        string
	RequiresPIN  bool
	UnlockTicket string
	ExpiresAt    time.Time
}

type SyncPort interface {
	Authenticate(ctx context.Context, gpgKeyID string, crypto CryptoPort) (*AuthResult, error)
	CreateEntry(ctx context.Context, envelope *entity.Envelope) (string, error)
	UpdateEntry(ctx context.Context, envelope *entity.Envelope) error
	GetEntry(ctx context.Context, id string) (*entity.Envelope, error)
	ListEntries(ctx context.Context, entryType entity.EntryType) ([]*entity.Envelope, error)
	DeleteEntry(ctx context.Context, id string) error
	Close(ctx context.Context) error
}
