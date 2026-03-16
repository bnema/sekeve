// Package grpc provides a gRPC server adapter implementing the Sekeve service.
package grpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"unicode"

	sekevev1 "github.com/bnema/sekeve/gen/proto/sekeve/v1"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// Server implements sekevev1.SekeveServer.
type Server struct {
	sekevev1.UnimplementedSekeveServer
	grpcServer *grpc.Server
	storage    port.StoragePort
	auth       *AuthManager
}

// NewServer creates a new Server with the auth interceptor registered.
func NewServer(ctx context.Context, storage port.StoragePort, auth *AuthManager) *Server {
	log := zerowrap.FromCtx(ctx)
	log.Info().Msg("creating gRPC server")

	s := &Server{
		storage: storage,
		auth:    auth,
	}

	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(auth.UnaryInterceptor()),
	)
	sekevev1.RegisterSekeveServer(s.grpcServer, s)

	// Register gRPC health service.
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s.grpcServer, healthSrv)

	return s
}

// Serve listens on the given TCP address and blocks until the context is cancelled,
// at which point it performs a graceful stop.
func (s *Server) Serve(ctx context.Context, addr string) error {
	log := zerowrap.FromCtx(ctx)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return log.WrapErrf(err, "failed to listen on %s", addr)
	}

	return s.ServeListener(ctx, lis)
}

// ServeListener serves on an existing net.Listener. Useful for bufconn tests.
func (s *Server) ServeListener(ctx context.Context, lis net.Listener) error {
	log := zerowrap.FromCtx(ctx)

	errCh := make(chan error, 1)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		s.grpcServer.GracefulStop()
		return nil
	case err := <-errCh:
		if err != nil {
			return log.WrapErr(err, "grpc server error")
		}
		return nil
	}
}

// validateKeyID checks that a GPG key ID contains only safe characters.
// Accepts fingerprints and email addresses. Does not accept full user IDs with spaces.
func validateKeyID(keyID string) error {
	if keyID == "" {
		return fmt.Errorf("GPG key ID cannot be empty")
	}
	for _, r := range keyID {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '@' && r != '.' && r != '-' && r != '_' {
			return fmt.Errorf("invalid character in GPG key ID: %c", r)
		}
	}
	return nil
}

// Authenticate imports the stored GPG public key, generates a challenge nonce,
// encrypts the challenge with the client's key and returns the ciphertext.
func (s *Server) Authenticate(ctx context.Context, req *sekevev1.AuthRequest) (*sekevev1.AuthChallenge, error) {
	log := zerowrap.FromCtx(ctx)

	if err := validateKeyID(req.GpgKeyId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid GPG key ID: %v", err)
	}

	pubKey := s.auth.GPGPublicKey()

	// Import the GPG public key.
	importCmd := exec.CommandContext(ctx, "gpg", "--batch", "--import")
	importCmd.Stdin = bytes.NewReader(pubKey)
	var importStderr bytes.Buffer
	importCmd.Stderr = &importStderr
	if err := importCmd.Run(); err != nil {
		return nil, log.WrapErrf(err, "gpg import failed: %s", importStderr.String())
	}

	nonce, err := s.auth.GenerateChallenge(ctx)
	if err != nil {
		return nil, err
	}

	challenge := s.auth.FormatChallenge(nonce)

	// Encrypt the challenge string with the recipient key.
	encCmd := exec.CommandContext(ctx, "gpg",
		"--batch", "--yes",
		"--trust-model", "always",
		"--recipient", req.GpgKeyId,
		"--encrypt",
	)
	encCmd.Stdin = bytes.NewBufferString(challenge)
	var stdout, stderr bytes.Buffer
	encCmd.Stdout = &stdout
	encCmd.Stderr = &stderr
	if err := encCmd.Run(); err != nil {
		return nil, log.WrapErrf(err, "gpg encrypt failed: %s", stderr.String())
	}

	return &sekevev1.AuthChallenge{
		EncryptedChallenge: stdout.Bytes(),
	}, nil
}

// VerifyChallenge verifies the nonce returned by the client and issues a session token.
func (s *Server) VerifyChallenge(ctx context.Context, req *sekevev1.ChallengeResponse) (*sekevev1.SessionToken, error) {
	token, expiry, err := s.auth.VerifyNonce(ctx, req.Nonce)
	if err != nil {
		return nil, err
	}

	return &sekevev1.SessionToken{
		Token:     token,
		ExpiresAt: expiry.Unix(),
	}, nil
}

// CreateEntry stores a new entry.
func (s *Server) CreateEntry(ctx context.Context, req *sekevev1.CreateEntryRequest) (*sekevev1.CreateEntryResponse, error) {
	log := zerowrap.FromCtx(ctx)

	if req.Entry == nil {
		return nil, status.Error(codes.InvalidArgument, "entry must not be nil")
	}

	env := protoToEnvelope(req.Entry)
	if err := s.storage.Create(ctx, env); err != nil {
		return nil, log.WrapErr(err, "failed to create entry")
	}

	return &sekevev1.CreateEntryResponse{Id: env.ID}, nil
}

// UpdateEntry updates an existing entry.
func (s *Server) UpdateEntry(ctx context.Context, req *sekevev1.UpdateEntryRequest) (*sekevev1.UpdateEntryResponse, error) {
	log := zerowrap.FromCtx(ctx)

	if req.Entry == nil {
		return nil, status.Error(codes.InvalidArgument, "entry must not be nil")
	}

	env := protoToEnvelope(req.Entry)
	if err := s.storage.Update(ctx, env); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "entry not found")
		}
		return nil, log.WrapErr(err, "failed to update entry")
	}

	return &sekevev1.UpdateEntryResponse{}, nil
}

// GetEntry retrieves an entry by ID.
func (s *Server) GetEntry(ctx context.Context, req *sekevev1.GetEntryRequest) (*sekevev1.Entry, error) {
	log := zerowrap.FromCtx(ctx)

	env, err := s.storage.GetByID(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "entry not found")
		}
		return nil, log.WrapErr(err, "failed to get entry")
	}

	return envelopeToProto(env), nil
}

// ListEntries lists entries, optionally filtered by type.
func (s *Server) ListEntries(ctx context.Context, req *sekevev1.ListEntriesRequest) (*sekevev1.ListEntriesResponse, error) {
	log := zerowrap.FromCtx(ctx)

	entryType := entity.EntryType(req.Type)
	envs, err := s.storage.List(ctx, entryType)
	if err != nil {
		return nil, log.WrapErr(err, "failed to list entries")
	}

	entries := make([]*sekevev1.Entry, 0, len(envs))
	for _, env := range envs {
		entries = append(entries, envelopeToProto(env))
	}

	return &sekevev1.ListEntriesResponse{Entries: entries}, nil
}

// DeleteEntry removes an entry by ID.
func (s *Server) DeleteEntry(ctx context.Context, req *sekevev1.DeleteEntryRequest) (*sekevev1.DeleteEntryResponse, error) {
	log := zerowrap.FromCtx(ctx)

	if err := s.storage.DeleteByID(ctx, req.GetId()); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "entry not found")
		}
		return nil, log.WrapErr(err, "failed to delete entry")
	}

	return &sekevev1.DeleteEntryResponse{}, nil
}
