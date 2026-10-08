package server

import (
	"os"
	"path"
	"strings"
	"time"
)

// Directory info classes (MS-FSCC 2.4).
const (
	fileDirectoryInformation       = 0x01
	fileFullDirectoryInformation   = 0x02
	fileBothDirectoryInformation   = 0x03
	fileNamesInformation           = 0x0C
	fileIdBothDirectoryInformation = 0x25
	fileIdFullDirectoryInformation = 0x26
)

// QUERY_DIRECTORY flags.
const (
	qdRestartScans = 0x01
	qdSingleEntry  = 0x02
	qdReopen       = 0x10
)

func (c *conn) doQueryDirectory(msg []byte, h *header, lastFID *[16]byte) (uint32, []byte) {
	t, st := c.treeFor(h)
	if st != statusSuccess {
		return st, errorBody()
	}
	body := msg[headerSize:]
	if len(body) < 32 {
		return statusInvalidParameter, errorBody()
	}
	class := int(body[2])
	flags := body[3]
	fid, ok := resolveFID(msg, headerSize+8, lastFID)
	if !ok {
		return statusInvalidParameter, errorBody()
	}
	nameOff := int(le16(body[24:]))
	nameLen := int(le16(body[26:]))
	outMax := int(le32(body[28:]))

	pattern := ""
	if nameLen > 0 {
		if raw, ok := field(msg, nameOff, nameLen); ok {
			pattern = utf16Decode(raw)
		}
	}

	c.mu.Lock()
	obj := t.opens[fid]
	c.mu.Unlock()
	if obj == nil {
		return statusInvalidHandle, errorBody()
	}
	if !obj.isDir {
		return statusInvalidParameter, errorBody()
	}

	if flags&(qdRestartScans|qdReopen) != 0 || !obj.started {
		if st := obj.loadEntries(pattern); st != statusSuccess {
			return st, errorBody()
		}
		obj.cursor = 0
	}
	if obj.cursor >= len(obj.entries) {
		return statusNoMoreFiles, errorBody()
	}

	var buf []byte
	prevStart := -1
	for obj.cursor < len(obj.entries) {
		entry := encodeDirEntry(class, obj.entries[obj.cursor])
		if entry == nil {
			return statusInvalidInfoClass, errorBody()
		}
		// Pad the previous entry to an 8-byte boundary before appending.
		padded := buf
		if prevStart >= 0 {
			for len(padded)%8 != 0 {
				padded = append(padded, 0)
			}
		}
		if len(padded)+len(entry) > outMax {
			if prevStart < 0 {
				return statusInfoLengthMismatch, errorBody()
			}
			break
		}
		buf = padded
		start := len(buf)
		buf = append(buf, entry...)
		if prevStart >= 0 {
			set32(buf, prevStart, uint32(start-prevStart)) // NextEntryOffset
		}
		prevStart = start
		obj.cursor++
		if flags&qdSingleEntry != 0 {
			break
		}
	}
	// Last entry's NextEntryOffset stays 0.

	b := make([]byte, 8)
	set16(b, 0, 9)
	set16(b, 2, 72) // OutputBufferOffset = header(64)+fixed(8)
	set32(b, 4, uint32(len(buf)))
	return statusSuccess, append(b, buf...)
}

// encodeDirEntry encodes one directory entry for the given info class, with
// NextEntryOffset left zero (the caller patches it).
func encodeDirEntry(class int, fi os.FileInfo) []byte {
	name := utf16Encode(fi.Name())
	var b []byte
	put := func(n int) { b = append(b, make([]byte, n)...) }

	if class == fileNamesInformation {
		put(12) // NextEntryOffset(4) FileIndex(4) FileNameLength(4)
		set32(b, 8, uint32(len(name)))
		return append(b, name...)
	}

	// Common header through FileAttributes for the directory classes.
	put(8) // NextEntryOffset(4) + FileIndex(4)
	t := fi.ModTime()
	for _, off := range []int{8, 16, 24, 32} {
		buf := make([]byte, 8)
		set64(buf, 0, filetime(t))
		b = append(b, buf...)
		_ = off
	}
	size := uint64(fi.Size())
	app64 := func(v uint64) { x := make([]byte, 8); set64(x, 0, v); b = append(b, x...) }
	app64(size) // EndOfFile
	app64(size) // AllocationSize
	app32 := func(v uint32) { x := make([]byte, 4); set32(x, 0, v); b = append(b, x...) }
	app32(dosAttr(fi))       // FileAttributes
	app32(uint32(len(name))) // FileNameLength

	switch class {
	case fileDirectoryInformation:
		// no extra fields
	case fileFullDirectoryInformation:
		app32(0) // EaSize
	case fileBothDirectoryInformation:
		app32(0)                           // EaSize
		b = append(b, 0, 0)                // ShortNameLength(1) + Reserved(1)
		b = append(b, make([]byte, 24)...) // ShortName[24]
	case fileIdFullDirectoryInformation:
		app32(0)           // EaSize
		app32(0)           // Reserved
		app64(inodeOf(fi)) // FileId(8)
	case fileIdBothDirectoryInformation:
		app32(0)                           // EaSize
		b = append(b, 0, 0)                // ShortNameLength + Reserved
		b = append(b, make([]byte, 24)...) // ShortName[24]
		b = append(b, 0, 0)                // Reserved2(2)
		app64(inodeOf(fi))                 // FileId(8)
	default:
		return nil
	}
	return append(b, name...)
}

func inodeOf(fi os.FileInfo) uint64 {
	// A stable-ish id from the name; real inode numbers aren't portable.
	var h uint64 = 1469598103934665603
	for _, c := range []byte(fi.Name()) {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}

// File info classes (MS-FSCC 2.4) for QUERY_INFO / SET_INFO.
const (
	infoFile   = 0x01
	infoFs     = 0x02
	infoSecure = 0x03

	fileBasicInformation       = 0x04
	fileStandardInformation    = 0x05
	fileInternalInformation    = 0x06
	fileEaInformation          = 0x07
	fileNameInformation        = 0x09
	fileRenameInformation      = 0x0A
	fileDispositionInformation = 0x0D
	filePositionInformation    = 0x0E
	fileAllocationInformation  = 0x13
	fileEndOfFileInformation   = 0x14
	fileAllInformation         = 0x12
	fileNetworkOpenInformation = 0x22
	fileAttributeTagInfo       = 0x23

	fsVolumeInformation    = 0x01
	fsSizeInformation      = 0x03
	fsDeviceInformation    = 0x04
	fsAttributeInformation = 0x05
	fsFullSizeInformation  = 0x07
)

func (c *conn) doQueryInfo(msg []byte, h *header, lastFID *[16]byte) (uint32, []byte) {
	t, st := c.treeFor(h)
	if st != statusSuccess {
		return st, errorBody()
	}
	body := msg[headerSize:]
	if len(body) < 40 {
		return statusInvalidParameter, errorBody()
	}
	infoType := body[2]
	class := body[3]
	outMax := int(le32(body[4:]))
	fid, ok := resolveFID(msg, headerSize+24, lastFID)
	if !ok {
		return statusInvalidParameter, errorBody()
	}
	c.mu.Lock()
	obj := t.opens[fid]
	c.mu.Unlock()
	if obj == nil {
		return statusInvalidHandle, errorBody()
	}

	var out []byte
	var status uint32 = statusSuccess
	switch infoType {
	case infoFile:
		out, status = queryFileInfo(obj, class)
	case infoFs:
		out, status = queryFsInfo(class)
	default:
		return statusNotSupported, errorBody()
	}
	if status != statusSuccess {
		return status, errorBody()
	}
	if len(out) > outMax {
		return statusBufferOverflow, errorBody()
	}

	b := make([]byte, 8)
	set16(b, 0, 9)
	set16(b, 2, 72)
	set32(b, 4, uint32(len(out)))
	return statusSuccess, append(b, out...)
}

func queryFileInfo(obj *object, class byte) ([]byte, uint32) {
	fi, err := os.Stat(obj.full)
	if err != nil {
		return nil, osErr(err)
	}
	switch class {
	case fileBasicInformation:
		return basicInfo(fi), statusSuccess
	case fileStandardInformation:
		return standardInfo(fi, obj.deleteOnClose), statusSuccess
	case fileInternalInformation:
		x := make([]byte, 8)
		set64(x, 0, inodeOf(fi))
		return x, statusSuccess
	case fileEaInformation:
		return make([]byte, 4), statusSuccess
	case filePositionInformation:
		return make([]byte, 8), statusSuccess
	case fileNameInformation:
		name := utf16Encode(smbPath(obj.path))
		x := make([]byte, 4)
		set32(x, 0, uint32(len(name)))
		return append(x, name...), statusSuccess
	case fileNetworkOpenInformation:
		return networkOpenInfo(fi), statusSuccess
	case fileAttributeTagInfo:
		x := make([]byte, 8)
		set32(x, 0, dosAttr(fi))
		return x, statusSuccess
	case fileAllInformation:
		return allInfo(fi, obj), statusSuccess
	default:
		return nil, statusInvalidInfoClass
	}
}

func basicInfo(fi os.FileInfo) []byte {
	b := make([]byte, 40)
	putTime(b, 0, fi.ModTime())
	putTime(b, 8, fi.ModTime())
	putTime(b, 16, fi.ModTime())
	putTime(b, 24, fi.ModTime())
	set32(b, 32, dosAttr(fi))
	return b
}

func standardInfo(fi os.FileInfo, deletePending bool) []byte {
	b := make([]byte, 24)
	set64(b, 0, uint64(fi.Size())) // AllocationSize
	set64(b, 8, uint64(fi.Size())) // EndOfFile
	set32(b, 16, 1)                // NumberOfLinks
	if deletePending {
		b[20] = 1
	}
	if fi.IsDir() {
		b[21] = 1
	}
	return b
}

func networkOpenInfo(fi os.FileInfo) []byte {
	b := make([]byte, 56)
	putTime(b, 0, fi.ModTime())
	putTime(b, 8, fi.ModTime())
	putTime(b, 16, fi.ModTime())
	putTime(b, 24, fi.ModTime())
	set64(b, 32, uint64(fi.Size()))
	set64(b, 40, uint64(fi.Size()))
	set32(b, 48, dosAttr(fi))
	return b
}

func allInfo(fi os.FileInfo, obj *object) []byte {
	b := basicInfo(fi)                                    // 40
	b = append(b, standardInfo(fi, obj.deleteOnClose)...) // +24 = 64
	internal := make([]byte, 8)
	set64(internal, 0, inodeOf(fi))
	b = append(b, internal...)        // +8 Internal
	b = append(b, make([]byte, 4)...) // EA
	access := make([]byte, 4)
	set32(access, 0, 0x001f01ff)
	b = append(b, access...)          // Access
	b = append(b, make([]byte, 8)...) // Position
	b = append(b, make([]byte, 4)...) // Mode
	b = append(b, make([]byte, 4)...) // Alignment
	name := utf16Encode(smbPath(obj.path))
	nb := make([]byte, 4)
	set32(nb, 0, uint32(len(name)))
	b = append(b, nb...)
	return append(b, name...)
}

func queryFsInfo(class byte) ([]byte, uint32) {
	switch class {
	case fsVolumeInformation:
		label := utf16Encode("shared")
		b := make([]byte, 18)
		set32(b, 8, 0x12345678) // VolumeSerialNumber
		set32(b, 12, uint32(len(label)))
		return append(b, label...), statusSuccess
	case fsSizeInformation:
		b := make([]byte, 24)
		set64(b, 0, 1<<30) // TotalAllocationUnits
		set64(b, 8, 1<<29) // AvailableAllocationUnits
		set32(b, 16, 8)    // SectorsPerAllocationUnit
		set32(b, 20, 512)  // BytesPerSector
		return b, statusSuccess
	case fsFullSizeInformation:
		b := make([]byte, 32)
		set64(b, 0, 1<<30)
		set64(b, 8, 1<<29)
		set64(b, 16, 1<<29)
		set32(b, 24, 8)
		set32(b, 28, 512)
		return b, statusSuccess
	case fsDeviceInformation:
		b := make([]byte, 8)
		set32(b, 0, 0x00000007) // FILE_DEVICE_DISK
		return b, statusSuccess
	case fsAttributeInformation:
		name := utf16Encode("NTFS")
		b := make([]byte, 12)
		set32(b, 0, 0x0000006f) // case-preserving, unicode, etc.
		set32(b, 4, 255)        // MaximumComponentNameLength
		set32(b, 8, uint32(len(name)))
		return append(b, name...), statusSuccess
	default:
		return nil, statusInvalidInfoClass
	}
}

// smbPath turns an SMB-relative path into the leading-backslash form clients
// expect from name information.
func smbPath(p string) string {
	p = strings.ReplaceAll(p, "/", `\`)
	p = strings.TrimLeft(p, `\`)
	return `\` + p
}

func (c *conn) doSetInfo(msg []byte, h *header, lastFID *[16]byte) (uint32, []byte) {
	t, st := c.treeFor(h)
	if st != statusSuccess {
		return st, errorBody()
	}
	body := msg[headerSize:]
	if len(body) < 32 {
		return statusInvalidParameter, errorBody()
	}
	infoType := body[2]
	class := body[3]
	bufLen := int(le32(body[4:]))
	bufOff := int(le16(body[8:]))
	fid, ok := resolveFID(msg, headerSize+16, lastFID)
	if !ok {
		return statusInvalidParameter, errorBody()
	}
	data, ok := field(msg, bufOff, bufLen)
	if !ok {
		return statusInvalidParameter, errorBody()
	}
	c.mu.Lock()
	obj := t.opens[fid]
	c.mu.Unlock()
	if obj == nil {
		return statusInvalidHandle, errorBody()
	}
	if infoType != infoFile {
		return statusNotSupported, errorBody()
	}
	if obj.share.ReadOnly {
		return statusMediaWriteProtected, errorBody()
	}

	switch class {
	case fileBasicInformation:
		return setBasic(obj, data), setInfoResp()
	case fileDispositionInformation:
		if len(data) >= 1 {
			obj.deleteOnClose = data[0]&1 != 0
		}
		return statusSuccess, setInfoRespBody()
	case fileEndOfFileInformation:
		if len(data) < 8 {
			return statusInvalidParameter, errorBody()
		}
		if err := obj.file.Truncate(int64(le64(data))); err != nil {
			return osErr(err), errorBody()
		}
		return statusSuccess, setInfoRespBody()
	case fileAllocationInformation, filePositionInformation:
		return statusSuccess, setInfoRespBody()
	case fileRenameInformation:
		return c.setRename(t, obj, data), setInfoRespBody()
	default:
		return statusInvalidInfoClass, errorBody()
	}
}

// setInfoResp returns the 2-byte SET_INFO success body; the odd helper names
// keep the switch above readable.
func setInfoRespBody() []byte { b := make([]byte, 2); set16(b, 0, 2); return b }
func setInfoResp() []byte     { return setInfoRespBody() }

func setBasic(obj *object, data []byte) uint32 {
	if len(data) < 36 {
		return statusInvalidParameter
	}
	_, aOK := fromFiletime(le64(data[8:]))
	atime, _ := fromFiletime(le64(data[8:]))
	mtime, mOK := fromFiletime(le64(data[16:]))
	if aOK || mOK {
		if !aOK {
			atime = time.Now()
		}
		if !mOK {
			mtime = time.Now()
		}
		if err := os.Chtimes(obj.full, atime, mtime); err != nil {
			return osErr(err)
		}
	}
	attr := le32(data[32:])
	if attr != 0 {
		mode := os.FileMode(0o644)
		if attr&0x01 != 0 { // READONLY
			mode = 0o444
		}
		_ = os.Chmod(obj.full, mode) // best effort
	}
	return statusSuccess
}

func (c *conn) setRename(t *tree, obj *object, data []byte) uint32 {
	if len(data) < 20 {
		return statusInvalidParameter
	}
	replace := data[0] != 0
	nameLen := int(le32(data[16:]))
	raw, ok := field(data, 20, nameLen)
	if !ok {
		return statusInvalidParameter
	}
	newName := utf16Decode(raw)
	newFull, err := t.share.resolve(newName)
	if err != nil {
		return statusObjectNameInvalid
	}
	if !replace {
		if _, err := os.Stat(newFull); err == nil {
			return statusObjectNameCollision
		}
	}
	// Close, rename, leave closed; the client re-opens by the new name.
	obj.file.Close()
	if err := os.Rename(obj.full, newFull); err != nil {
		return osErr(err)
	}
	obj.full = newFull
	obj.path = strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(newName, `\`, "/")), "/")
	if f, err := os.Open(newFull); err == nil {
		obj.file = f
	}
	return statusSuccess
}
