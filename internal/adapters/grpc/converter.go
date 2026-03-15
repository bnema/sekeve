package grpc

import (
	"time"

	sekevev1 "github.com/bnema/sekeve/gen/proto/sekeve/v1"
	"github.com/bnema/sekeve/internal/domain/entity"
)

func envelopeToProto(env *entity.Envelope) *sekevev1.Entry {
	return &sekevev1.Entry{
		Id:        env.ID,
		Name:      env.Name,
		Type:      sekevev1.EntryType(env.Type),
		Meta:      env.Meta,
		Payload:   env.Payload,
		CreatedAt: env.CreatedAt.Unix(),
		UpdatedAt: env.UpdatedAt.Unix(),
	}
}

func protoToEnvelope(entry *sekevev1.Entry) *entity.Envelope {
	return &entity.Envelope{
		ID:        entry.Id,
		Name:      entry.Name,
		Type:      entity.EntryType(entry.Type),
		Meta:      entry.Meta,
		Payload:   entry.Payload,
		CreatedAt: time.Unix(entry.CreatedAt, 0),
		UpdatedAt: time.Unix(entry.UpdatedAt, 0),
	}
}
