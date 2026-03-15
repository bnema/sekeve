package port

import (
	"context"

	"github.com/bnema/sekeve/internal/domain/entity"
)

type StoragePort interface {
	Create(ctx context.Context, envelope *entity.Envelope) error
	Update(ctx context.Context, envelope *entity.Envelope) error
	Get(ctx context.Context, name string) (*entity.Envelope, error)
	List(ctx context.Context, entryType entity.EntryType) ([]*entity.Envelope, error)
	Delete(ctx context.Context, name string) error
	Close(ctx context.Context) error
}
