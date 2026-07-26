package crypto

import (
	"strings"
	"testing"
)

func TestBoxRoundTrip(t *testing.T) {
	box, err := NewBox("test-secret-material-32b!!")
	if err != nil {
		t.Fatal(err)
	}
	const plain = "auth=super-secret-cookie-value"
	enc, err := box.Seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if enc == plain {
		t.Fatal("ciphertext equals plaintext")
	}
	if !strings.HasPrefix(enc, "v1.") {
		t.Fatalf("ciphertext = %q, want versioned v1 payload", enc)
	}
	got, err := box.Open(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("got %q want %q", got, plain)
	}
}

func TestBoxSeal_uses_distinct_nonces_for_same_plaintext(t *testing.T) {
	// Given
	box, err := NewBox("test-secret-material-32b!!")
	if err != nil {
		t.Fatal(err)
	}

	// When
	first, err := box.Seal("same plaintext")
	if err != nil {
		t.Fatal(err)
	}
	second, err := box.Seal("same plaintext")
	if err != nil {
		t.Fatal(err)
	}

	// Then
	if first == second {
		t.Fatal("Seal() returned identical ciphertext for repeated plaintext")
	}
	for _, payload := range []string{first, second} {
		plain, openErr := box.Open(payload)
		if openErr != nil {
			t.Fatalf("Open() error = %v", openErr)
		}
		if plain != "same plaintext" {
			t.Fatalf("Open() = %q", plain)
		}
	}
}

func TestBoxOpen_rejects_tampered_ciphertext(t *testing.T) {
	// Given
	box, err := NewBox("test-secret-material-32b!!")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := box.Seal("secret")
	if err != nil {
		t.Fatal(err)
	}
	tampered := payload[:len(payload)-1] + "A"

	// When
	_, err = box.Open(tampered)

	// Then
	if err == nil {
		t.Fatal("Open() error = nil, want authenticated decryption failure")
	}
}

func TestBoxOpen_rejects_wrong_key(t *testing.T) {
	// Given
	sealer, err := NewBox("test-secret-material-32b!!")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := NewBox("different-secret-material-32b")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := sealer.Seal("secret")
	if err != nil {
		t.Fatal(err)
	}

	// When
	_, err = reader.Open(payload)

	// Then
	if err == nil {
		t.Fatal("Open() error = nil, want wrong-key failure")
	}
}

func TestMaskKey(t *testing.T) {
	m := MaskKey("oc-abcdefghijklmnop")
	if m == "oc-abcdefghijklmnop" {
		t.Fatal("key not masked")
	}
}
