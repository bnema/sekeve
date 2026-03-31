package pinhash

import (
	"testing"
)

func TestHash_ProducesDifferentHashesForSamePIN(t *testing.T) {
	hash1, salt1, err := Hash("1234")
	if err != nil {
		t.Fatal(err)
	}
	hash2, salt2, err := Hash("1234")
	if err != nil {
		t.Fatal(err)
	}
	if string(salt1) == string(salt2) {
		t.Error("expected different salts")
	}
	if string(hash1) == string(hash2) {
		t.Error("expected different hashes due to different salts")
	}
}

func TestVerify_CorrectPIN(t *testing.T) {
	hash, salt, err := Hash("5678")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify("5678", hash, salt) {
		t.Error("expected correct PIN to verify")
	}
}

func TestVerify_WrongPIN(t *testing.T) {
	hash, salt, err := Hash("5678")
	if err != nil {
		t.Fatal(err)
	}
	if Verify("0000", hash, salt) {
		t.Error("expected wrong PIN to fail")
	}
}

func TestHash_EmptyPIN(t *testing.T) {
	_, _, err := Hash("")
	if err == nil {
		t.Error("expected error for empty PIN")
	}
}
