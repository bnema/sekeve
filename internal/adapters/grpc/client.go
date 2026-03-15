package grpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	sekevev1 "github.com/bnema/sekeve/gen/proto/sekeve/v1"
)

type Client struct {
	conn   *grpc.ClientConn
	client sekevev1.SekeveClient
	token  string
}

func NewClient(ctx context.Context, addr string) (*Client, error) {
	log := zerowrap.FromCtx(ctx)

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, log.WrapErr(err, "failed to connect to server")
	}

	return &Client{
		conn:   conn,
		client: sekevev1.NewSekeveClient(conn),
	}, nil
}

func (c *Client) authedCtx(ctx context.Context) context.Context {
	if c.token == "" {
		return ctx
	}
	md := metadata.Pairs("authorization", c.token)
	return metadata.NewOutgoingContext(ctx, md)
}

func (c *Client) SetToken(token string) {
	c.token = token
}

func (c *Client) Authenticate(ctx context.Context, gpgKeyID string, crypto port.CryptoPort) (string, error) {
	log := zerowrap.FromCtx(ctx)
	ctx = zerowrap.CtxWithFields(ctx, map[string]any{
		zerowrap.FieldAdapter: "grpc-client",
		zerowrap.FieldAction:  "authenticate",
	})

	challenge, err := c.client.Authenticate(ctx, &sekevev1.AuthRequest{GpgKeyId: gpgKeyID})
	if err != nil {
		return "", log.WrapErr(err, "failed to get challenge")
	}

	var nonce string
	err = crypto.DecryptAndUse(ctx, challenge.EncryptedChallenge, func(plaintext []byte) {
		parts := strings.SplitN(string(plaintext), ":", 3)
		if len(parts) != 3 || parts[0] != "sekeve-challenge" {
			return
		}
		nonce = parts[1]
	})
	if err != nil {
		return "", log.WrapErr(err, "failed to decrypt challenge")
	}
	if nonce == "" {
		return "", fmt.Errorf("invalid challenge format")
	}

	tokenResp, err := c.client.VerifyChallenge(ctx, &sekevev1.ChallengeResponse{Nonce: nonce})
	if err != nil {
		return "", log.WrapErr(err, "failed to verify challenge")
	}

	c.token = tokenResp.Token
	return tokenResp.Token, nil
}

func (c *Client) CreateEntry(ctx context.Context, envelope *entity.Envelope) (string, error) {
	log := zerowrap.FromCtx(ctx)
	ctx = c.authedCtx(ctx)
	resp, err := c.client.CreateEntry(ctx, &sekevev1.CreateEntryRequest{Entry: envelopeToProto(envelope)})
	if err != nil {
		return "", log.WrapErr(err, "failed to create entry")
	}
	return resp.Id, nil
}

func (c *Client) UpdateEntry(ctx context.Context, envelope *entity.Envelope) error {
	log := zerowrap.FromCtx(ctx)
	ctx = c.authedCtx(ctx)
	_, err := c.client.UpdateEntry(ctx, &sekevev1.UpdateEntryRequest{Entry: envelopeToProto(envelope)})
	if err != nil {
		return log.WrapErr(err, "failed to update entry")
	}
	return nil
}

func (c *Client) GetEntry(ctx context.Context, name string) (*entity.Envelope, error) {
	log := zerowrap.FromCtx(ctx)
	ctx = c.authedCtx(ctx)
	entry, err := c.client.GetEntry(ctx, &sekevev1.GetEntryRequest{Name: name})
	if err != nil {
		return nil, log.WrapErr(err, "failed to get entry")
	}
	return protoToEnvelope(entry), nil
}

func (c *Client) ListEntries(ctx context.Context, entryType entity.EntryType) ([]*entity.Envelope, error) {
	log := zerowrap.FromCtx(ctx)
	ctx = c.authedCtx(ctx)
	resp, err := c.client.ListEntries(ctx, &sekevev1.ListEntriesRequest{Type: sekevev1.EntryType(entryType)})
	if err != nil {
		return nil, log.WrapErr(err, "failed to list entries")
	}
	var envelopes []*entity.Envelope
	for _, entry := range resp.Entries {
		envelopes = append(envelopes, protoToEnvelope(entry))
	}
	return envelopes, nil
}

func (c *Client) DeleteEntry(ctx context.Context, name string) error {
	log := zerowrap.FromCtx(ctx)
	ctx = c.authedCtx(ctx)
	_, err := c.client.DeleteEntry(ctx, &sekevev1.DeleteEntryRequest{Name: name})
	if err != nil {
		return log.WrapErr(err, "failed to delete entry")
	}
	return nil
}

func (c *Client) Close(ctx context.Context) error {
	log := zerowrap.FromCtx(ctx)
	if err := c.conn.Close(); err != nil {
		return log.WrapErr(err, "failed to close gRPC connection")
	}
	return nil
}

// CheckHealth calls the gRPC health check service to verify the server is reachable.
func (c *Client) CheckHealth(ctx context.Context) error {
	healthClient := grpc_health_v1.NewHealthClient(c.conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("server health check failed: %w", err)
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("server not serving (status: %s)", resp.Status)
	}
	return nil
}
