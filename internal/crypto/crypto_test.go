package crypto

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	var key [32]byte
	copy(key[:], "test-key-32-bytes-long-padding!!")

	plaintext := []byte("hello, invito!")
	enc, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	dec, err := Decrypt(key, enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, dec) {
		t.Fatalf("got %q, want %q", dec, plaintext)
	}
}

func TestTamperedCiphertext(t *testing.T) {
	var key [32]byte
	copy(key[:], "test-key-32-bytes-long-padding!!")

	enc, err := Encrypt(key, []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	enc[len(enc)-1] ^= 0xff

	if _, err := Decrypt(key, enc); err == nil {
		t.Fatal("expected error on tampered ciphertext")
	}
}
