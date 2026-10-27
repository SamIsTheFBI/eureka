package crypto

import (
	"testing"
)

func TestEncryptDecryptCredential(t *testing.T) {
	masterKey := make([]byte, 32)
	copy(masterKey, []byte("01234567890123456789012345678901"))

	plaintext := "sk-super-secret-credential-value-123"
	aad := "woo"

	enc, err := EncryptCredential(masterKey, plaintext, aad)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	dec, err := DecryptCredential(masterKey, enc, aad)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if dec != plaintext {
		t.Fatalf("expected decrypted text %q, got %q", plaintext, dec)
	}

	// Verify tampering with AAD fails (PRD section 10: credential cannot be moved to another row)
	_, err = DecryptCredential(masterKey, enc, "different-slug")
	if err == nil {
		t.Fatal("expected decryption with mismatched AAD to fail, but it succeeded")
	}
}

func TestPasswordHashing(t *testing.T) {
	pass := "SecurePassword123!"
	hash, err := HashPassword(pass)
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}

	if !CheckPassword(pass, hash) {
		t.Fatal("password verification failed for matching password")
	}

	if CheckPassword("WrongPassword", hash) {
		t.Fatal("password verification succeeded for wrong password")
	}
}

func TestAPIKeyGeneration(t *testing.T) {
	fullKey, prefix, hash, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("key generation failed: %v", err)
	}

	if len(prefix) != 8 {
		t.Fatalf("expected 8 char prefix, got %d (%s)", len(prefix), prefix)
	}

	computedHash := HashKey(fullKey)
	if computedHash != hash {
		t.Fatalf("expected hash %s, got %s", hash, computedHash)
	}
}
