package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MaxFrame is the hard upper bound on a single message body.
const MaxFrame = 1 << 20 // 1 MiB

// WriteFrame writes a length-prefixed body to w.
func WriteFrame(w io.Writer, body []byte) error {
	var hdr [4]byte
	//nolint:gosec // len(body) fits in uint32: bodies this large cannot be allocated on supported platforms
	binary.BigEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	return nil
}

// ReadFrame reads one length-prefixed body from r. Bodies larger than maxBytes
// are rejected.
func ReadFrame(r io.Reader, maxBytes int) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint32(hdr[:]))
	if n > maxBytes {
		return nil, fmt.Errorf("frame too large: %d > %d", n, maxBytes)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
