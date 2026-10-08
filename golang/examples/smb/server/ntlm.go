package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"errors"
	"strings"

	"golang.org/x/crypto/md4"
)

// NTLM (MS-NLMP), server side, NTLMv2 only.

var ntlmSignature = []byte("NTLMSSP\x00")

const (
	ntlmNegotiateUnicode   uint32 = 0x00000001
	ntlmRequestTarget      uint32 = 0x00000004
	ntlmNegotiateSign      uint32 = 0x00000010
	ntlmNegotiateSeal      uint32 = 0x00000020
	ntlmNegotiateNTLM      uint32 = 0x00000200
	ntlmAlwaysSign         uint32 = 0x00008000
	ntlmTargetTypeServer   uint32 = 0x00020000
	ntlmExtendedSession    uint32 = 0x00080000
	ntlmNegotiateTargetInf uint32 = 0x00800000
	ntlmNegotiateVersion   uint32 = 0x02000000
	ntlmNegotiate128       uint32 = 0x20000000
	ntlmNegotiateKeyExch   uint32 = 0x40000000
	ntlmNegotiate56        uint32 = 0x80000000
)

// identity is who a session belongs to.
type identity struct {
	User      string
	Domain    string
	Guest     bool // unknown user mapped to guest
	Anonymous bool // null session
}

var errLogon = errors.New("ntlm: logon failure")

// ntlmServer runs one NTLM exchange: NEGOTIATE -> CHALLENGE -> AUTHENTICATE.
type ntlmServer struct {
	users      map[string]string // lower-case user name -> password
	allowGuest bool
	name       string // NetBIOS server name, upper case

	challenge [8]byte
	flags     uint32
}

func isNTLM(b []byte) bool { return bytes.HasPrefix(b, ntlmSignature) && len(b) >= 12 }

func ntlmType(b []byte) uint32 { return le32(b[8:]) }

// challengeFor answers a NEGOTIATE message with a CHALLENGE message.
func (s *ntlmServer) challengeFor(negotiate []byte) ([]byte, error) {
	if !isNTLM(negotiate) || ntlmType(negotiate) != 1 || len(negotiate) < 16 {
		return nil, errors.New("ntlm: expected NEGOTIATE")
	}
	client := le32(negotiate[12:])
	s.flags = ntlmNegotiateUnicode | ntlmRequestTarget | ntlmNegotiateNTLM | ntlmAlwaysSign |
		ntlmTargetTypeServer | ntlmExtendedSession | ntlmNegotiateTargetInf | ntlmNegotiateVersion |
		ntlmNegotiate128 | ntlmNegotiate56 |
		client&(ntlmNegotiateSign|ntlmNegotiateSeal|ntlmNegotiateKeyExch)
	if _, err := rand.Read(s.challenge[:]); err != nil {
		return nil, err
	}

	name := utf16Encode(s.name)
	domain := utf16Encode("WORKGROUP")
	// No MsvAvTimestamp: with it, clients add a MIC and then expect a SPNEGO
	// mechListMIC from the server, which this server does not produce.
	var info []byte
	info = appendAV(info, 2, domain) // MsvAvNbDomainName
	info = appendAV(info, 1, name)   // MsvAvNbComputerName
	info = appendAV(info, 4, domain) // MsvAvDnsDomainName
	info = appendAV(info, 3, name)   // MsvAvDnsComputerName
	info = appendAV(info, 0, nil)    // MsvAvEOL

	const fixed = 56
	m := append([]byte{}, ntlmSignature...)
	m = put32(m, 2)
	m = appendSecBuf(m, len(name), fixed)
	m = put32(m, s.flags)
	m = append(m, s.challenge[:]...)
	m = append(m, make([]byte, 8)...)
	m = appendSecBuf(m, len(info), fixed+len(name))
	m = append(m, 10, 0, 0x61, 0x4A, 0, 0, 0, 15) // Version: 10.0.19041, NTLM revision 15
	m = append(m, name...)
	m = append(m, info...)
	return m, nil
}

func appendAV(b []byte, id uint16, v []byte) []byte {
	b = put16(b, id)
	b = put16(b, uint16(len(v)))
	return append(b, v...)
}

func appendSecBuf(b []byte, n, off int) []byte {
	b = put16(b, uint16(n))
	b = put16(b, uint16(n))
	return put32(b, uint32(off))
}

func secBuf(m []byte, at int) ([]byte, bool) {
	if at+8 > len(m) {
		return nil, false
	}
	return field(m, int(le32(m[at+4:])), int(le16(m[at:])))
}

// authenticate checks an AUTHENTICATE message and returns the identity and
// the exported session key (nil for guest and anonymous sessions).
func (s *ntlmServer) authenticate(m []byte) (identity, []byte, error) {
	if !isNTLM(m) || ntlmType(m) != 3 || len(m) < 64 {
		return identity{}, nil, errors.New("ntlm: expected AUTHENTICATE")
	}
	lm, ok1 := secBuf(m, 12)
	nt, ok2 := secBuf(m, 20)
	dom, ok3 := secBuf(m, 28)
	usr, ok4 := secBuf(m, 36)
	encKey, ok5 := secBuf(m, 52)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return identity{}, nil, errors.New("ntlm: bad AUTHENTICATE")
	}
	flags := le32(m[60:])
	user, domain := utf16Decode(usr), utf16Decode(dom)

	if user == "" && len(nt) == 0 && (len(lm) == 0 || bytes.Equal(lm, []byte{0})) {
		if !s.allowGuest {
			return identity{}, nil, errLogon
		}
		return identity{Anonymous: true}, nil, nil
	}

	password, known := s.users[strings.ToLower(user)]
	if !known {
		if s.allowGuest {
			return identity{User: user, Domain: domain, Guest: true}, nil, nil
		}
		return identity{}, nil, errLogon
	}
	if len(nt) <= 24 { // NTLMv1 responses are exactly 24 bytes; not accepted
		return identity{}, nil, errLogon
	}

	h := md4.New()
	h.Write(utf16Encode(password))
	ntHash := h.Sum(nil)
	proof, blob := nt[:16], nt[16:]

	// Clients differ in which domain they hash in; accept the common forms.
	for _, d := range uniq(domain, strings.ToUpper(domain), "") {
		owf := hmacMD5(ntHash, utf16Encode(strings.ToUpper(user)+d))
		if !hmac.Equal(hmacMD5(owf, s.challenge[:], blob), proof) {
			continue
		}
		key := hmacMD5(owf, proof)
		if flags&ntlmNegotiateKeyExch != 0 && len(encKey) == 16 {
			c, _ := rc4.NewCipher(key)
			exported := make([]byte, 16)
			c.XORKeyStream(exported, encKey)
			key = exported
		}
		return identity{User: user, Domain: domain}, key, nil
	}
	return identity{}, nil, errLogon
}

func hmacMD5(key []byte, parts ...[]byte) []byte {
	h := hmac.New(md5.New, key)
	for _, p := range parts {
		h.Write(p)
	}
	return h.Sum(nil)
}

func uniq(xs ...string) []string {
	var out []string
	for _, x := range xs {
		dup := false
		for _, y := range out {
			dup = dup || x == y
		}
		if !dup {
			out = append(out, x)
		}
	}
	return out
}
