package server

import (
	"bytes"
	"errors"
)

// SPNEGO (RFC 4178) wrapping of NTLM tokens, with just enough DER to read
// NegTokenInit/NegTokenResp and to write the server's replies.

var (
	oidSPNEGO = []byte{0x2b, 0x06, 0x01, 0x05, 0x05, 0x02}                         // 1.3.6.1.5.5.2
	oidNTLM   = []byte{0x2b, 0x06, 0x01, 0x04, 0x01, 0x82, 0x37, 0x02, 0x02, 0x0a} // 1.3.6.1.4.1.311.2.2.10
)

// SPNEGO negState values.
const (
	negAcceptCompleted  = 0
	negAcceptIncomplete = 1
	negReject           = 2
)

func derTLV(tag byte, content ...[]byte) []byte {
	n := 0
	for _, c := range content {
		n += len(c)
	}
	b := []byte{tag}
	switch {
	case n < 0x80:
		b = append(b, byte(n))
	case n < 0x100:
		b = append(b, 0x81, byte(n))
	case n < 0x10000:
		b = append(b, 0x82, byte(n>>8), byte(n))
	default:
		b = append(b, 0x83, byte(n>>16), byte(n>>8), byte(n))
	}
	for _, c := range content {
		b = append(b, c...)
	}
	return b
}

// derNext splits the first TLV off b.
func derNext(b []byte) (tag byte, content, rest []byte, err error) {
	if len(b) < 2 {
		return 0, nil, nil, errors.New("der: short")
	}
	tag, n, i := b[0], int(b[1]), 2
	if n&0x80 != 0 {
		k := n & 0x7f
		if k == 0 || k > 3 || len(b) < 2+k {
			return 0, nil, nil, errors.New("der: bad length")
		}
		n = 0
		for _, c := range b[2 : 2+k] {
			n = n<<8 | int(c)
		}
		i += k
	}
	if len(b) < i+n {
		return 0, nil, nil, errors.New("der: truncated")
	}
	return tag, b[i : i+n], b[i+n:], nil
}

// negHint is the security buffer of the NEGOTIATE response: a NegTokenInit
// offering only NTLM.
func negHint() []byte {
	mechs := derTLV(0xa0, derTLV(0x30, derTLV(0x06, oidNTLM)))
	return derTLV(0x60, derTLV(0x06, oidSPNEGO), derTLV(0xa0, derTLV(0x30, mechs)))
}

// negResp builds a NegTokenResp.
func negResp(state int, withMech bool, token []byte) []byte {
	var parts [][]byte
	parts = append(parts, derTLV(0xa0, derTLV(0x0a, []byte{byte(state)})))
	if withMech {
		parts = append(parts, derTLV(0xa1, derTLV(0x06, oidNTLM)))
	}
	if token != nil {
		parts = append(parts, derTLV(0xa2, derTLV(0x04, token)))
	}
	return derTLV(0xa1, derTLV(0x30, parts...))
}

// spnegoToken is what a SESSION_SETUP security buffer carried.
type spnegoToken struct {
	raw       bool   // a bare NTLM message, no SPNEGO
	init      bool   // NegTokenInit (first leg)
	ntlmOffer bool   // NegTokenInit listed NTLM among its mechanisms
	mech      []byte // the embedded mechanism token, if any
}

func parseSPNEGO(b []byte) (spnegoToken, error) {
	if isNTLM(b) {
		return spnegoToken{raw: true, mech: b}, nil
	}
	tag, body, _, err := derNext(b)
	if err != nil {
		return spnegoToken{}, err
	}
	switch tag {
	case 0x60: // GSS-API InitialContextToken: OID, then [0] NegTokenInit
		t, oid, rest, err := derNext(body)
		if err != nil || t != 0x06 || !bytes.Equal(oid, oidSPNEGO) {
			return spnegoToken{}, errors.New("spnego: not SPNEGO")
		}
		t, init, _, err := derNext(rest)
		if err != nil || t != 0xa0 {
			return spnegoToken{}, errors.New("spnego: expected NegTokenInit")
		}
		return parseNegTokenInit(init)
	case 0xa0:
		return parseNegTokenInit(body)
	case 0xa1:
		t, seq, _, err := derNext(body)
		if err != nil || t != 0x30 {
			return spnegoToken{}, errors.New("spnego: bad NegTokenResp")
		}
		tok := spnegoToken{}
		for len(seq) > 0 {
			var f []byte
			if t, f, seq, err = derNext(seq); err != nil {
				return spnegoToken{}, err
			}
			if t == 0xa2 { // responseToken
				if _, tok.mech, _, err = derNext(f); err != nil {
					return spnegoToken{}, err
				}
			}
		}
		return tok, nil
	}
	return spnegoToken{}, errors.New("spnego: unknown token")
}

func parseNegTokenInit(b []byte) (spnegoToken, error) {
	t, seq, _, err := derNext(b)
	if err != nil || t != 0x30 {
		return spnegoToken{}, errors.New("spnego: bad NegTokenInit")
	}
	tok := spnegoToken{init: true}
	for len(seq) > 0 {
		var f []byte
		if t, f, seq, err = derNext(seq); err != nil {
			return spnegoToken{}, err
		}
		switch t {
		case 0xa0: // mechTypes
			_, list, _, err := derNext(f)
			if err != nil {
				return spnegoToken{}, err
			}
			for len(list) > 0 {
				var oid []byte
				if _, oid, list, err = derNext(list); err != nil {
					return spnegoToken{}, err
				}
				tok.ntlmOffer = tok.ntlmOffer || bytes.Equal(oid, oidNTLM)
			}
		case 0xa2: // mechToken
			if _, tok.mech, _, err = derNext(f); err != nil {
				return spnegoToken{}, err
			}
		}
	}
	return tok, nil
}

// authExchange drives SPNEGO+NTLM for one session across SESSION_SETUP legs.
type authExchange struct {
	ntlm ntlmServer
	raw  bool // the client speaks bare NTLM; answer the same way
}

// step consumes one security buffer. It returns the reply buffer, and done
// with the identity and session key once the client is authenticated.
func (a *authExchange) step(in []byte) (out []byte, done bool, id identity, key []byte, err error) {
	tok, err := parseSPNEGO(in)
	if err != nil {
		return nil, false, id, nil, err
	}
	a.raw = tok.raw
	wrap := func(state int, withMech bool, t []byte) []byte {
		if a.raw {
			return t
		}
		return negResp(state, withMech, t)
	}

	switch {
	case isNTLM(tok.mech) && ntlmType(tok.mech) == 1:
		ch, err := a.ntlm.challengeFor(tok.mech)
		if err != nil {
			return nil, false, id, nil, err
		}
		return wrap(negAcceptIncomplete, true, ch), false, id, nil, nil
	case isNTLM(tok.mech) && ntlmType(tok.mech) == 3:
		id, key, err := a.ntlm.authenticate(tok.mech)
		if err != nil {
			return wrap(negReject, false, nil), false, id, nil, err
		}
		if a.raw {
			return nil, true, id, key, nil
		}
		return negResp(negAcceptCompleted, false, nil), true, id, key, nil
	case tok.init && tok.ntlmOffer:
		// The client's optimistic token is for another mechanism (Kerberos,
		// NEGOEX): ask for NTLM instead.
		return negResp(negAcceptIncomplete, true, nil), false, id, nil, nil
	}
	return nil, false, id, nil, errors.New("spnego: no NTLM token")
}
