package quicken

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

const wsMagic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B39"

// maxMessageSize bounds both a single frame's declared payload length and the
// total size of a reassembled (possibly fragmented) application message. It
// guards against a malicious or buggy client declaring an unbounded 64-bit
// length, or driving memory exhaustion through an endless run of continuation
// frames.
const maxMessageSize = 8 << 20 // 8 MiB

const (
	opCont   byte = 0x0
	opText   byte = 0x1
	opBinary byte = 0x2
	opClose  byte = 0x8
	opPing   byte = 0x9
	opPong   byte = 0xA
)

// wsAccept computes the Sec-WebSocket-Accept response value for a client key.
func wsAccept(key string) string {
	h := sha1.New()
	io.WriteString(h, key+wsMagic)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

type wsConn struct {
	conn net.Conn
	br   *bufio.Reader
	wmu  sync.Mutex // serializes writeFrame against concurrent writers (auto-pong vs application writes)
}

func newReader(c net.Conn) *bufio.Reader { return bufio.NewReader(c) }

// wsUpgrade validates the WebSocket upgrade request, hijacks the connection,
// and completes the handshake. On any validation failure it writes a 400 and
// returns an error without hijacking.
func wsUpgrade(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		!strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		http.Error(w, "expected websocket upgrade", http.StatusBadRequest)
		return nil, errors.New("quicken: not a websocket upgrade")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return nil, errors.New("quicken: missing websocket key")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		return nil, errors.New("quicken: response writer is not a hijacker")
	}
	conn, brw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + wsAccept(key) + "\r\n\r\n"
	if _, err := brw.WriteString(resp); err != nil {
		conn.Close()
		return nil, err
	}
	if err := brw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}
	return &wsConn{conn: conn, br: brw.Reader}, nil
}

// readFrame reads one frame: its opcode, FIN bit, and unmasked payload.
func (c *wsConn) readFrame() (opcode byte, fin bool, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(c.br, h[:]); err != nil {
		return 0, false, nil, err
	}
	fin = h[0]&0x80 != 0
	opcode = h[0] & 0x0f
	masked := h[1]&0x80 != 0
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return 0, false, nil, err
		}
		n = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return 0, false, nil, err
		}
		n = binary.BigEndian.Uint64(ext[:])
	}
	if n > maxMessageSize {
		return 0, false, nil, errors.New("quicken: frame too large")
	}
	isControl := opcode == opClose || opcode == opPing || opcode == opPong
	if isControl && (n > 125 || !fin) {
		return 0, false, nil, errors.New("quicken: invalid control frame")
	}
	if !masked && n > 0 {
		return 0, false, nil, errors.New("quicken: client frame not masked")
	}
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(c.br, mask[:]); err != nil {
			return 0, false, nil, err
		}
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(c.br, payload); err != nil {
		return 0, false, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, fin, payload, nil
}

// ReadMessage returns the next application message, reassembling fragments and
// transparently answering ping and close control frames.
func (c *wsConn) ReadMessage() (byte, []byte, error) {
	var buf []byte
	var msgOp byte
	for {
		op, fin, payload, err := c.readFrame()
		if err != nil {
			return 0, nil, err
		}
		switch op {
		case opPing:
			if err := c.writeFrame(opPong, payload); err != nil {
				return 0, nil, err
			}
			continue
		case opPong:
			continue
		case opClose:
			if werr := c.WriteClose(1000); werr != nil {
				return 0, nil, werr
			}
			return 0, nil, io.EOF
		case opText, opBinary:
			if len(buf)+len(payload) > maxMessageSize {
				return 0, nil, errors.New("quicken: message too large")
			}
			msgOp = op
			buf = append(buf, payload...)
		case opCont:
			if len(buf)+len(payload) > maxMessageSize {
				return 0, nil, errors.New("quicken: message too large")
			}
			buf = append(buf, payload...)
		}
		if fin && (op == opText || op == opBinary || op == opCont) {
			return msgOp, buf, nil
		}
	}
}

func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	var hdr []byte
	hdr = append(hdr, 0x80|opcode)
	n := len(payload)
	switch {
	case n < 126:
		hdr = append(hdr, byte(n))
	case n < 1<<16:
		hdr = append(hdr, 126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(n))
		hdr = append(hdr, ext[:]...)
	default:
		hdr = append(hdr, 127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(n))
		hdr = append(hdr, ext[:]...)
	}
	if _, err := c.conn.Write(hdr); err != nil {
		return err
	}
	_, err := c.conn.Write(payload)
	return err
}

func (c *wsConn) WriteText(b []byte) error { return c.writeFrame(opText, b) }

func (c *wsConn) WriteClose(code uint16) error {
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], code)
	return c.writeFrame(opClose, p[:])
}

func (c *wsConn) Close() error { return c.conn.Close() }
