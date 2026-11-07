package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"agui/internal/lexer"
	"agui/internal/parser"
)

// lspServer is a deliberately small Language Server: full-document sync,
// syntax diagnostics and keyword/product completion. It has no dependencies.
func runLSP() error {
	in := bufio.NewReader(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	docs := map[string]string{}
	send := func(v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Content-Length: %d\r\n\r\n", len(b)); err != nil {
			return err
		}
		if _, err := out.Write(b); err != nil {
			return err
		}
		return out.Flush()
	}
	respond := func(id any, result any) error {
		return send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	for {
		length := 0
		for {
			line, err := in.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				fmt.Sscanf(strings.TrimSpace(line[len("content-length:"):]), "%d", &length)
			}
		}
		if length <= 0 {
			continue
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(in, buf); err != nil {
			return err
		}
		var msg struct {
			ID     any             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(buf, &msg); err != nil {
			continue
		}
		switch msg.Method {
		case "initialize":
			_ = respond(msg.ID, map[string]any{
				"capabilities": map[string]any{
					"textDocumentSync": 1,
					"completionProvider": map[string]any{
						"triggerCharacters": []string{".", "(", " "},
					},
				},
				"serverInfo": map[string]any{"name": "agui-lsp", "version": version},
			})
		case "initialized":
			// notification, no reply
		case "shutdown":
			_ = respond(msg.ID, nil)
		case "exit":
			return nil
		case "textDocument/didOpen", "textDocument/didChange", "textDocument/didSave":
			var p struct {
				TextDocument struct {
					URI  string `json:"uri"`
					Text string `json:"text"`
				} `json:"textDocument"`
				ContentChanges []struct {
					Text string `json:"text"`
				} `json:"contentChanges"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			text := p.TextDocument.Text
			if len(p.ContentChanges) > 0 {
				text = p.ContentChanges[len(p.ContentChanges)-1].Text
			}
			docs[p.TextDocument.URI] = text
			diags := diagnosticsFor(text)
			_ = send(map[string]any{
				"jsonrpc": "2.0",
				"method":  "textDocument/publishDiagnostics",
				"params": map[string]any{
					"uri":         p.TextDocument.URI,
					"diagnostics": diags,
				},
			})
		case "textDocument/didClose":
			var p struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			delete(docs, p.TextDocument.URI)
			_ = send(map[string]any{
				"jsonrpc": "2.0",
				"method":  "textDocument/publishDiagnostics",
				"params":  map[string]any{"uri": p.TextDocument.URI, "diagnostics": []any{}},
			})
		case "textDocument/completion":
			items := []map[string]any{}
			for _, k := range lspKeywords() {
				items = append(items, map[string]any{"label": k, "kind": 14})
			}
			_ = respond(msg.ID, map[string]any{"isIncomplete": false, "items": items})
		case "workspace/symbol":
			_ = respond(msg.ID, []any{})
		default:
			if msg.ID != nil {
				_ = respond(msg.ID, nil)
			}
		}
	}
}

func diagnosticsFor(src string) []map[string]any {
	toks, err := lexer.Lex(src)
	if err != nil {
		return []map[string]any{diagnostic(err, 1)}
	}
	if _, err := parser.Parse(toks); err != nil {
		return []map[string]any{diagnostic(err, 1)}
	}
	return []map[string]any{}
}

func diagnostic(err error, fallbackLine int) map[string]any {
	line, col := fallbackLine, 1
	msg := err.Error()
	if pe, ok := err.(*parser.Error); ok {
		line = pe.Line
		col = pe.Col
		msg = pe.Msg
	}
	return map[string]any{
		"range": map[string]any{
			"start": map[string]any{"line": maxInt(line-1, 0), "character": maxInt(col-1, 0)},
			"end":   map[string]any{"line": maxInt(line-1, 0), "character": maxInt(col, 0)},
		},
		"severity": 1,
		"source":   "agui",
		"message":  msg,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func lspKeywords() []string {
	return []string{
		"func", "return", "if", "elif", "else", "for", "in", "while", "break", "continue",
		"on", "import", "try", "catch", "finally", "raise", "class", "self", "async", "and", "or", "not",
		"true", "false", "null", "window", "panel", "label", "button", "edit", "checkbox",
		"progress", "image", "canvas", "menu", "table", "tree", "print", "str", "num", "len",
		"range", "list", "dict", "append", "format", "after", "every", "save_json", "load_json",
		"game_loop", "camera", "camera_shake", "key_down", "action_pressed", "aabb", "ray_rect",
	}
}
