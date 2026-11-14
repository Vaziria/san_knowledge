package san_exness

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SignVersion is the only signature protocol version Exness accepts.
const SignVersion = 1

var (
	// ErrInvalidKey: the private key text is neither 128 hex characters
	// (Exness-generated: the 32-byte seed, then the 32-byte public key)
	// nor an unencrypted PKCS#8 PEM Ed25519 key.
	ErrInvalidKey = errors.New("exness: private key must be 128 hex characters or an unencrypted PKCS#8 PEM Ed25519 key")
	// ErrInvalidIdempotencyKey: a mutating request needs a key of 5 to 128
	// characters from A-Z a-z 0-9 . _ - ~; a signed GET an empty one.
	ErrInvalidIdempotencyKey = errors.New("exness: invalid idempotency key")
)

var (
	hexKey        = regexp.MustCompile(`^[0-9a-fA-F]{128}$`)
	idempotentKey = regexp.MustCompile(`^[A-Za-z0-9._~-]{5,128}$`)
)

// ParsePrivateKey reads an Ed25519 signing key in either format Key
// Management hands out. Its errors never contain the key text.
func ParsePrivateKey(text string) (ed25519.PrivateKey, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "-----BEGIN") {
		block, _ := pem.Decode([]byte(text))
		if block == nil || block.Type != "PRIVATE KEY" {
			return nil, ErrInvalidKey
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, ErrInvalidKey
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok {
			return nil, ErrInvalidKey
		}
		return key, nil
	}
	if !hexKey.MatchString(text) {
		return nil, ErrInvalidKey
	}
	raw, _ := hex.DecodeString(text)
	key := ed25519.NewKeyFromSeed(raw[:ed25519.SeedSize])
	// The second half must be the seed's own public key.
	if subtle.ConstantTimeCompare(key[ed25519.SeedSize:], raw[ed25519.SeedSize:]) != 1 {
		return nil, fmt.Errorf("%w: its public half does not match its seed", ErrInvalidKey)
	}
	return key, nil
}

// Signer signs requests with an API key's Ed25519 private key.
type Signer struct {
	APIKey string // EXN-API-KEY, "exnsk-..."
	Key    ed25519.PrivateKey
}

// Headers are the EXN-* headers of one signed request.
type Headers struct {
	APIKey         string // EXN-API-KEY
	IdempotencyKey string // EXN-IDEMPOTENCY-KEY (sent even when empty)
	Timestamp      string // EXN-TIMESTAMP, Unix milliseconds
	SignVersion    string // EXN-SIGN-VERSION
	Data           string // EXN-DATA, base64url of the signed JSON
	Sign           string // EXN-SIGN, base64url Ed25519 signature of that JSON
}

// Map returns the headers by their exact names. Set them on a request with
// req.Header[name] = []string{value}: Header.Set would rewrite the names.
func (h Headers) Map() map[string]string {
	return map[string]string{
		"EXN-API-KEY":         h.APIKey,
		"EXN-IDEMPOTENCY-KEY": h.IdempotencyKey,
		"EXN-TIMESTAMP":       h.Timestamp,
		"EXN-SIGN-VERSION":    h.SignVersion,
		"EXN-DATA":            h.Data,
		"EXN-SIGN":            h.Sign,
	}
}

// signedData is the JSON inside EXN-DATA, in the documented field order.
type signedData struct {
	APIKey         string `json:"api_key"`
	IdempotencyKey string `json:"idempotency_key"`
	Timestamp      int64  `json:"timestamp"`
	SignVersion    int    `json:"sign_version"`
	Method         string `json:"method"`
	Path           string `json:"path"`
	BodyHash       string `json:"body_hash"`
}

// bodyHash is SHA-256 of the exact body bytes, base64url without padding;
// no body hashes the empty byte sequence.
func bodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Sign signs one request. pathWithQuery is the path and query exactly as
// sent (not a full URL); a WebSocket handshake is signed as a GET of its
// path. A GET takes an empty idempotency key, any other method a valid one.
func (s Signer) Sign(method, pathWithQuery, idempotencyKey string, body []byte, at time.Time) (Headers, error) {
	method = strings.ToUpper(method)
	if method == "GET" {
		if idempotencyKey != "" {
			return Headers{}, fmt.Errorf("%w: a signed GET sends an empty one", ErrInvalidIdempotencyKey)
		}
	} else if !idempotentKey.MatchString(idempotencyKey) {
		return Headers{}, fmt.Errorf("%w: 5 to 128 characters of A-Z a-z 0-9 . _ - ~", ErrInvalidIdempotencyKey)
	}
	if len(s.Key) != ed25519.PrivateKeySize {
		return Headers{}, ErrInvalidKey
	}
	ms := at.UnixMilli()
	// Serialize once: the same bytes are encoded and signed. No HTML
	// escaping, so a query's "&" stays "&" as in Exness's own examples.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(signedData{
		APIKey:         s.APIKey,
		IdempotencyKey: idempotencyKey,
		Timestamp:      ms,
		SignVersion:    SignVersion,
		Method:         method,
		Path:           pathWithQuery,
		BodyHash:       bodyHash(body),
	}); err != nil {
		return Headers{}, err
	}
	payload := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return Headers{
		APIKey:         s.APIKey,
		IdempotencyKey: idempotencyKey,
		Timestamp:      strconv.FormatInt(ms, 10),
		SignVersion:    strconv.Itoa(SignVersion),
		Data:           base64.RawURLEncoding.EncodeToString(payload),
		Sign:           base64.RawURLEncoding.EncodeToString(ed25519.Sign(s.Key, payload)),
	}, nil
}
