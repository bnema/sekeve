// Package storage provides a bbolt-backed implementation of port.StoragePort.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/domain/port"
	"github.com/bnema/zerowrap"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketEntries   = []byte("entries")
	bucketIndexName = []byte("index_name")
	bucketIndexType = []byte("index_type")
	bucketAuth      = []byte("auth")

	keyGPGPublicKey = []byte("gpg_public_key")
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
		for _, bucket := range [][]byte{bucketEntries, bucketIndexName, bucketIndexType, bucketAuth} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("create bucket %q: %w", bucket, err)
			}
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, log.WrapErr(err, "failed to initialise buckets")
	}

	return &BboltStore{db: db}, nil
}

// Create stores a new Envelope, generating a UUIDv7 and setting timestamps.
// Returns port.ErrAlreadyExists when the name is already taken.
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
		bName := tx.Bucket(bucketIndexName)

		if existing := bName.Get([]byte(envelope.Name)); existing != nil {
			return port.ErrAlreadyExists
		}

		bEntries := tx.Bucket(bucketEntries)
		if err := bEntries.Put([]byte(envelope.ID), data); err != nil {
			return fmt.Errorf("put entries: %w", err)
		}

		if err := bName.Put([]byte(envelope.Name), []byte(envelope.ID)); err != nil {
			return fmt.Errorf("put index_name: %w", err)
		}

		bType := tx.Bucket(bucketIndexType)
		typeKey := typeIndexKey(envelope)
		if err := bType.Put(typeKey, []byte(envelope.ID)); err != nil {
			return fmt.Errorf("put index_type: %w", err)
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, port.ErrAlreadyExists) {
			return port.ErrAlreadyExists
		}
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
		if oldEnv.Type != envelope.Type || oldEnv.Name != envelope.Name {
			oldTypeKey := fmt.Sprintf("%d:%s", oldEnv.Type, oldEnv.Name)
			if err := typeIdx.Delete([]byte(oldTypeKey)); err != nil {
				return fmt.Errorf("delete old index_type: %w", err)
			}
			newTypeKey := fmt.Sprintf("%d:%s", envelope.Type, envelope.Name)
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

// Get retrieves an Envelope by name.
// Returns port.ErrNotFound when no entry with that name exists.
func (s *BboltStore) Get(ctx context.Context, name string) (*entity.Envelope, error) {
	log := zerowrap.FromCtx(ctx)

	var envelope entity.Envelope

	err := s.db.View(func(tx *bolt.Tx) error {
		bName := tx.Bucket(bucketIndexName)

		idBytes := bName.Get([]byte(name))
		if idBytes == nil {
			return port.ErrNotFound
		}

		// Copy the ID since bbolt values are only valid within the transaction.
		id := make([]byte, len(idBytes))
		copy(id, idBytes)

		bEntries := tx.Bucket(bucketEntries)
		data := bEntries.Get(id)
		if data == nil {
			return port.ErrNotFound
		}

		// Copy data before unmarshalling.
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

		// Prefix scan on index_type: keys are "<type>:<name>".
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

// Delete removes an Envelope and all its index entries.
// Returns port.ErrNotFound when no entry with that name exists.
func (s *BboltStore) Delete(ctx context.Context, name string) error {
	log := zerowrap.FromCtx(ctx)

	err := s.db.Update(func(tx *bolt.Tx) error {
		bName := tx.Bucket(bucketIndexName)

		idBytes := bName.Get([]byte(name))
		if idBytes == nil {
			return port.ErrNotFound
		}

		id := make([]byte, len(idBytes))
		copy(id, idBytes)

		// Fetch entry to get type for type-index removal.
		bEntries := tx.Bucket(bucketEntries)
		data := bEntries.Get(id)
		if data != nil {
			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)

			var env entity.Envelope
			if err := json.Unmarshal(dataCopy, &env); err != nil {
				log.Warn().Err(err).Str("name", name).Msg("failed to unmarshal entry during delete, type index may be stale")
			} else {
				bType := tx.Bucket(bucketIndexType)
				typeKey := typeIndexKey(&env)
				_ = bType.Delete(typeKey)
			}
		}

		if err := bEntries.Delete(id); err != nil {
			return fmt.Errorf("delete entries: %w", err)
		}

		if err := bName.Delete([]byte(name)); err != nil {
			return fmt.Errorf("delete index_name: %w", err)
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

// Close closes the underlying bbolt database.
func (s *BboltStore) Close(_ context.Context) error {
	return s.db.Close()
}

// typeIndexKey returns the composite key used in the type index.
// Format: "<type_int>:<name>"
func typeIndexKey(envelope *entity.Envelope) []byte {
	return []byte(fmt.Sprintf("%d:%s", envelope.Type, envelope.Name))
}
