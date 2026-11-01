package lang

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// WebSocket is a small RFC 6455 client. It implements Widget so on ws.message:
// works exactly like GUI events.
type WebSocket struct {
	interp *Interp
	conn   net.Conn
	br     *bufio.Reader

	url      string
	protocol string
	origin   string

	mu       sync.Mutex
	writeMu  sync.Mutex
	handlers map[string][]func() error
	closed   bool
	lastErr  string

	lastMessage string
	lastType    string
	lastBinary  []byte
	count       int

	done     chan struct{}
	msgs     chan wsMessage
	closeOne sync.Once
}

type wsMessage struct {
	typ  string
	data []byte
}

func registerWebSocket(i *Interp) {
	i.RegisterNative("ws_connect", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		return wsConnect(i, args, kwargs)
	})
}

func wsConnect(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("ws_connect(url, ...) 需要 1 个参数")
	}
	raw, ok := asString(args[0])
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("ws_connect(): url 必须是非空字符串")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("ws_connect: %v", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "ws" && scheme != "wss" {
		return nil, fmt.Errorf("ws_connect: 只支持 ws:// 或 wss://")
	}
	timeout := 15.0
	if v, ok := kwargs["timeout"]; ok {
		f, ok := asNumber(v)
		if !ok || f <= 0 {
			return nil, fmt.Errorf("timeout 必须是正数（秒）")
		}
		timeout = f
	}
	headers, err := headerMap(kwargs["headers"])
	if err != nil {
		return nil, err
	}
	protocols := ""
	if v, ok := kwargs["protocols"]; ok {
		switch p := v.(type) {
		case string:
			protocols = p
		case *List:
			items := make([]string, 0, len(p.Items))
			for _, item := range p.Items {
				items = append(items, ToString(item))
			}
			protocols = strings.Join(items, ", ")
		}
	}
	origin := ""
	if v, ok := kwargs["origin"]; ok {
		origin, _ = asString(v)
	}
	insecure := stdBool(kwargs, "insecure", false)

	host := u.Host
	if u.Port() == "" {
		if scheme == "wss" {
			host = net.JoinHostPort(u.Hostname(), "443")
		} else {
			host = net.JoinHostPort(u.Hostname(), "80")
		}
	}
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("ws_connect: 随机数: %v", err)
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)

	var conn net.Conn
	var br *bufio.Reader
	err = i.Blocking(func() error {
		dialer := net.Dialer{Timeout: time.Duration(timeout * float64(time.Second))}
		var c net.Conn
		var err error
		if scheme == "wss" {
			td := &tls.Dialer{NetDialer: &dialer, Config: &tls.Config{ServerName: u.Hostname(), InsecureSkipVerify: insecure}}
			c, err = td.Dial("tcp", host)
		} else {
			c, err = dialer.Dial("tcp", host)
		}
		if err != nil {
			return err
		}
		conn = c
		br = bufio.NewReader(c)
		var b strings.Builder
		fmt.Fprintf(&b, "GET %s HTTP/1.1\r\n", path)
		fmt.Fprintf(&b, "Host: %s\r\n", u.Host)
		b.WriteString("Upgrade: websocket\r\n")
		b.WriteString("Connection: Upgrade\r\n")
		fmt.Fprintf(&b, "Sec-WebSocket-Key: %s\r\n", key)
		b.WriteString("Sec-WebSocket-Version: 13\r\n")
		if protocols != "" {
			fmt.Fprintf(&b, "Sec-WebSocket-Protocol: %s\r\n", protocols)
		}
		if origin != "" {
			fmt.Fprintf(&b, "Origin: %s\r\n", origin)
		}
		for k, v := range headers {
			fmt.Fprintf(&b, "%s: %s\r\n", k, v)
		}
		b.WriteString("\r\n")
		if _, err := io.WriteString(c, b.String()); err != nil {
			c.Close()
			return err
		}
		req := &http.Request{Method: "GET", URL: u, Host: u.Host, Header: http.Header{}}
		resp, err := http.ReadResponse(br, req)
		if err != nil {
			c.Close()
			return err
		}
		if resp.StatusCode != http.StatusSwitchingProtocols {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			c.Close()
			return fmt.Errorf("WebSocket 握手失败: %s %s", resp.Status, strings.TrimSpace(string(body)))
		}
		if !strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") ||
			!headerContains(resp.Header.Get("Connection"), "upgrade") {
			c.Close()
			return fmt.Errorf("WebSocket 握手失败: 响应缺少 Upgrade/Connection")
		}
		expect := base64.StdEncoding.EncodeToString(sha1Sum(key + wsGUID))
		if got := resp.Header.Get("Sec-WebSocket-Accept"); got != expect {
			c.Close()
			return fmt.Errorf("WebSocket 握手失败: Sec-WebSocket-Accept 不正确")
		}
		return nil
	})
	if err != nil {
		if conn != nil {
			conn.Close()
		}
		if ctx := i.TaskContext(); ctx != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("WebSocket 已取消")
		}
		return nil, fmt.Errorf("ws_connect: %v", err)
	}
	w := &WebSocket{
		interp: i, conn: conn, br: br, url: raw, protocol: protocols,
		handlers: map[string][]func() error{}, done: make(chan struct{}),
		msgs: make(chan wsMessage, 64),
	}
	go w.readLoop()
	return w, nil
}

func sha1Sum(s string) []byte {
	h := sha1.New()
	h.Write([]byte(s))
	return h.Sum(nil)
}

func headerContains(v, want string) bool {
	for _, part := range strings.Split(v, ",") {
		if strings.EqualFold(strings.TrimSpace(part), want) {
			return true
		}
	}
	return false
}

func (w *WebSocket) readLoop() {
	var frag []byte
	var fragOp byte
	for {
		op, fin, payload, err := w.readFrame()
		if err != nil {
			w.fail(err)
			return
		}
		switch op {
		case 0:
			if len(frag) == 0 {
				w.fail(errors.New("意外的 WebSocket continuation 帧"))
				return
			}
			frag = append(frag, payload...)
			if fin {
				w.dispatch(fragOp, frag)
				frag, fragOp = nil, 0
			}
		case 1, 2:
			if !fin {
				fragOp = op
				frag = append([]byte(nil), payload...)
				continue
			}
			w.dispatch(op, payload)
		case 8:
			w.closeWith("", false)
			return
		case 9:
			_ = w.writeFrame(10, payload)
		case 10:
			// pong
		default:
			w.fail(fmt.Errorf("未知 WebSocket 操作码 %d", op))
			return
		}
	}
}

func (w *WebSocket) dispatch(op byte, data []byte) {
	w.mu.Lock()
	w.count++
	w.lastBinary = append(w.lastBinary[:0], data...)
	if op == 1 {
		w.lastMessage = string(data)
		w.lastType = "text"
	} else {
		w.lastMessage = string(data)
		w.lastType = "binary"
	}
	w.mu.Unlock()
	select {
	case w.msgs <- wsMessage{typ: w.lastType, data: append([]byte(nil), data...)}:
	default:
	}
	w.emit("message")
}

func (w *WebSocket) fail(err error) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	w.lastErr = err.Error()
	w.mu.Unlock()
	w.conn.Close()
	w.closeOne.Do(func() { close(w.done) })
	w.emit("error")
	w.emit("close")
}

func (w *WebSocket) emit(name string) {
	w.mu.Lock()
	handlers := append([]func() error(nil), w.handlers[name]...)
	w.mu.Unlock()
	for _, fn := range handlers {
		h := fn
		w.interp.Dispatch(func() {
			if err := h(); err != nil {
				w.mu.Lock()
				w.lastErr = err.Error()
				w.mu.Unlock()
			}
		})
	}
}

// Kind implements Widget.
func (w *WebSocket) Kind() string { return "websocket" }

// Listen implements Widget.
func (w *WebSocket) Listen(event string, fn func() error) error {
	switch event {
	case "open", "message", "close", "error":
	default:
		return fmt.Errorf("websocket 不支持事件 %q", event)
	}
	w.mu.Lock()
	w.handlers[event] = append(w.handlers[event], fn)
	connected := !w.closed
	w.mu.Unlock()
	if event == "open" && connected {
		w.interp.Dispatch(func() {
			if err := fn(); err != nil {
				w.mu.Lock()
				w.lastErr = err.Error()
				w.mu.Unlock()
			}
		})
	}
	return nil
}

// GetProp implements Widget.
func (w *WebSocket) GetProp(name string) (Value, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	switch name {
	case "url":
		return w.url, true
	case "protocol", "protocols":
		return w.protocol, true
	case "connected":
		return !w.closed, true
	case "closed":
		return w.closed, true
	case "message", "text", "data":
		if w.lastType == "binary" {
			return string(w.lastBinary), true
		}
		return w.lastMessage, true
	case "type":
		return w.lastType, true
	case "count":
		return float64(w.count), true
	case "last_error":
		return w.lastErr, true
	}
	return nil, false
}

// SetProp implements Widget. WebSocket properties are read-only.
func (w *WebSocket) SetProp(name string, value Value) error {
	return fmt.Errorf("websocket 属性 %q 是只读的", name)
}

// CallMethod implements Widget.
func (w *WebSocket) CallMethod(name string, args []Value, kwargs map[string]Value) (Value, bool, error) {
	switch name {
	case "send", "send_text":
		if len(args) != 1 {
			return nil, true, fmt.Errorf("send(文本) 需要 1 个参数")
		}
		if err := w.writeFrame(1, []byte(ToString(args[0]))); err != nil {
			return nil, true, err
		}
		return nil, true, nil
	case "send_binary":
		if len(args) != 1 {
			return nil, true, fmt.Errorf("send_binary(数据) 需要 1 个参数")
		}
		if err := w.writeFrame(2, []byte(ToString(args[0]))); err != nil {
			return nil, true, err
		}
		return nil, true, nil
	case "recv", "receive":
		timeout := 5.0
		if len(args) > 1 {
			return nil, true, fmt.Errorf("recv(timeout=5) 接受 0 到 1 个参数")
		}
		if len(args) == 1 {
			f, ok := asNumber(args[0])
			if !ok || f < 0 {
				return nil, true, fmt.Errorf("recv(): timeout 必须是非负数")
			}
			timeout = f
		}
		return w.recv(timeout)
	case "ping":
		if err := w.writeFrame(9, nil); err != nil {
			return nil, true, err
		}
		return nil, true, nil
	case "close":
		w.closeWith("", true)
		return nil, true, nil
	}
	return nil, false, nil
}

func (w *WebSocket) recv(timeout float64) (Value, bool, error) {
	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(time.Duration(timeout * float64(time.Second)))
		defer t.Stop()
		timer = t.C
	}
	select {
	case msg := <-w.msgs:
		return string(msg.data), true, nil
	case <-w.done:
		return nil, true, fmt.Errorf("WebSocket 已关闭")
	case <-timer:
		return nil, true, fmt.Errorf("recv 超时")
	}
}

func (w *WebSocket) writeFrame(op byte, data []byte) error {
	w.mu.Lock()
	closed := w.closed
	w.mu.Unlock()
	if closed {
		return fmt.Errorf("WebSocket 已关闭")
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	header := make([]byte, 0, 14)
	header = append(header, 0x80|op)
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	n := len(data)
	switch {
	case n < 126:
		header = append(header, 0x80|byte(n))
	case n <= 65535:
		header = append(header, 0x80|126, 0, 0)
		binary.BigEndian.PutUint16(header[len(header)-2:], uint16(n))
	default:
		header = append(header, 0x80|127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[len(header)-8:], uint64(n))
	}
	header = append(header, mask...)
	masked := make([]byte, n)
	for i := range data {
		masked[i] = data[i] ^ mask[i%4]
	}
	header = append(header, masked...)
	_, err := w.conn.Write(header)
	return err
}

func (w *WebSocket) readFrame() (byte, bool, []byte, error) {
	var head [2]byte
	if _, err := io.ReadFull(w.br, head[:]); err != nil {
		return 0, false, nil, err
	}
	fin := head[0]&0x80 != 0
	if head[0]&0x70 != 0 {
		return 0, false, nil, fmt.Errorf("WebSocket 帧使用了不支持的 RSV 位")
	}
	op := head[0] & 0x0F
	masked := head[1]&0x80 != 0
	n := uint64(head[1] & 0x7F)
	switch n {
	case 126:
		var b [2]byte
		if _, err := io.ReadFull(w.br, b[:]); err != nil {
			return 0, false, nil, err
		}
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err := io.ReadFull(w.br, b[:]); err != nil {
			return 0, false, nil, err
		}
		n = binary.BigEndian.Uint64(b[:])
	}
	if n > 32<<20 {
		return 0, false, nil, fmt.Errorf("WebSocket 帧过大（%d 字节）", n)
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(w.br, mask[:]); err != nil {
			return 0, false, nil, err
		}
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(w.br, data); err != nil {
		return 0, false, nil, err
	}
	if masked {
		for i := range data {
			data[i] ^= mask[i%4]
		}
	}
	if op >= 8 {
		if !fin || n > 125 {
			return 0, false, nil, fmt.Errorf("WebSocket 控制帧非法")
		}
	}
	return op, fin, data, nil
}

func (w *WebSocket) closeWith(reason string, send bool) {
	w.closeOne.Do(func() {
		payload := make([]byte, 2+len(reason))
		binary.BigEndian.PutUint16(payload, 1000)
		copy(payload[2:], reason)
		if send {
			_ = w.writeFrame(8, payload)
		}
		w.mu.Lock()
		w.closed = true
		if reason != "" {
			w.lastErr = reason
		}
		w.mu.Unlock()
		w.conn.Close()
		close(w.done)
		w.emit("close")
	})
}

// Describe implements Value formatting.
func (w *WebSocket) Describe() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	state := "已连接"
	if w.closed {
		state = "已关闭"
	}
	return "<websocket " + state + ">"
}
