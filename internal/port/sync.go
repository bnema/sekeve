package port

import (
	"context"

	"github.com/bnema/sekeve/internal/domain/entity"
)

type SyncPort interface {
	Authenticate(ctx context.Context, gpgKeyID string, crypto CryptoPort) (token string, err error)
	CreateEntry(ctx context.Context, envelope *entity.Envelope) (string, error)
	UpdateEntry(ctx context.Context, envelope *entity.Envelope) error
	GetEntry(ctx context.Context, id string) (*entity.Envelope, error)
	ListEntries(ctx context.Context, entryType entity.EntryType) ([]*entity.Envelope, error)
	DeleteEntry(ctx context.Context, id string) error
	Close(ctx context.Context) error
}
