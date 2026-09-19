package live

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrBadRef is returned when a sealed upstream reference fails to open, which means
// it was forged, truncated, or sealed under a different key.
var ErrBadRef = errors.New("invalid upstream reference")

// Sealer encrypts upstream URLs into opaque references that can be embedded in a
// manifest served to clients.
//
// Encrypted rather than merely signed, and for two separate reasons. Signing alone
// would stop the proxy being used as an open relay, but the upstream URL is itself a
// credential: it is signed by the provider, often carries a session token in its
// query string, and anyone holding it can pull the feed directly and bypass us
// entirely. Sealing keeps it inside the backend while still leaving the proxy
// stateless, so a segment request can be served by any process without a shared
// lookup table.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer derives an AES-GCM key from the application session secret. The domain
// separation string keeps this key distinct from any other use of that secret.
func NewSealer(secret string) (*Sealer, error) {
	key := sha256.Sum256([]byte("inox-live-ref-v1|" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to initialize live ref cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize live ref AEAD: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

// Seal encrypts an absolute upstream URL, scoped to one channel. Passing the channel
// ID as additional authenticated data means a reference minted for one channel cannot
// be replayed against another.
func (s *Sealer) Seal(channelID, rawURL string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	sealed := s.aead.Seal(nonce, nonce, []byte(rawURL), []byte(channelID))
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Open reverses Seal, returning ErrBadRef for anything this server did not mint for
// this channel.
func (s *Sealer) Open(channelID, ref string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(ref)
	if err != nil {
		return "", ErrBadRef
	}
	ns := s.aead.NonceSize()
	if len(raw) < ns {
		return "", ErrBadRef
	}
	plain, err := s.aead.Open(nil, raw[:ns], raw[ns:], []byte(channelID))
	if err != nil {
		return "", ErrBadRef
	}
	return string(plain), nil
}
