package port

import (
	"context"

	"github.com/bnema/sekeve/internal/domain/entity"
)

type StoragePort interface {
	Create(ctx context.Context, envelope *entity.Envelope) error
	Update(ctx context.Context, envelope *entity.Envelope) error
	GetByID(ctx context.Context, id string) (*entity.Envelope, error)
	List(ctx context.Context, entryType entity.EntryType) ([]*entity.Envelope, error)
	DeleteByID(ctx context.Context, id string) error
	Close(ctx context.Context) error
	StorePINHash(ctx context.Context, hash, salt []byte) error
	GetPINHash(ctx context.Context) (hash, salt []byte, err error)
	DeletePINHash(ctx context.Context) error
}
