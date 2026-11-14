package san_exness

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

// rfc8032Key is the public test key of RFC 8032 (Test 1), the key of
// Exness's own signing test vector. It is not a credential.
const rfc8032Key = "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60" +
	"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"

func testKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	key, err := ParsePrivateKey(rfc8032Key)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// The vector from Exness's Code Examples page (Test vector): the same inputs
// always give the same EXN-DATA and EXN-SIGN.
func TestSignMatchesExnessTestVector(t *testing.T) {
	s := Signer{APIKey: "exnsk-synthetic-example", Key: testKey(t)}
	h, err := s.Sign("GET", "/v1/trading/access-point?account_id=123456", "", nil, time.UnixMilli(1773656070123))
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"api_key":"exnsk-synthetic-example","idempotency_key":"","timestamp":1773656070123,"sign_version":1,` +
		`"method":"GET","path":"/v1/trading/access-point?account_id=123456","body_hash":"47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU"}`
	want := Headers{
		APIKey:         "exnsk-synthetic-example",
		IdempotencyKey: "",
		Timestamp:      "1773656070123",
		SignVersion:    "1",
		Data:           base64.RawURLEncoding.EncodeToString([]byte(payload)),
		Sign:           "l-H_Acia8YoTjKFMg-3fon3NOEX8TalNVA7c01DQ9dHb7pVBJC4y06qC-DbpETzh1KToiOprqEV6uzN3t69ICQ",
	}
	if h != want {
		t.Fatalf("headers\n got %+v\nwant %+v", h, want)
	}
}

func TestSignHashesTheBody(t *testing.T) {
	s := Signer{APIKey: "exnsk-x", Key: testKey(t)}
	body := []byte(`{"instrument":"EURUSD","side":"buy","volume":"0.10"}`)
	h, err := s.Sign("post", "/v1/trading/accounts/1/positions", "order-0001", body, time.UnixMilli(1))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := base64.RawURLEncoding.DecodeString(h.Data)
	for _, part := range []string{`"method":"POST"`, `"idempotency_key":"order-0001"`, `"body_hash":"` + bodyHash(body) + `"`} {
		if !strings.Contains(string(data), part) {
			t.Errorf("signed data %s lacks %s", data, part)
		}
	}
	q, err := s.Sign("GET", "/v1/history/accounts/1/deals?from=a&limit=5", "", nil, time.UnixMilli(1))
	if err != nil {
		t.Fatal(err)
	}
	if qd, _ := base64.RawURLEncoding.DecodeString(q.Data); !strings.Contains(string(qd), `"path":"/v1/history/accounts/1/deals?from=a&limit=5"`) {
		t.Errorf("signed data %s escapes the query", qd)
	}
	if bodyHash(body) == bodyHash(nil) {
		t.Fatal("a body hashes like no body")
	}
	sig, _ := base64.RawURLEncoding.DecodeString(h.Sign)
	if !ed25519.Verify(testKey(t).Public().(ed25519.PublicKey), data, sig) {
		t.Fatal("signature does not verify")
	}
}

func TestSignIdempotencyRules(t *testing.T) {
	s := Signer{APIKey: "exnsk-x", Key: testKey(t)}
	for _, c := range []struct {
		method, key string
		ok          bool
	}{
		{"GET", "", true},
		{"GET", "abcde", false}, // a signed GET carries an empty key
		{"POST", "", false},     // a mutating request needs one
		{"POST", "abcd", false}, // 5..128 characters
		{"POST", "abcde", true},
		{"DELETE", strings.Repeat("a", 128), true},
		{"PATCH", strings.Repeat("a", 129), false},
		{"POST", "order 1", false}, // A-Z a-z 0-9 . _ - ~ only
		{"POST", "order-1.a_b~c", true},
	} {
		_, err := s.Sign(c.method, "/p", c.key, nil, time.UnixMilli(1))
		if (err == nil) != c.ok {
			t.Errorf("%s with key %q: err %v, want ok=%v", c.method, c.key, err, c.ok)
		}
	}
}

func TestParsePrivateKey(t *testing.T) {
	want := testKey(t)
	der, err := x509.MarshalPKCS8PrivateKey(want)
	if err != nil {
		t.Fatal(err)
	}
	pemText := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	for name, text := range map[string]string{
		"hex":             rfc8032Key,
		"hex upper":       strings.ToUpper(rfc8032Key),
		"hex with spaces": "  " + rfc8032Key + "\n",
		"PKCS#8 PEM":      pemText,
	} {
		got, err := ParsePrivateKey(text)
		if err != nil || !got.Equal(want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	other := ed25519.NewKeyFromSeed(make([]byte, 32))
	mismatched := rfc8032Key[:64] + hex.EncodeToString(other[32:])
	for name, text := range map[string]string{
		"empty":               "",
		"short hex":           rfc8032Key[:126],
		"seed only":           rfc8032Key[:64],
		"public half differs": mismatched,
		"not a key":           "exnsk-synthetic-example",
		"bad PEM":             "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n",
	} {
		if _, err := ParsePrivateKey(text); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

// Errors about a key never repeat it.
func TestParsePrivateKeyErrorHidesTheKey(t *testing.T) {
	bad := rfc8032Key[:127] + "z"
	_, err := ParsePrivateKey(bad)
	if err == nil || strings.Contains(err.Error(), rfc8032Key[:20]) {
		t.Fatalf("err = %v", err)
	}
}
