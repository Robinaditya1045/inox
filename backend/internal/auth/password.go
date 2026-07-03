package auth

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
	saltSize    = 16
	keySize     = 32
	timeCost    = 1
	memoryCost  = 64 * 1024 // 64 MB
	threads     = 4
)

// HashPassword hashes a plain text password using Argon2id.
// It returns a formatted string containing the algorithm parameters, salt, and hash:
// $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, timeCost, memoryCost, threads, keySize)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryCost, timeCost, threads, b64Salt, b64Hash)

	return encodedHash, nil
}

// VerifyPassword compares a plaintext password against an Argon2id formatted hash string.
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, errors.New("invalid password hash format")
	}

	if parts[1] != "argon2id" {
		return false, errors.New("unsupported password hash algorithm")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, errors.New("invalid hash version")
	}
	if version != argon2.Version {
		return false, errors.New("incompatible Argon2 version")
	}

	var memory uint32
	var time uint32
	var parallel uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &parallel); err != nil {
		return false, errors.New("invalid hash parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	calculatedHash := argon2.IDKey([]byte(password), salt, time, memory, parallel, uint32(len(expectedHash)))

	// Use constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare(expectedHash, calculatedHash) == 1 {
		return true, nil
	}
	return false, nil
}
