package server

import (
	"encoding/binary"
	"errors"
	"time"
	"unicode/utf16"
)

// SMB2 header layout (MS-SMB2 2.2.1).
const headerSize = 64

var protocolID = [4]byte{0xFE, 'S', 'M', 'B'}

// Commands.
const (
	cmdNegotiate      uint16 = 0x00
	cmdSessionSetup   uint16 = 0x01
	cmdLogoff         uint16 = 0x02
	cmdTreeConnect    uint16 = 0x03
	cmdTreeDisconnect uint16 = 0x04
	cmdCreate         uint16 = 0x05
	cmdClose          uint16 = 0x06
	cmdFlush          uint16 = 0x07
	cmdRead           uint16 = 0x08
	cmdWrite          uint16 = 0x09
	cmdLock           uint16 = 0x0A
	cmdIoctl          uint16 = 0x0B
	cmdCancel         uint16 = 0x0C
	cmdEcho           uint16 = 0x0D
	cmdQueryDirectory uint16 = 0x0E
	cmdChangeNotify   uint16 = 0x0F
	cmdQueryInfo      uint16 = 0x10
	cmdSetInfo        uint16 = 0x11
	cmdOplockBreak    uint16 = 0x12
)

// Header flags.
const (
	flagServerToRedir uint32 = 0x00000001
	flagAsync         uint32 = 0x00000002
	flagRelated       uint32 = 0x00000004
	flagSigned        uint32 = 0x00000008
)

// Dialects.
const (
	dialect202      uint16 = 0x0202
	dialect210      uint16 = 0x0210
	dialect300      uint16 = 0x0300
	dialect302      uint16 = 0x0302
	dialect311      uint16 = 0x0311
	dialectWildcard uint16 = 0x02FF
)

// NTSTATUS values used by the server.
const (
	statusSuccess                uint32 = 0x00000000
	statusPending                uint32 = 0x00000103
	statusNotifyCleanup          uint32 = 0x0000010B
	statusBufferOverflow         uint32 = 0x80000005
	statusNoMoreFiles            uint32 = 0x80000006
	statusNotImplemented         uint32 = 0xC0000002
	statusInvalidInfoClass       uint32 = 0xC0000003
	statusInfoLengthMismatch     uint32 = 0xC0000004
	statusInvalidHandle          uint32 = 0xC0000008
	statusInvalidParameter       uint32 = 0xC000000D
	statusNoSuchFile             uint32 = 0xC000000F
	statusInvalidDeviceRequest   uint32 = 0xC0000010
	statusEndOfFile              uint32 = 0xC0000011
	statusMoreProcessingRequired uint32 = 0xC0000016
	statusAccessDenied           uint32 = 0xC0000022
	statusBufferTooSmall         uint32 = 0xC0000023
	statusObjectNameInvalid      uint32 = 0xC0000033
	statusObjectNameNotFound     uint32 = 0xC0000034
	statusObjectNameCollision    uint32 = 0xC0000035
	statusObjectPathNotFound     uint32 = 0xC000003A
	statusSharingViolation       uint32 = 0xC0000043
	statusDeletePending          uint32 = 0xC0000056
	statusLogonFailure           uint32 = 0xC000006D
	statusDiskFull               uint32 = 0xC000007F
	statusMediaWriteProtected    uint32 = 0xC00000A2
	statusFileIsADirectory       uint32 = 0xC00000BA
	statusNotSupported           uint32 = 0xC00000BB
	statusNetworkNameDeleted     uint32 = 0xC00000C9
	statusBadNetworkName         uint32 = 0xC00000CC
	statusDirectoryNotEmpty      uint32 = 0xC0000101
	statusNotADirectory          uint32 = 0xC0000103
	statusCancelled              uint32 = 0xC0000120
	statusFileClosed             uint32 = 0xC0000128
	statusFSDriverRequired       uint32 = 0xC000019C
	statusUserSessionDeleted     uint32 = 0xC0000203
	statusNotFound               uint32 = 0xC0000225
)

// header is a parsed SMB2 header. Sync and async headers share the struct:
// AsyncID is set when flagAsync is, TreeID otherwise.
type header struct {
	CreditCharge uint16
	Status       uint32 // ChannelSequence in requests; unused
	Command      uint16
	Credits      uint16 // CreditRequest / CreditResponse
	Flags        uint32
	NextCommand  uint32
	MessageID    uint64
	AsyncID      uint64
	TreeID       uint32
	SessionID    uint64
	Signature    [16]byte
}

var errShort = errors.New("smb2: message too short")

func parseHeader(b []byte) (header, error) {
	var h header
	if len(b) < headerSize || [4]byte(b[0:4]) != protocolID {
		return h, errShort
	}
	h.CreditCharge = le16(b[6:])
	h.Status = le32(b[8:])
	h.Command = le16(b[12:])
	h.Credits = le16(b[14:])
	h.Flags = le32(b[16:])
	h.NextCommand = le32(b[20:])
	h.MessageID = le64(b[24:])
	if h.Flags&flagAsync != 0 {
		h.AsyncID = le64(b[32:])
	} else {
		h.TreeID = le32(b[36:])
	}
	h.SessionID = le64(b[40:])
	copy(h.Signature[:], b[48:64])
	return h, nil
}

// appendHeader writes a 64-byte header; the signature is left zero.
func appendHeader(b []byte, h *header) []byte {
	b = append(b, protocolID[:]...)
	b = put16(b, headerSize)
	b = put16(b, h.CreditCharge)
	b = put32(b, h.Status)
	b = put16(b, h.Command)
	b = put16(b, h.Credits)
	b = put32(b, h.Flags)
	b = put32(b, h.NextCommand)
	b = put64(b, h.MessageID)
	if h.Flags&flagAsync != 0 {
		b = put64(b, h.AsyncID)
	} else {
		b = put32(b, 0xFEFF) // Reserved (ProcessId)
		b = put32(b, h.TreeID)
	}
	b = put64(b, h.SessionID)
	return append(b, make([]byte, 16)...)
}

// errorBody is the 9-byte SMB2 ERROR response body.
func errorBody() []byte { return []byte{9, 0, 0, 0, 0, 0, 0, 0, 0} }

// Little-endian helpers. Readers assume the caller checked the length.

func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }
func le64(b []byte) uint64 { return binary.LittleEndian.Uint64(b) }

func put16(b []byte, v uint16) []byte { return binary.LittleEndian.AppendUint16(b, v) }
func put32(b []byte, v uint32) []byte { return binary.LittleEndian.AppendUint32(b, v) }
func put64(b []byte, v uint64) []byte { return binary.LittleEndian.AppendUint64(b, v) }

func set16(b []byte, off int, v uint16) { binary.LittleEndian.PutUint16(b[off:], v) }
func set32(b []byte, off int, v uint32) { binary.LittleEndian.PutUint32(b[off:], v) }

// pad8 pads b with zeros to a multiple of 8 bytes, measured from base.
func pad8(b []byte, base int) []byte {
	for (len(b)-base)%8 != 0 {
		b = append(b, 0)
	}
	return b
}

// field returns b[off:off+n] if the range is inside b.
func field(b []byte, off, n int) ([]byte, bool) {
	if off < 0 || n < 0 || off+n > len(b) {
		return nil, false
	}
	return b[off : off+n], true
}

func utf16Decode(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = le16(b[2*i:])
	}
	return string(utf16.Decode(u))
}

func utf16Encode(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, 0, 2*len(u))
	for _, c := range u {
		b = put16(b, c)
	}
	return b
}

// FILETIME: 100ns ticks since 1601-01-01 UTC.
const filetimeEpochDelta = 116444736000000000

func filetime(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	return uint64(t.UnixNano()/100 + filetimeEpochDelta)
}

// fromFiletime converts a FILETIME; ok is false for 0 and -1 ("don't change").
func fromFiletime(v uint64) (time.Time, bool) {
	if v == 0 || v == 0xFFFFFFFFFFFFFFFF || v == 0xFFFFFFFFFFFFFFFE {
		return time.Time{}, false
	}
	return time.Unix(0, (int64(v)-filetimeEpochDelta)*100), true
}
