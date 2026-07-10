package quicken

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWSAcceptRFC6455Vector(t *testing.T) {
	// The nonce from RFC 6455 section 1.3. The expected value below is the
	// base64(SHA1(key + magic GUID)) computed independently with openssl,
	// Python hashlib, and Node crypto; it does not match the literal digest
	// string quoted in the RFC prose, which appears to be a transcription
	// error in that document.
	got := wsAccept("dGhlIHNhbXBsZSBub25jZQ==")
	want := "NM9PMnjLEySD056c6McrYhxPzQc="
	if got != want {
		t.Fatalf("wsAccept = %q, want %q", got, want)
	}
}

// writeClientFrame writes a single masked frame the way a browser would, for
// tests only. mask is a fixed 4-byte key so the test is deterministic.
func writeClientFrame(conn net.Conn, opcode byte, payload []byte) error {
	var hdr []byte
	hdr = append(hdr, 0x80|opcode) // FIN + opcode
	n := len(payload)
	switch {
	case n < 126:
		hdr = append(hdr, 0x80|byte(n))
	case n < 1<<16:
		hdr = append(hdr, 0x80|126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(n))
		hdr = append(hdr, ext[:]...)
	default:
		hdr = append(hdr, 0x80|127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(n))
		hdr = append(hdr, ext[:]...)
	}
	key := []byte{0x12, 0x34, 0x56, 0x78}
	hdr = append(hdr, key...)
	masked := make([]byte, n)
	for i := 0; i < n; i++ {
		masked[i] = payload[i] ^ key[i%4]
	}
	if _, err := conn.Write(hdr); err != nil {
		return err
	}
	_, err := conn.Write(masked)
	return err
}

// writeClientFragment writes a single masked frame with an explicit FIN bit,
// for tests only, so fragmented messages can be exercised.
func writeClientFragment(conn net.Conn, opcode byte, payload []byte, fin bool) error {
	b0 := opcode
	if fin {
		b0 |= 0x80
	}
	key := []byte{0x12, 0x34, 0x56, 0x78}
	hdr := []byte{b0, 0x80 | byte(len(payload))}
	hdr = append(hdr, key...)
	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ key[i%4]
	}
	if _, err := conn.Write(hdr); err != nil {
		return err
	}
	_, err := conn.Write(masked)
	return err
}

func TestWSReadMessageUnmasksClientFrame(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		writeClientFrame(cli, opText, []byte("hello"))
	}()
	op, payload, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if op != opText || string(payload) != "hello" {
		t.Fatalf("got op=%x payload=%q", op, payload)
	}
}

func TestWSReadMessageReassemblesFragments(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		// "he" as text non-final, "llo" as continuation final.
		writeClientFragment(cli, opText, []byte("he"), false)
		writeClientFragment(cli, opCont, []byte("llo"), true)
	}()
	op, payload, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if op != opText || string(payload) != "hello" {
		t.Fatalf("got op=%x payload=%q", op, payload)
	}
}

func TestWSReadMessageAnswersPing(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		writeClientFrame(cli, opPing, []byte("x"))
		writeClientFrame(cli, opText, []byte("after"))
	}()
	// Drain the pong the server sends back.
	go func() { io.Copy(io.Discard, cli) }()
	op, payload, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if op != opText || string(payload) != "after" {
		t.Fatalf("got op=%x payload=%q", op, payload)
	}
}

func TestWSWriteTextIsUnmaskedServerFrame(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() { c.WriteText([]byte("hi")) }()
	buf := make([]byte, 4)
	if _, err := io.ReadFull(cli, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if buf[0] != (0x80|opText) || buf[1] != 2 { // FIN+text, len 2, MASK bit clear
		t.Fatalf("server frame header = % x", buf[:2])
	}
	if !bytes.Equal(buf[2:], []byte("hi")) {
		t.Fatalf("payload = % x", buf[2:])
	}
}

// writeUnmaskedClientFrame writes a single frame with the mask bit clear, for
// tests only, to exercise the reader's rejection of unmasked client frames
// (RFC 6455 section 5.1: a server MUST fail the connection on an unmasked
// frame from a client).
func writeUnmaskedClientFrame(conn net.Conn, opcode byte, payload []byte) error {
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
	if _, err := conn.Write(hdr); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}

// writeClientFrameFin writes a masked frame with an explicit FIN bit and full
// RFC6455 length encoding (including the 16-bit and 64-bit extensions), for
// tests only. writeClientFrame always sets FIN and writeClientFragment only
// supports payloads under 126 bytes, so this fills the gap for tests that
// need a non-final fragment larger than 125 bytes.
func writeClientFrameFin(conn net.Conn, opcode byte, payload []byte, fin bool) error {
	b0 := opcode
	if fin {
		b0 |= 0x80
	}
	var hdr []byte
	hdr = append(hdr, b0)
	n := len(payload)
	switch {
	case n < 126:
		hdr = append(hdr, 0x80|byte(n))
	case n < 1<<16:
		hdr = append(hdr, 0x80|126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(n))
		hdr = append(hdr, ext[:]...)
	default:
		hdr = append(hdr, 0x80|127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(n))
		hdr = append(hdr, ext[:]...)
	}
	key := []byte{0x12, 0x34, 0x56, 0x78}
	hdr = append(hdr, key...)
	masked := make([]byte, n)
	for i := 0; i < n; i++ {
		masked[i] = payload[i] ^ key[i%4]
	}
	if _, err := conn.Write(hdr); err != nil {
		return err
	}
	_, err := conn.Write(masked)
	return err
}

func TestWSReadMessageRejectsUnmaskedFrame(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		writeUnmaskedClientFrame(cli, opText, []byte("plain"))
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error for an unmasked client data frame")
	}
}

func TestWSReadFrameOversizedSingleFrameErrors(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		var hdr []byte
		hdr = append(hdr, 0x80|opBinary)  // FIN + binary
		hdr = append(hdr, 0x80|byte(127)) // masked, 64-bit extended length
		var ext [8]byte
		// A declared length far beyond maxMessageSize. readFrame must reject
		// this before ever calling make([]byte, n), so the peer never needs
		// to send the mask key or any payload bytes for the read to fail.
		binary.BigEndian.PutUint64(ext[:], uint64(1)<<62)
		hdr = append(hdr, ext[:]...)
		cli.Write(hdr)
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error for a declared frame length over maxMessageSize")
	}
}

func TestWSReadMessageOversizedReassemblyErrors(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	first := make([]byte, maxMessageSize) // exactly at the per-frame bound, allowed alone.
	go func() {
		writeClientFrameFin(cli, opBinary, first, false)
		writeClientFrameFin(cli, opCont, []byte("x"), true) // pushes the total 1 byte over the bound.
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error when the reassembled message exceeds maxMessageSize")
	}
}

func TestWSReadMessageOversizedViaRepeatedDataOpcodeErrors(t *testing.T) {
	// This codec does not require a continuation opcode after a non-final
	// data frame, so a second opText frame (rather than opCont) can also
	// drive the accumulation buffer over maxMessageSize; this exercises that
	// distinct size check, in the opText/opBinary case rather than opCont.
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	first := make([]byte, maxMessageSize) // exactly at the per-frame bound, allowed alone.
	go func() {
		writeClientFrameFin(cli, opText, first, false)
		writeClientFrameFin(cli, opText, []byte("x"), true) // pushes the total 1 byte over the bound.
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error when a repeated opText frame pushes the message over maxMessageSize")
	}
}

func TestWSReadFrameControlFrameOverMaxPayloadErrors(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		// FIN + ping, masked, 16-bit extended length declaring 126 bytes: over
		// the 125-byte control-frame cap. readFrame must reject this right
		// after the length is decoded, before requiring a mask key or
		// payload, so only the 4-byte header is sent.
		var hdr []byte
		hdr = append(hdr, 0x80|opPing)
		hdr = append(hdr, 0x80|126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], 126)
		hdr = append(hdr, ext[:]...)
		cli.Write(hdr)
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error for a control frame payload over 125 bytes")
	}
}

func TestWSReadFrameFragmentedControlFrameErrors(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		// FIN clear + ping opcode, masked, 1-byte declared length: control
		// frames must not be fragmented, so readFrame should reject this from
		// the 2-byte header alone, before requiring a mask key or payload.
		cli.Write([]byte{opPing, 0x80 | 1})
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error for a fragmented (FIN-clear) ping frame")
	}
}

func TestWSReadMessageCloseEchoWriteErrorPropagates(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		var p [2]byte
		binary.BigEndian.PutUint16(p[:], 1000)
		writeClientFrame(cli, opClose, p[:])
		cli.Close() // close the peer immediately so the server's close echo fails to write.
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error when the close echo write fails")
	}
}

func TestWSReadMessageHandles16BitExtendedLength(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 200) // >= 126, forces the 16-bit length extension.
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		writeClientFrame(cli, opBinary, payload)
	}()
	op, got, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if op != opBinary || !bytes.Equal(got, payload) {
		t.Fatalf("got op=%x len=%d, want op=%x len=%d", op, len(got), opBinary, len(payload))
	}
}

func TestWSReadMessageHandles64BitExtendedLength(t *testing.T) {
	payload := bytes.Repeat([]byte("b"), 1<<16+10) // >= 65536, forces the 64-bit length extension.
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		writeClientFrame(cli, opBinary, payload)
	}()
	op, got, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if op != opBinary || !bytes.Equal(got, payload) {
		t.Fatalf("got op=%x len=%d, want op=%x len=%d", op, len(got), opBinary, len(payload))
	}
}

func TestWSWriteTextUses16BitExtendedLength(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 200)
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() { c.WriteText(payload) }()
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(cli, hdr); err != nil {
		t.Fatalf("read header: %v", err)
	}
	if hdr[1] != 126 {
		t.Fatalf("length byte = %d, want 126", hdr[1])
	}
	if got := binary.BigEndian.Uint16(hdr[2:4]); got != uint16(len(payload)) {
		t.Fatalf("extended length = %d, want %d", got, len(payload))
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(cli, got); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("payload mismatch")
	}
}

func TestWSWriteTextUses64BitExtendedLength(t *testing.T) {
	payload := bytes.Repeat([]byte("b"), 1<<16+10)
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() { c.WriteText(payload) }()
	hdr := make([]byte, 10)
	if _, err := io.ReadFull(cli, hdr); err != nil {
		t.Fatalf("read header: %v", err)
	}
	if hdr[1] != 127 {
		t.Fatalf("length byte = %d, want 127", hdr[1])
	}
	if got := binary.BigEndian.Uint64(hdr[2:10]); got != uint64(len(payload)) {
		t.Fatalf("extended length = %d, want %d", got, len(payload))
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(cli, got); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("payload mismatch")
	}
}

func TestWSReadMessageHandlesCloseFrame(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4)
		if _, err := io.ReadFull(cli, buf); err != nil {
			return
		}
		if buf[0] != (0x80|opClose) || buf[1] != 2 {
			t.Errorf("close response header = % x, want FIN+close, len 2", buf[:2])
		}
	}()
	go func() {
		var p [2]byte
		binary.BigEndian.PutUint16(p[:], 1000)
		writeClientFrame(cli, opClose, p[:])
	}()
	_, _, err := c.ReadMessage()
	if err != io.EOF {
		t.Fatalf("ReadMessage on close = %v, want io.EOF", err)
	}
	<-done
}

func TestWSWriteCloseSendsCloseFrame(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() { c.WriteClose(1001) }()
	buf := make([]byte, 4)
	if _, err := io.ReadFull(cli, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if buf[0] != (0x80|opClose) || buf[1] != 2 {
		t.Fatalf("close frame header = % x", buf[:2])
	}
	if got := binary.BigEndian.Uint16(buf[2:4]); got != 1001 {
		t.Fatalf("close code = %d, want 1001", got)
	}
}

func TestWSCloseThenWriteErrors(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	cli.Close()
	if err := c.WriteText([]byte("x")); err == nil {
		t.Fatal("expected an error writing to a closed connection")
	}
}

func TestWSReadFrameTruncatedHeaderErrors(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		// Write one byte of a two-byte header, then close before completing it.
		cli.Write([]byte{0x80 | opText})
		cli.Close()
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error reading a truncated frame header")
	}
}

func TestWSReadFrameTruncated16BitExtendedLengthErrors(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		// Header announces a 16-bit extended length but the connection closes
		// before the 2 extension bytes arrive.
		cli.Write([]byte{0x80 | opText, 0x80 | 126})
		cli.Close()
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error reading a truncated 16-bit extended length")
	}
}

func TestWSReadFrameTruncated64BitExtendedLengthErrors(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		// Header announces a 64-bit extended length but the connection closes
		// before the 8 extension bytes arrive.
		cli.Write([]byte{0x80 | opText, 0x80 | 127})
		cli.Close()
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error reading a truncated 64-bit extended length")
	}
}

func TestWSReadFrameTruncatedMaskErrors(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		// Header + length complete and masked, but the connection closes
		// before the 4 mask-key bytes arrive.
		cli.Write([]byte{0x80 | opText, 0x80 | 5})
		cli.Close()
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error reading a truncated mask key")
	}
}

func TestWSReadFrameTruncatedPayloadErrors(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		// Header + length + mask complete, but the connection closes before
		// the 5 payload bytes arrive.
		cli.Write([]byte{0x80 | opText, 0x80 | 5, 0x12, 0x34, 0x56, 0x78})
		cli.Close()
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error reading a truncated payload")
	}
}

func TestWSReadMessageSkipsUnsolicitedPong(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		writeClientFrame(cli, opPong, []byte("x"))
		writeClientFrame(cli, opText, []byte("after"))
	}()
	op, payload, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if op != opText || string(payload) != "after" {
		t.Fatalf("got op=%x payload=%q", op, payload)
	}
}

func TestWSReadMessagePongWriteErrorPropagates(t *testing.T) {
	cli, srv := net.Pipe()
	c := &wsConn{conn: srv, br: newReader(srv)}
	go func() {
		writeClientFrame(cli, opPing, []byte("x"))
		cli.Close()
	}()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected an error when the pong response write fails")
	}
}

func validUpgradeRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	return req
}

func TestWSUpgradeRejectsNonWebsocketRequest(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := wsUpgrade(w, req); err == nil {
		t.Fatal("expected an error for a non-websocket request")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWSUpgradeRejectsMissingKey(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	if _, err := wsUpgrade(w, req); err == nil {
		t.Fatal("expected an error for a missing Sec-WebSocket-Key")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWSUpgradeRejectsNonHijacker(t *testing.T) {
	w := httptest.NewRecorder()
	if _, err := wsUpgrade(w, validUpgradeRequest()); err == nil {
		t.Fatal("expected an error when the ResponseWriter is not a Hijacker")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// fakeHijackWriter lets tests control what Hijack returns without going
// through a real HTTP server.
type fakeHijackWriter struct {
	http.ResponseWriter
	hijack func() (net.Conn, *bufio.ReadWriter, error)
}

func (f *fakeHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) { return f.hijack() }

func TestWSUpgradeHijackError(t *testing.T) {
	w := &fakeHijackWriter{
		ResponseWriter: httptest.NewRecorder(),
		hijack: func() (net.Conn, *bufio.ReadWriter, error) {
			return nil, nil, errors.New("hijack boom")
		},
	}
	if _, err := wsUpgrade(w, validUpgradeRequest()); err == nil {
		t.Fatal("expected an error when Hijack fails")
	}
}

func TestWSUpgradeWriteStringError(t *testing.T) {
	c1, c2 := net.Pipe()
	c2.Close()
	w := &fakeHijackWriter{
		ResponseWriter: httptest.NewRecorder(),
		hijack: func() (net.Conn, *bufio.ReadWriter, error) {
			// A tiny write buffer forces WriteString itself to flush to the
			// (already closed) peer and fail, rather than merely buffering.
			return c1, bufio.NewReadWriter(bufio.NewReader(c1), bufio.NewWriterSize(c1, 8)), nil
		},
	}
	if _, err := wsUpgrade(w, validUpgradeRequest()); err == nil {
		t.Fatal("expected an error when WriteString fails")
	}
}

func TestWSUpgradeFlushError(t *testing.T) {
	c1, c2 := net.Pipe()
	c2.Close()
	w := &fakeHijackWriter{
		ResponseWriter: httptest.NewRecorder(),
		hijack: func() (net.Conn, *bufio.ReadWriter, error) {
			return c1, bufio.NewReadWriter(bufio.NewReader(c1), bufio.NewWriter(c1)), nil
		},
	}
	if _, err := wsUpgrade(w, validUpgradeRequest()); err == nil {
		t.Fatal("expected an error when Flush fails")
	}
}

func TestWSUpgradeHappyPath(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := wsUpgrade(w, r)
			if err != nil {
				return
			}
			defer c.Close()
		}),
	}
	go server.Serve(ln)
	defer server.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	req := "GET / HTTP/1.1\r\n" +
		"Host: " + ln.Addr().String() + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write request: %v", err)
	}
	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read status line: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("status line = %q, want 101", status)
	}
	var gotAccept string
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read header line: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "sec-websocket-accept:") {
			gotAccept = strings.TrimSpace(line[len("sec-websocket-accept:"):])
		}
	}
	want := wsAccept("dGhlIHNhbXBsZSBub25jZQ==")
	if gotAccept != want {
		t.Fatalf("Sec-WebSocket-Accept = %q, want %q", gotAccept, want)
	}
}
