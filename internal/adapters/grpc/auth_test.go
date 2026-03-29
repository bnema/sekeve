package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVerifyNonce_WithPIN_ReturnsUnlockTicket(t *testing.T) {
	am := NewAuthManager(nil)
	am.SetPINConfigured(true)

	nonce, err := am.GenerateChallenge(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	result, err := am.VerifyNonce(context.Background(), nonce)
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "" {
		t.Error("expected empty token when PIN is configured")
	}
	if result.UnlockTicket == "" {
		t.Error("expected unlock ticket when PIN is configured")
	}
	if !result.RequiresPIN {
		t.Error("expected RequiresPIN = true")
	}
}

func TestVerifyNonce_WithoutPIN_ReturnsToken(t *testing.T) {
	am := NewAuthManager(nil)
	am.SetPINConfigured(false)

	nonce, err := am.GenerateChallenge(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	result, err := am.VerifyNonce(context.Background(), nonce)
	if err != nil {
		t.Fatal(err)
	}
	if result.Token == "" {
		t.Error("expected token when no PIN")
	}
	if result.RequiresPIN {
		t.Error("expected RequiresPIN = false")
	}
}

func TestRedeemUnlockTicket_Valid(t *testing.T) {
	am := NewAuthManager(nil)
	am.SetPINConfigured(true)

	nonce, _ := am.GenerateChallenge(context.Background())
	result, _ := am.VerifyNonce(context.Background(), nonce)

	token, expiresAt, err := am.RedeemUnlockTicket(context.Background(), result.UnlockTicket)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Error("expected token after redeeming ticket")
	}
	if expiresAt.IsZero() {
		t.Error("expected non-zero expiry")
	}
}

func TestRedeemUnlockTicket_OneUse(t *testing.T) {
	am := NewAuthManager(nil)
	am.SetPINConfigured(true)

	nonce, _ := am.GenerateChallenge(context.Background())
	result, _ := am.VerifyNonce(context.Background(), nonce)

	_, _, err := am.RedeemUnlockTicket(context.Background(), result.UnlockTicket)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = am.RedeemUnlockTicket(context.Background(), result.UnlockTicket)
	if err == nil {
		t.Error("expected error on second redemption")
	}
}

func TestRedeemUnlockTicket_Invalid(t *testing.T) {
	am := NewAuthManager(nil)
	_, _, err := am.RedeemUnlockTicket(context.Background(), "bogus")
	if err == nil {
		t.Error("expected error for invalid ticket")
	}
}

func TestPINRateLimit_BlocksAfterFailures(t *testing.T) {
	am := NewAuthManager(nil)

	// First check should pass.
	if err := am.CheckPINRateLimit(); err != nil {
		t.Fatal("expected no rate limit initially")
	}

	// Record failures — each should set a lockout.
	am.RecordPINFailure()
	if err := am.CheckPINRateLimit(); err == nil {
		t.Error("expected rate limit after first failure")
	}
}

func TestPINRateLimit_ResetsOnSuccess(t *testing.T) {
	am := NewAuthManager(nil)

	am.RecordPINFailure()
	am.RecordPINFailure()
	am.ResetPINFailures()

	if err := am.CheckPINRateLimit(); err != nil {
		t.Error("expected no rate limit after reset")
	}
}

func TestPINRateLimit_ExponentialBackoff(t *testing.T) {
	am := NewAuthManager(nil)

	// Each failure should increase the lockout duration.
	am.RecordPINFailure() // 2s
	am.mu.Lock()
	d1 := am.pinLockedUntil
	am.mu.Unlock()

	am.RecordPINFailure() // 4s
	am.mu.Lock()
	d2 := am.pinLockedUntil
	am.mu.Unlock()

	if !d2.After(d1) {
		t.Error("expected increasing lockout duration")
	}
}

func TestSessionCap_RejectsWhenFull(t *testing.T) {
	am := NewAuthManager(nil)
	am.SetPINConfigured(false)

	// Fill to capacity.
	for range maxSessions {
		nonce, err := am.GenerateChallenge(context.Background())
		require.NoError(t, err)
		result, err := am.VerifyNonce(context.Background(), nonce)
		require.NoError(t, err)
		require.NotEmpty(t, result.Token)
	}

	// Next session should fail.
	nonce, err := am.GenerateChallenge(context.Background())
	require.NoError(t, err)
	_, err = am.VerifyNonce(context.Background(), nonce)
	require.Error(t, err, "should reject when at session cap")
}

func TestInvalidateAllSessions(t *testing.T) {
	am := NewAuthManager(nil)
	am.SetPINConfigured(false)

	// Create a session.
	nonce, _ := am.GenerateChallenge(context.Background())
	result, _ := am.VerifyNonce(context.Background(), nonce)
	require.True(t, am.validateToken(result.Token))

	// Invalidate.
	am.InvalidateAllSessions()

	require.False(t, am.validateToken(result.Token))
}

func TestSweepExpiredSessions(t *testing.T) {
	am := NewAuthManager(nil)

	// Manually insert an expired session.
	am.mu.Lock()
	am.sessions["expired-token"] = sessionEntry{expiresAt: time.Now().Add(-time.Hour)}
	am.sessions["valid-token"] = sessionEntry{expiresAt: time.Now().Add(time.Hour)}
	am.mu.Unlock()

	am.SweepExpired()

	require.False(t, am.validateToken("expired-token"))
	require.True(t, am.validateToken("valid-token"))
}
