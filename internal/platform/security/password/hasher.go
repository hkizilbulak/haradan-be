package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonAlgorithm = "argon2id"
	saltSize       = 16
)

// Params controls Argon2id cost. Tests inject low values.
type Params struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
}

// DefaultParams returns production-oriented Argon2id parameters.
// Exact product costs are not locked in docs; these are engineering defaults.
func DefaultParams() Params {
	return Params{
		Time:    3,
		Memory:  64 * 1024,
		Threads: 2,
		KeyLen:  32,
	}
}

// TestParams returns fast parameters for unit tests.
func TestParams() Params {
	return Params{
		Time:    1,
		Memory:  8 * 1024,
		Threads: 1,
		KeyLen:  32,
	}
}

// Hasher hashes and verifies passwords with Argon2id.
type Hasher struct {
	params Params
}

// NewHasher constructs a Hasher.
func NewHasher(params Params) (*Hasher, error) {
	if params.Time == 0 || params.Memory == 0 || params.Threads == 0 || params.KeyLen == 0 {
		return nil, fmt.Errorf("password hasher params must be positive")
	}
	return &Hasher{params: params}, nil
}

// Hash returns an encoded Argon2id password hash. Plaintext is never retained.
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)
	return encodeHash(h.params, salt, hash), nil
}

// Verify compares password against an encoded hash using constant-time compare.
func (h *Hasher) Verify(encodedHash, password string) (bool, error) {
	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}
	other := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, params.KeyLen)
	if subtle.ConstantTimeCompare(hash, other) == 1 {
		return true, nil
	}
	return false, nil
}

func encodeHash(p Params, salt, hash []byte) string {
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$%s$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonAlgorithm, p.Memory, p.Time, p.Threads, b64Salt, b64Hash)
}

func decodeHash(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != argonAlgorithm {
		return Params{}, nil, nil, errors.New("invalid password hash encoding")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != 19 {
		return Params{}, nil, nil, errors.New("unsupported argon2 version")
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, errors.New("invalid argon2 params")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, errors.New("invalid salt encoding")
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, errors.New("invalid hash encoding")
	}
	p.KeyLen = uint32(len(hash))
	if p.KeyLen == 0 {
		return Params{}, nil, nil, errors.New("empty password hash")
	}
	return p, salt, hash, nil
}

// DummyHash is a valid encoded hash used to equalize timing when a user is missing.
func DummyHash(h *Hasher) string {
	encoded, err := h.Hash("timing-equalization-dummy-password")
	if err != nil {
		// Extremely unlikely; return a fixed shape that Verify will reject safely.
		return "$argon2id$v=19$m=8192,t=1,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	return encoded
}
