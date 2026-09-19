package live

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrBadToken covers forged, malformed, and expired playback tokens alike; the
// caller has no use for the distinction and reporting it leaks information.
var ErrBadToken = errors.New("invalid or expired playback token")

// TokenTTL bounds how long a minted playback token stays usable. Long enough that a
// viewer never gets interrupted mid-session by a token expiring between segment
// requests, short enough that a leaked manifest stops working the same day.
const TokenTTL = 6 * time.Hour

// TokenSigner mints and verifies short-lived playback tokens.
//
// Only the master playlist request is authenticated with a real session. Every URL
// the proxy writes into a manifest carries one of these instead, which keeps the
// caller's session ID out of manifest bodies and scopes the credential to a single
// channel: a token that leaks grants this one stream until it expires, and nothing
// else on the API.
type TokenSigner struct {
	key []byte
}

// NewTokenSigner derives a signing key from the application session secret.
func NewTokenSigner(secret string) *TokenSigner {
	key := sha256.Sum256([]byte("inox-live-token-v1|" + secret))
	return &TokenSigner{key: key[:]}
}

// Mint issues a token binding one user to one channel until expiry.
func (t *TokenSigner) Mint(channelID, userID string, now time.Time) string {
	body := fmt.Sprintf("%d|%s|%s", now.Add(TokenTTL).Unix(), channelID, userID)
	return base64.RawURLEncoding.EncodeToString([]byte(body)) + "." + t.sign(body)
}

// Verify checks a token against a channel and returns the user it was minted for.
func (t *TokenSigner) Verify(channelID, token string, now time.Time) (string, error) {
	encoded, mac, ok := strings.Cut(token, ".")
	if !ok {
		return "", ErrBadToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrBadToken
	}
	body := string(raw)

	// Constant-time comparison: a byte-wise early return here would leak the
	// expected signature one byte at a time to anyone willing to time the endpoint.
	if !hmac.Equal([]byte(t.sign(body)), []byte(mac)) {
		return "", ErrBadToken
	}

	parts := strings.Split(body, "|")
	if len(parts) != 3 {
		return "", ErrBadToken
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || now.Unix() > exp {
		return "", ErrBadToken
	}
	if parts[1] != channelID {
		return "", ErrBadToken
	}
	return parts[2], nil
}

func (t *TokenSigner) sign(body string) string {
	m := hmac.New(sha256.New, t.key)
	m.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
