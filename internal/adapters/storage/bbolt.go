// Package storage provides a bbolt-backed implementation of port.StoragePort.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketEntries   = []byte("entries")
	bucketIndexType = []byte("index_type")
	bucketAuth      = []byte("auth")

	keyGPGPublicKey = []byte("gpg_public_key")
	keyPINHash      = []byte("pin_hash")
	keyPINSalt      = []byte("pin_salt")
)

// BboltStore implements port.StoragePort using bbolt (embedded key-value store).
type BboltStore struct {
	db *bolt.DB
}

// NewBboltStore opens (or creates) a bbolt database at the given path and
// initialises all required buckets.
func NewBboltStore(ctx context.Context, path string) (*BboltStore, error) {
	log := zerowrap.FromCtx(ctx)

	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, log.WrapErr(err, "failed to open bbolt database")
	}

	err = db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{bucketEntries, bucketIndexType, bucketAuth} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("create bucket %q: %w", bucket, err)
			}
		}
		return nil
	})
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close database after init error")
		}
		return nil, log.WrapErr(err, "failed to initialise buckets")
	}

	return &BboltStore{db: db}, nil
}

// Create stores a new Envelope, generating a UUIDv7 and setting timestamps.
func (s *BboltStore) Create(ctx context.Context, envelope *entity.Envelope) error {
	log := zerowrap.FromCtx(ctx)

	id, err := uuid.NewV7()
	if err != nil {
		return log.WrapErr(err, "failed to generate UUID")
	}

	now := time.Now().UTC()
	envelope.ID = id.String()
	envelope.CreatedAt = now
	envelope.UpdatedAt = now

	data, err := json.Marshal(envelope)
	if err != nil {
		return log.WrapErr(err, "failed to marshal envelope")
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		bEntries := tx.Bucket(bucketEntries)
		if err := bEntries.Put([]byte(envelope.ID), data); err != nil {
			return fmt.Errorf("put entries: %w", err)
		}

		bType := tx.Bucket(bucketIndexType)
		typeKey := typeIndexKey(envelope)
		if err := bType.Put(typeKey, []byte(envelope.ID)); err != nil {
			return fmt.Errorf("put index_type: %w", err)
		}

		return nil
	})
	if err != nil {
		return log.WrapErr(err, "failed to create entry")
	}

	return nil
}

// Update overwrites an existing Envelope identified by its ID, refreshing UpdatedAt.
// Returns port.ErrNotFound when the ID does not exist.
func (s *BboltStore) Update(ctx context.Context, envelope *entity.Envelope) error {
	log := zerowrap.FromCtx(ctx)

	err := s.db.Update(func(tx *bolt.Tx) error {
		bEntries := tx.Bucket(bucketEntries)

		existing := bEntries.Get([]byte(envelope.ID))
		if existing == nil {
			return port.ErrNotFound
		}

		// Decode old entry to check if type changed.
		var oldEnv entity.Envelope
		if err := json.Unmarshal(existing, &oldEnv); err != nil {
			return log.WrapErr(err, "failed to unmarshal existing entry")
		}

		// Update type index if type changed.
		typeIdx := tx.Bucket(bucketIndexType)
		if oldEnv.Type != envelope.Type {
			oldTypeKey := fmt.Sprintf("%d:%s", oldEnv.Type, oldEnv.ID)
			if err := typeIdx.Delete([]byte(oldTypeKey)); err != nil {
				return fmt.Errorf("delete old index_type: %w", err)
			}
			newTypeKey := fmt.Sprintf("%d:%s", envelope.Type, envelope.ID)
			if err := typeIdx.Put([]byte(newTypeKey), []byte(envelope.ID)); err != nil {
				return fmt.Errorf("put index_type: %w", err)
			}
		}

		// Preserve CreatedAt from old entry and update UpdatedAt.
		envelope.CreatedAt = oldEnv.CreatedAt
		envelope.UpdatedAt = time.Now().UTC()

		data, err := json.Marshal(envelope)
		if err != nil {
			return log.WrapErr(err, "failed to marshal envelope")
		}

		if err := bEntries.Put([]byte(envelope.ID), data); err != nil {
			return fmt.Errorf("put entries: %w", err)
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return port.ErrNotFound
		}
		return log.WrapErr(err, "failed to update entry")
	}

	return nil
}

// GetByID retrieves an Envelope by its ID.
// Returns port.ErrNotFound when no entry with that ID exists.
func (s *BboltStore) GetByID(ctx context.Context, id string) (*entity.Envelope, error) {
	log := zerowrap.FromCtx(ctx)

	var envelope entity.Envelope

	err := s.db.View(func(tx *bolt.Tx) error {
		bEntries := tx.Bucket(bucketEntries)
		data := bEntries.Get([]byte(id))
		if data == nil {
			return port.ErrNotFound
		}

		dataCopy := make([]byte, len(data))
		copy(dataCopy, data)

		return json.Unmarshal(dataCopy, &envelope)
	})
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, port.ErrNotFound
		}
		return nil, log.WrapErr(err, "failed to get entry")
	}

	return &envelope, nil
}

// List returns all Envelopes, or only those matching entryType when it is not
// EntryTypeUnspecified.
func (s *BboltStore) List(ctx context.Context, entryType entity.EntryType) ([]*entity.Envelope, error) {
	log := zerowrap.FromCtx(ctx)

	var envelopes []*entity.Envelope

	err := s.db.View(func(tx *bolt.Tx) error {
		bEntries := tx.Bucket(bucketEntries)

		if entryType == entity.EntryTypeUnspecified {
			// Iterate all entries.
			return bEntries.ForEach(func(_, v []byte) error {
				dataCopy := make([]byte, len(v))
				copy(dataCopy, v)

				var env entity.Envelope
				if err := json.Unmarshal(dataCopy, &env); err != nil {
					return err
				}
				envelopes = append(envelopes, &env)
				return nil
			})
		}

		// Prefix scan on index_type: keys are "<type>:<id>".
		bType := tx.Bucket(bucketIndexType)
		prefix := []byte(fmt.Sprintf("%d:", int(entryType)))
		c := bType.Cursor()

		for k, idVal := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, idVal = c.Next() {
			id := make([]byte, len(idVal))
			copy(id, idVal)

			data := bEntries.Get(id)
			if data == nil {
				continue
			}
			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)

			var env entity.Envelope
			if err := json.Unmarshal(dataCopy, &env); err != nil {
				return err
			}
			envelopes = append(envelopes, &env)
		}

		return nil
	})
	if err != nil {
		return nil, log.WrapErr(err, "failed to list entries")
	}

	if envelopes == nil {
		envelopes = []*entity.Envelope{}
	}

	return envelopes, nil
}

// DeleteByID removes an Envelope by its ID and all its index entries.
// Returns port.ErrNotFound when no entry with that ID exists.
func (s *BboltStore) DeleteByID(ctx context.Context, id string) error {
	log := zerowrap.FromCtx(ctx)

	err := s.db.Update(func(tx *bolt.Tx) error {
		bEntries := tx.Bucket(bucketEntries)

		data := bEntries.Get([]byte(id))
		if data == nil {
			return port.ErrNotFound
		}

		dataCopy := make([]byte, len(data))
		copy(dataCopy, data)

		var env entity.Envelope
		if err := json.Unmarshal(dataCopy, &env); err != nil {
			log.Warn().Err(err).Str("id", id).Msg("failed to unmarshal entry during delete, type index may be stale")
		} else {
			bType := tx.Bucket(bucketIndexType)
			typeKey := typeIndexKey(&env)
			if err := bType.Delete(typeKey); err != nil {
				return fmt.Errorf("delete index_type: %w", err)
			}
		}

		if err := bEntries.Delete([]byte(id)); err != nil {
			return fmt.Errorf("delete entries: %w", err)
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return port.ErrNotFound
		}
		return log.WrapErr(err, "failed to delete entry")
	}

	return nil
}

// StoreAuthKey persists a GPG public key in the auth bucket.
func (s *BboltStore) StoreAuthKey(ctx context.Context, pubKey []byte) error {
	log := zerowrap.FromCtx(ctx)

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAuth)
		return b.Put(keyGPGPublicKey, pubKey)
	})
	if err != nil {
		return log.WrapErr(err, "failed to store auth key")
	}

	return nil
}

// GetAuthKey retrieves the stored GPG public key.
// Returns port.ErrNotFound when no key has been stored yet.
func (s *BboltStore) GetAuthKey(ctx context.Context) ([]byte, error) {
	log := zerowrap.FromCtx(ctx)

	var key []byte

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAuth)
		v := b.Get(keyGPGPublicKey)
		if v == nil {
			return port.ErrNotFound
		}
		key = make([]byte, len(v))
		copy(key, v)
		return nil
	})
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, port.ErrNotFound
		}
		return nil, log.WrapErr(err, "failed to get auth key")
	}

	return key, nil
}

// StorePINHash persists a PIN hash and salt in the auth bucket.
func (s *BboltStore) StorePINHash(_ context.Context, hash, salt []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAuth)
		if err := b.Put(keyPINHash, hash); err != nil {
			return err
		}
		return b.Put(keyPINSalt, salt)
	})
}

// GetPINHash retrieves the stored PIN hash and salt.
// Returns an error when no PIN has been configured yet.
func (s *BboltStore) GetPINHash(_ context.Context) (hash, salt []byte, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAuth)
		h := b.Get(keyPINHash)
		sv := b.Get(keyPINSalt)
		if h == nil || sv == nil {
			return fmt.Errorf("PIN not configured")
		}
		hash = make([]byte, len(h))
		copy(hash, h)
		salt = make([]byte, len(sv))
		copy(salt, sv)
		return nil
	})
	return
}

// DeletePINHash removes the PIN hash and salt from the auth bucket.
func (s *BboltStore) DeletePINHash(_ context.Context) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAuth)
		if err := b.Delete(keyPINHash); err != nil {
			return err
		}
		return b.Delete(keyPINSalt)
	})
}

// Close closes the underlying bbolt database.
func (s *BboltStore) Close(_ context.Context) error {
	return s.db.Close()
}

// typeIndexKey returns the composite key used in the type index.
// Format: "<type_int>:<id>"
func typeIndexKey(envelope *entity.Envelope) []byte {
	return []byte(fmt.Sprintf("%d:%s", envelope.Type, envelope.ID))
}
