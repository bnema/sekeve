// Package grpc provides a gRPC server adapter implementing the Sekeve service.
package grpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"time"
	"unicode"

	sekevev1 "github.com/bnema/sekeve/gen/proto/sekeve/v1"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/pinhash"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// Server implements sekevev1.SekeveServer.
type Server struct {
	sekevev1.UnimplementedSekeveServer
	grpcServer *grpc.Server
	storage    port.StoragePort
	auth       *AuthManager
	gpg        port.ServerCryptoPort
}

// NewServer creates a new Server with the auth interceptor registered.
func NewServer(ctx context.Context, storage port.StoragePort, auth *AuthManager, crypto port.ServerCryptoPort) *Server {
	log := zerowrap.FromCtx(ctx)
	log.Info().Msg("creating gRPC server")

	s := &Server{
		storage: storage,
		auth:    auth,
		gpg:     crypto,
	}

	// Check if PIN is already configured so auth interceptor knows about it.
	if _, _, err := storage.GetPINHash(ctx); err == nil {
		auth.SetPINConfigured(true)
	}

	// Extract fingerprint from stored public key for authentication validation.
	fp, err := crypto.FingerprintFromArmored(ctx, auth.GPGPublicKey())
	if err != nil {
		log.Warn().Err(err).Msg("could not extract fingerprint from stored key")
	} else {
		auth.SetGPGFingerprint(fp)
		log.Info().Str("fingerprint", fp).Msg("registered GPG key fingerprint")
	}

	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(auth.UnaryInterceptor()),
		grpc.MaxRecvMsgSize(1<<20), // 1 MB
		grpc.MaxSendMsgSize(4<<20), // 4 MB (list responses can be larger)
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			MaxConnectionAge:  30 * time.Minute,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	sekevev1.RegisterSekeveServer(s.grpcServer, s)

	// Register gRPC health service.
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s.grpcServer, healthSrv)

	auth.StartSweeper(ctx)

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

// validatePIN checks that a PIN is 4-6 ASCII digits.
func validatePIN(pin string) error {
	if len(pin) < 4 || len(pin) > 6 {
		return fmt.Errorf("PIN must be 4-6 digits")
	}
	for _, r := range pin {
		if r < '0' || r > '9' {
			return fmt.Errorf("PIN must contain only digits")
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

	// Verify the requested key ID resolves to the registered fingerprint.
	expectedFP := s.auth.GPGFingerprint()
	if expectedFP == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "server GPG key not configured")
	}
	if err := s.gpg.VerifyKeyIDMatchesFingerprint(ctx, req.GpgKeyId, expectedFP, pubKey); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	nonce, err := s.auth.GenerateChallenge(ctx)
	if err != nil {
		return nil, err
	}

	challenge := s.auth.FormatChallenge(nonce)

	ciphertext, err := s.gpg.Encrypt(ctx, []byte(challenge), req.GpgKeyId)
	if err != nil {
		return nil, err
	}

	return &sekevev1.AuthChallenge{
		EncryptedChallenge: ciphertext,
	}, nil
}

// VerifyChallenge verifies the nonce returned by the client and issues a session token.
// When PIN is configured, it returns an unlock ticket instead of a session token.
func (s *Server) VerifyChallenge(ctx context.Context, req *sekevev1.ChallengeResponse) (*sekevev1.SessionToken, error) {
	result, err := s.auth.VerifyNonce(ctx, req.Nonce)
	if err != nil {
		return nil, err
	}

	return &sekevev1.SessionToken{
		Token:        result.Token,
		ExpiresAt:    result.ExpiresAt.Unix(),
		RequiresPin:  result.RequiresPIN,
		UnlockTicket: result.UnlockTicket,
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

// HasPIN reports whether a PIN has been configured on the server.
// This RPC is unauthenticated (listed in skipAuthMethods).
func (s *Server) HasPIN(ctx context.Context, _ *sekevev1.HasPINRequest) (*sekevev1.HasPINResponse, error) {
	_, _, err := s.storage.GetPINHash(ctx)
	return &sekevev1.HasPINResponse{HasPin: err == nil}, nil
}

// SetPIN stores (or changes) the server PIN. Requires a valid session token.
// When a PIN already exists the request must include the correct current PIN.
func (s *Server) SetPIN(ctx context.Context, req *sekevev1.SetPINRequest) (*sekevev1.SetPINResponse, error) {
	if err := validatePIN(req.NewPin); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	existingHash, existingSalt, err := s.storage.GetPINHash(ctx)
	if err == nil {
		// A PIN is already set — caller must supply the correct current PIN.
		if err := s.auth.CheckPINRateLimit(); err != nil {
			return nil, status.Errorf(codes.ResourceExhausted, "%v", err)
		}
		if !pinhash.Verify(req.CurrentPin, existingHash, existingSalt) {
			s.auth.RecordPINFailure()
			return nil, status.Errorf(codes.PermissionDenied, "incorrect current PIN")
		}
		s.auth.ResetPINFailures()
	}

	hash, salt, err := pinhash.Hash(req.NewPin)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to hash PIN: %v", err)
	}
	if err := s.storage.StorePINHash(ctx, hash, salt); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to store PIN: %v", err)
	}

	s.auth.SetPINConfigured(true)
	s.auth.InvalidateAllSessions()
	return &sekevev1.SetPINResponse{}, nil
}

// Unlock exchanges a one-time unlock ticket and PIN for a session token.
// The ticket is consumed on first use regardless of PIN correctness.
// This RPC is unauthenticated (listed in skipAuthMethods).
func (s *Server) Unlock(ctx context.Context, req *sekevev1.UnlockRequest) (*sekevev1.UnlockResponse, error) {
	if err := s.auth.CheckPINRateLimit(); err != nil {
		return nil, status.Errorf(codes.ResourceExhausted, "%v", err)
	}

	// Consume the ticket first — prevents unlimited PIN guessing per ticket.
	token, expiresAt, err := s.auth.RedeemUnlockTicket(ctx, req.UnlockTicket)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid unlock ticket: %v", err)
	}

	// Verify PIN after ticket consumption.
	hash, salt, pinErr := s.storage.GetPINHash(ctx)
	if pinErr != nil {
		// No PIN configured but ticket was issued — revoke the session.
		s.auth.RevokeSession(token)
		return nil, status.Errorf(codes.FailedPrecondition, "no PIN configured")
	}
	if !pinhash.Verify(req.Pin, hash, salt) {
		s.auth.RecordPINFailure()
		// Revoke the just-issued session since PIN was wrong.
		s.auth.RevokeSession(token)
		return nil, status.Errorf(codes.PermissionDenied, "incorrect PIN")
	}

	s.auth.ResetPINFailures()
	return &sekevev1.UnlockResponse{
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
	}, nil
}
