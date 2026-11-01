package lang_test

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agui/internal/lang"
)

func TestWebSocketMessageEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "upgrade required", http.StatusBadRequest)
			return
		}
		key := r.Header.Get("Sec-WebSocket-Key")
		sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		accept := base64.StdEncoding.EncodeToString(sum[:])
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("hijack unsupported")
			return
		}
		conn, rw, err := hj.Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
		if err := rw.Flush(); err != nil {
			return
		}
		op, payload, err := readClientFrame(rw.Reader)
		if err != nil || op != 1 {
			return
		}
		echo := append([]byte("echo:"), payload...)
		writeServerFrame(rw.Writer, 1, echo)
		_ = rw.Flush()
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	src := fmt.Sprintf(`
ws = ws_connect("ws://%s", timeout=3)
on ws.message:
    print("got", ws.message, ws.type)
    ws.close()
ws.send("hello")
test_yield(500)
print("done", ws.closed, ws.count)
`, host)
	out, _ := runSource(t, src, func(i *lang.Interp) {
		i.RegisterNative("test_yield", func(i *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
			ms := 100.0
			if len(args) > 0 {
				if f, ok := args[0].(float64); ok {
					ms = f
				}
			}
			depth := i.Suspend()
			time.Sleep(time.Duration(ms) * time.Millisecond)
			i.Resume(depth)
			return nil, nil
		})
	})
	if !strings.Contains(out, "got echo:hello text") || !strings.Contains(out, "done true 1") {
		t.Fatalf("输出 = %q", out)
	}
}

func readClientFrame(r *bufio.Reader) (byte, []byte, error) {
	h := make([]byte, 2)
	if _, err := io.ReadFull(r, h); err != nil {
		return 0, nil, err
	}
	op := h[0] & 0x0f
	masked := h[1]&0x80 != 0
	n := int(h[1] & 0x7f)
	switch n {
	case 126:
		b := make([]byte, 2)
		if _, err := io.ReadFull(r, b); err != nil {
			return 0, nil, err
		}
		n = int(b[0])<<8 | int(b[1])
	case 127:
		b := make([]byte, 8)
		if _, err := io.ReadFull(r, b); err != nil {
			return 0, nil, err
		}
		n = 0
		for _, x := range b {
			n = n<<8 | int(x)
		}
	}
	var mask []byte
	if masked {
		mask = make([]byte, 4)
		if _, err := io.ReadFull(r, mask); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return op, payload, nil
}

func writeServerFrame(w io.Writer, op byte, payload []byte) {
	h := []byte{0x80 | op}
	switch {
	case len(payload) < 126:
		h = append(h, byte(len(payload)))
	case len(payload) < 65536:
		h = append(h, 126, byte(len(payload)>>8), byte(len(payload)))
	default:
		h = append(h, 127, 0, 0, 0, 0, byte(len(payload)>>24), byte(len(payload)>>16), byte(len(payload)>>8), byte(len(payload)))
	}
	_, _ = w.Write(h)
	_, _ = w.Write(payload)
}
