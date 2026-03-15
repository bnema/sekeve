package service

import (
	"context"

	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/domain/port"
	"github.com/bnema/zerowrap"
)

type VaultService struct {
	crypto   port.CryptoPort
	sync     port.SyncPort
	gpgKeyID string
}

func NewVaultService(crypto port.CryptoPort, sync port.SyncPort, gpgKeyID string) *VaultService {
	return &VaultService{
		crypto:   crypto,
		sync:     sync,
		gpgKeyID: gpgKeyID,
	}
}

func (s *VaultService) AddEntry(ctx context.Context, envelope *entity.Envelope) error {
	log := zerowrap.FromCtx(ctx)
	ctx = zerowrap.CtxWithFields(ctx, map[string]any{
		zerowrap.FieldLayer:   "domain",
		zerowrap.FieldUseCase: "AddEntry",
	})

	encrypted, err := s.crypto.Encrypt(ctx, envelope.Payload, s.gpgKeyID)
	if err != nil {
		return log.WrapErr(err, "failed to encrypt payload")
	}
	envelope.Payload = encrypted

	id, err := s.sync.CreateEntry(ctx, envelope)
	if err != nil {
		return log.WrapErr(err, "failed to create entry")
	}
	envelope.ID = id
	return nil
}

func (s *VaultService) GetEntry(ctx context.Context, name string) (*entity.Envelope, error) {
	log := zerowrap.FromCtx(ctx)
	ctx = zerowrap.CtxWithFields(ctx, map[string]any{
		zerowrap.FieldLayer:   "domain",
		zerowrap.FieldUseCase: "GetEntry",
	})

	envelope, err := s.sync.GetEntry(ctx, name)
	if err != nil {
		return nil, log.WrapErr(err, "failed to get entry")
	}
	return envelope, nil
}

func (s *VaultService) DecryptAndUse(ctx context.Context, ciphertext []byte, fn func(plaintext []byte)) error {
	return s.crypto.DecryptAndUse(ctx, ciphertext, fn)
}

func (s *VaultService) ListEntries(ctx context.Context, entryType entity.EntryType) ([]*entity.Envelope, error) {
	log := zerowrap.FromCtx(ctx)
	ctx = zerowrap.CtxWithFields(ctx, map[string]any{
		zerowrap.FieldLayer:   "domain",
		zerowrap.FieldUseCase: "ListEntries",
	})

	entries, err := s.sync.ListEntries(ctx, entryType)
	if err != nil {
		return nil, log.WrapErr(err, "failed to list entries")
	}
	return entries, nil
}

func (s *VaultService) DeleteEntry(ctx context.Context, name string) error {
	log := zerowrap.FromCtx(ctx)
	ctx = zerowrap.CtxWithFields(ctx, map[string]any{
		zerowrap.FieldLayer:   "domain",
		zerowrap.FieldUseCase: "DeleteEntry",
	})

	if err := s.sync.DeleteEntry(ctx, name); err != nil {
		return log.WrapErr(err, "failed to delete entry")
	}
	return nil
}

func (s *VaultService) UpdateEntry(ctx context.Context, envelope *entity.Envelope) error {
	log := zerowrap.FromCtx(ctx)
	ctx = zerowrap.CtxWithFields(ctx, map[string]any{
		zerowrap.FieldLayer:   "domain",
		zerowrap.FieldUseCase: "UpdateEntry",
	})

	encrypted, err := s.crypto.Encrypt(ctx, envelope.Payload, s.gpgKeyID)
	if err != nil {
		return log.WrapErr(err, "failed to encrypt payload")
	}
	envelope.Payload = encrypted

	if err := s.sync.UpdateEntry(ctx, envelope); err != nil {
		return log.WrapErr(err, "failed to update entry")
	}
	return nil
}

func (s *VaultService) Authenticate(ctx context.Context) (string, error) {
	log := zerowrap.FromCtx(ctx)
	ctx = zerowrap.CtxWithFields(ctx, map[string]any{
		zerowrap.FieldLayer:   "domain",
		zerowrap.FieldUseCase: "Authenticate",
	})

	token, err := s.sync.Authenticate(ctx, s.gpgKeyID, s.crypto)
	if err != nil {
		return "", log.WrapErr(err, "authentication failed")
	}
	return token, nil
}
