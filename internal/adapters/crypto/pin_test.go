package crypto

import (
	"testing"
)

func TestHashPIN_ProducesDifferentHashesForSamePIN(t *testing.T) {
	hash1, salt1, err := HashPIN("1234")
	if err != nil {
		t.Fatal(err)
	}
	hash2, salt2, err := HashPIN("1234")
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

func TestVerifyPIN_CorrectPIN(t *testing.T) {
	hash, salt, err := HashPIN("5678")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPIN("5678", hash, salt) {
		t.Error("expected correct PIN to verify")
	}
}

func TestVerifyPIN_WrongPIN(t *testing.T) {
	hash, salt, err := HashPIN("5678")
	if err != nil {
		t.Fatal(err)
	}
	if VerifyPIN("0000", hash, salt) {
		t.Error("expected wrong PIN to fail")
	}
}

func TestHashPIN_EmptyPIN(t *testing.T) {
	_, _, err := HashPIN("")
	if err == nil {
		t.Error("expected error for empty PIN")
	}
}
