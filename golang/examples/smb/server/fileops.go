package server

import (
	"errors"
	"io"
	"os"
)

// CreateOptions bits (MS-SMB2 2.2.13).
const (
	optDirectoryFile    uint32 = 0x00000001
	optNonDirectoryFile uint32 = 0x00000040
	optDeleteOnClose    uint32 = 0x00001000
)

var allFF = [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// resolveFID reads a 16-byte FileId at off; a compound "use previous" id
// (all 0xFF) is replaced with lastFID.
func resolveFID(msg []byte, off int, lastFID *[16]byte) (uint64, bool) {
	raw, ok := field(msg, off, 16)
	if !ok {
		return 0, false
	}
	var id [16]byte
	copy(id[:], raw)
	if id == allFF {
		id = *lastFID
	}
	return le64(id[:]), true
}

func encodeFID(fid uint64) [16]byte {
	var id [16]byte
	set64(id[:], 0, fid)
	return id
}

func (c *conn) openFor(h *header, fid uint64) *object {
	t, st := c.treeFor(h)
	if st != statusSuccess {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return t.opens[fid]
}

func (c *conn) doCreate(msg []byte, h *header, lastFID *[16]byte) (uint32, []byte) {
	t, st := c.treeFor(h)
	if st != statusSuccess {
		return st, errorBody()
	}
	body := msg[headerSize:]
	if len(body) < 57 {
		return statusInvalidParameter, errorBody()
	}
	disposition := le32(body[36:])
	options := le32(body[40:])
	nameOff := int(le16(body[44:]))
	nameLen := int(le16(body[46:]))

	name := ""
	if nameLen > 0 {
		raw, ok := field(msg, nameOff, nameLen)
		if !ok {
			return statusInvalidParameter, errorBody()
		}
		name = utf16Decode(raw)
	}

	wantDir := options&optDirectoryFile != 0
	deleteOnClose := options&optDeleteOnClose != 0
	obj, st := t.share.open(name, disposition, wantDir, deleteOnClose)
	if st != statusSuccess {
		return st, errorBody()
	}
	fi, st2 := obj.stat()
	if st2 != statusSuccess {
		obj.close()
		return st2, errorBody()
	}

	c.mu.Lock()
	t.nextFID++
	fid := t.nextFID
	t.opens[fid] = obj
	c.mu.Unlock()
	*lastFID = encodeFID(fid)

	action := uint32(1) // FILE_OPENED
	switch disposition {
	case dispCreate:
		action = 2 // FILE_CREATED
	case dispSupersede:
		action = 0 // FILE_SUPERSEDED
	case dispOverwrite, dispOverwriteIf:
		action = 3 // FILE_OVERWRITTEN
	case dispOpenIf:
		if fi.Size() == 0 { // best-effort: created vs opened
			action = 1
		}
	}

	b := make([]byte, 88)
	set16(b, 0, 89)
	b[2] = 0 // OplockLevel: none
	set32(b, 4, action)
	putTime(b, 8, modTime(fi))
	putTime(b, 16, modTime(fi))
	putTime(b, 24, modTime(fi))
	putTime(b, 32, modTime(fi))
	size := uint64(fi.Size())
	set64(b, 40, size) // AllocationSize
	set64(b, 48, size) // EndOfFile
	set32(b, 56, dosAttr(fi))
	copy(b[64:80], lastFID[:])
	return statusSuccess, b
}

func (c *conn) doClose(msg []byte, h *header, lastFID *[16]byte) (uint32, []byte) {
	t, st := c.treeFor(h)
	if st != statusSuccess {
		return st, errorBody()
	}
	body := msg[headerSize:]
	if len(body) < 24 {
		return statusInvalidParameter, errorBody()
	}
	flags := le16(body[2:])
	fid, ok := resolveFID(msg, headerSize+8, lastFID)
	if !ok {
		return statusInvalidParameter, errorBody()
	}

	c.mu.Lock()
	obj := t.opens[fid]
	delete(t.opens, fid)
	c.mu.Unlock()
	if obj == nil {
		return statusInvalidHandle, errorBody()
	}

	b := make([]byte, 60)
	set16(b, 0, 60)
	if flags&0x0001 != 0 { // POSTQUERY_ATTRIB
		if fi, err := os.Stat(obj.full); err == nil {
			set16(b, 2, 0x0001)
			putTime(b, 8, fi.ModTime())
			putTime(b, 16, fi.ModTime())
			putTime(b, 24, fi.ModTime())
			putTime(b, 32, fi.ModTime())
			set64(b, 40, uint64(fi.Size()))
			set64(b, 48, uint64(fi.Size()))
			set32(b, 56, dosAttr(fi))
		}
	}
	obj.close()
	return statusSuccess, b
}

func (c *conn) doRead(msg []byte, h *header, lastFID *[16]byte) (uint32, []byte) {
	t, st := c.treeFor(h)
	if st != statusSuccess {
		return st, errorBody()
	}
	body := msg[headerSize:]
	if len(body) < 48 {
		return statusInvalidParameter, errorBody()
	}
	length := le32(body[4:])
	offset := le64(body[8:])
	fid, ok := resolveFID(msg, headerSize+16, lastFID)
	if !ok {
		return statusInvalidParameter, errorBody()
	}
	c.mu.Lock()
	obj := t.opens[fid]
	c.mu.Unlock()
	if obj == nil {
		return statusInvalidHandle, errorBody()
	}
	if obj.isDir {
		return statusInvalidDeviceRequest, errorBody()
	}
	if length > c.srv.maxRead {
		length = c.srv.maxRead
	}

	buf := make([]byte, length)
	n, err := obj.file.ReadAt(buf, int64(offset))
	if n == 0 && errors.Is(err, io.EOF) {
		return statusEndOfFile, errorBody()
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return osErr(err), errorBody()
	}

	b := make([]byte, 16)
	set16(b, 0, 17)
	b[2] = 80 // DataOffset = header(64)+fixed(16)
	set32(b, 4, uint32(n))
	return statusSuccess, append(b, buf[:n]...)
}

func (c *conn) doWrite(msg []byte, h *header, lastFID *[16]byte) (uint32, []byte) {
	t, st := c.treeFor(h)
	if st != statusSuccess {
		return st, errorBody()
	}
	body := msg[headerSize:]
	if len(body) < 48 {
		return statusInvalidParameter, errorBody()
	}
	dataOff := int(le16(body[2:]))
	length := le32(body[4:])
	offset := le64(body[8:])
	fid, ok := resolveFID(msg, headerSize+16, lastFID)
	if !ok {
		return statusInvalidParameter, errorBody()
	}
	data, ok := field(msg, dataOff, int(length))
	if !ok {
		return statusInvalidParameter, errorBody()
	}
	c.mu.Lock()
	obj := t.opens[fid]
	c.mu.Unlock()
	if obj == nil {
		return statusInvalidHandle, errorBody()
	}
	if obj.share.ReadOnly {
		return statusMediaWriteProtected, errorBody()
	}
	n, err := obj.file.WriteAt(data, int64(offset))
	if err != nil {
		return osErr(err), errorBody()
	}

	b := make([]byte, 16)
	set16(b, 0, 17)
	set32(b, 4, uint32(n)) // Count
	return statusSuccess, b
}

func (c *conn) doFlush(msg []byte, h *header, lastFID *[16]byte) (uint32, []byte) {
	t, st := c.treeFor(h)
	if st != statusSuccess {
		return st, errorBody()
	}
	fid, ok := resolveFID(msg, headerSize+8, lastFID)
	if !ok {
		return statusInvalidParameter, errorBody()
	}
	c.mu.Lock()
	obj := t.opens[fid]
	c.mu.Unlock()
	if obj == nil {
		return statusInvalidHandle, errorBody()
	}
	if !obj.isDir {
		if err := obj.file.Sync(); err != nil {
			return osErr(err), errorBody()
		}
	}
	return statusSuccess, []byte{4, 0, 0, 0}
}

// FSCTL codes handled explicitly.
const (
	fsctlValidateNegotiateInfo uint32 = 0x00140204
	fsctlQueryNetworkInterface uint32 = 0x001401FC
)

func (c *conn) doIoctl(msg []byte, h *header, lastFID *[16]byte) (uint32, []byte) {
	body := msg[headerSize:]
	if len(body) < 56 {
		return statusInvalidParameter, errorBody()
	}
	ctl := le32(body[4:])
	switch ctl {
	case fsctlValidateNegotiateInfo:
		// Confirm the earlier NEGOTIATE so the client won't suspect a
		// downgrade attack. Echo our capabilities, GUID, mode and dialect.
		out := make([]byte, 24)
		set32(out, 0, 0x00000004) // Capabilities: LARGE_MTU
		copy(out[4:20], serverGUID[:])
		set16(out, 20, 0x0001) // SecurityMode: signing enabled
		set16(out, 22, c.dialect)
		return statusSuccess, ioctlResponse(ctl, out)
	case fsctlQueryNetworkInterface:
		return statusSuccess, ioctlResponse(ctl, nil)
	default:
		return statusInvalidDeviceRequest, errorBody()
	}
}

// ioctlResponse builds an IOCTL response carrying out as the output buffer.
func ioctlResponse(ctl uint32, out []byte) []byte {
	b := make([]byte, 48)
	set16(b, 0, 49)
	set32(b, 4, ctl)
	// FileId at 8..24 left zero (valid for FSCTLs not bound to a handle).
	set32(b, 24, 112) // InputOffset = header(64)+fixed(48)
	set32(b, 32, 112) // OutputOffset
	set32(b, 36, uint32(len(out)))
	return append(b, out...)
}
