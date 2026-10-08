package server

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
)

// kdf is the SP800-108 counter-mode KDF with HMAC-SHA256 that SMB 3.x uses
// to derive 128-bit keys (MS-SMB2 3.1.4.2).
func kdf(key, label, context []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte{0, 0, 0, 1})
	h.Write(label)
	h.Write([]byte{0})
	h.Write(context)
	h.Write([]byte{0, 0, 0, 128})
	return h.Sum(nil)[:16]
}

// signingKey derives a session's signing key from its 16-byte session key.
func signingKey(dialect uint16, sessionKey, preauth []byte) []byte {
	switch {
	case dialect >= dialect311:
		return kdf(sessionKey, []byte("SMBSigningKey\x00"), preauth)
	case dialect >= dialect300:
		return kdf(sessionKey, []byte("SMB2AESCMAC\x00"), []byte("SmbSign\x00"))
	default:
		return sessionKey
	}
}

// signature computes the 16-byte signature of msg, whose signature field
// must be zero: HMAC-SHA256 for SMB 2.x, AES-128-CMAC for SMB 3.x.
func signature(dialect uint16, key, msg []byte) [16]byte {
	var out [16]byte
	if dialect >= dialect300 {
		return aesCMAC(key, msg)
	}
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	copy(out[:], h.Sum(nil))
	return out
}

// sign sets the signed flag and writes the signature into msg in place.
func sign(dialect uint16, key, msg []byte) {
	set32(msg, 16, le32(msg[16:])|flagSigned)
	clear(msg[48:64])
	s := signature(dialect, key, msg)
	copy(msg[48:64], s[:])
}

// verify checks the signature of a received message. msg is modified
// (signature zeroed) and restored.
func verify(dialect uint16, key, msg []byte) bool {
	var got [16]byte
	copy(got[:], msg[48:64])
	clear(msg[48:64])
	want := signature(dialect, key, msg)
	copy(msg[48:64], got[:])
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

// aesCMAC is AES-128-CMAC (RFC 4493).
func aesCMAC(key, msg []byte) [16]byte {
	c, err := aes.NewCipher(key)
	if err != nil {
		panic(err) // key is always 16 bytes
	}
	var l, k1, k2 [16]byte
	c.Encrypt(l[:], l[:])
	k1 = cmacDouble(l)
	k2 = cmacDouble(k1)

	n := (len(msg) + 15) / 16
	complete := n > 0 && len(msg)%16 == 0
	if n == 0 {
		n = 1
	}
	var last [16]byte
	tail := msg[(n-1)*16:]
	if complete {
		for i := range last {
			last[i] = tail[i] ^ k1[i]
		}
	} else {
		copy(last[:], tail)
		last[len(tail)] = 0x80
		for i := range last {
			last[i] ^= k2[i]
		}
	}
	var x [16]byte
	for i := 0; i < n-1; i++ {
		for j := range x {
			x[j] ^= msg[i*16+j]
		}
		c.Encrypt(x[:], x[:])
	}
	for j := range x {
		x[j] ^= last[j]
	}
	c.Encrypt(x[:], x[:])
	return x
}

func cmacDouble(in [16]byte) [16]byte {
	var out [16]byte
	for i := 0; i < 15; i++ {
		out[i] = in[i]<<1 | in[i+1]>>7
	}
	out[15] = in[15] << 1
	if in[0]&0x80 != 0 {
		out[15] ^= 0x87
	}
	return out
}

// preauthUpdate extends an SMB 3.1.1 preauth integrity hash with a message.
func preauthUpdate(hash []byte, msg []byte) []byte {
	h := sha512.New()
	h.Write(hash)
	h.Write(msg)
	return h.Sum(nil)
}
