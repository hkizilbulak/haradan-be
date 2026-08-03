package password_test

import (
	"strings"
	"testing"

	"github.com/hkizilbulak/haradan-be/internal/platform/security/password"
)

func TestHashVerify(t *testing.T) {
	h, err := password.NewHasher(password.TestParams())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := h.Hash("SecretPass1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "SecretPass1") {
		t.Fatal("plaintext found in hash")
	}
	ok, err := h.Verify(encoded, "SecretPass1")
	if err != nil || !ok {
		t.Fatalf("verify ok=%v err=%v", ok, err)
	}
	ok, err = h.Verify(encoded, "wrong-password")
	if err != nil || ok {
		t.Fatalf("verify ok=%v err=%v", ok, err)
	}
}
