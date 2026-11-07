package lang

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// registerStdlib installs the JSON, base64 and HTTP helper functions.
func registerStdlib(i *Interp) {
	reg := func(name string, fn func(*Interp, []Value, map[string]Value) (Value, error)) {
		i.RegisterNative(name, fn)
	}

	reg("json_encode", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("json_encode(值, pretty=false) 需要 1 个参数")
		}
		pretty := stdBool(kwargs, "pretty", false)
		if v, ok := kwargs["indent"]; ok {
			if n, ok := asInt(v); ok && n > 0 {
				pretty = true
			}
		}
		val, err := toJSONValue(args[0], 0)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if pretty {
			enc.SetIndent("", "  ")
		}
		if err := enc.Encode(val); err != nil {
			return nil, fmt.Errorf("json_encode: %v", err)
		}
		return strings.TrimSuffix(buf.String(), "\n"), nil
	})

	reg("json_decode", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("json_decode(字符串) 需要 1 个参数")
		}
		s, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("json_decode(): 参数必须是字符串")
		}
		dec := json.NewDecoder(strings.NewReader(s))
		dec.UseNumber()
		var raw any
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("json_decode: %v", err)
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			if err == nil {
				return nil, fmt.Errorf("json_decode: JSON 后面还有多余内容")
			}
			return nil, fmt.Errorf("json_decode: %v", err)
		}
		return fromJSONValue(raw), nil
	})

	reg("base64_encode", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("base64_encode(字符串, urlsafe=false) 接受 1 到 2 个参数")
		}
		s, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("base64_encode(): 参数必须是字符串")
		}
		urlsafe := stdBool(kwargs, "urlsafe", false) || stdBool(kwargs, "url", false)
		if len(args) == 2 {
			urlsafe = Truthy(args[1])
		}
		if urlsafe {
			return base64.RawURLEncoding.EncodeToString([]byte(s)), nil
		}
		return base64.StdEncoding.EncodeToString([]byte(s)), nil
	})

	reg("base64_decode", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("base64_decode(字符串, urlsafe=false) 接受 1 到 2 个参数")
		}
		s, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("base64_decode(): 参数必须是字符串")
		}
		urlsafe := stdBool(kwargs, "urlsafe", false) || stdBool(kwargs, "url", false)
		if len(args) == 2 {
			urlsafe = Truthy(args[1])
		}
		var (
			data []byte
			err  error
		)
		if urlsafe {
			data, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
		} else {
			data, err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				data, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
			}
		}
		if err != nil {
			return nil, fmt.Errorf("base64_decode: %v", err)
		}
		return string(data), nil
	})

	reg("http_get", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("http_get(url, headers=..., timeout=15) 需要 1 个位置参数")
		}
		return doHTTP(i, "GET", args[0], nil, kwargs)
	})

	reg("http_post", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("http_post(url, body=\"\", headers=..., timeout=15) 接受 1 到 2 个参数")
		}
		var body Value = ""
		if len(args) == 2 {
			body = args[1]
		}
		return doHTTP(i, "POST", args[0], body, kwargs)
	})

	reg("http_request", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("http_request(method, url, body=\"\", headers=..., timeout=15) 接受 2 到 3 个参数")
		}
		method, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("http_request(): method 必须是字符串")
		}
		var body Value = ""
		if len(args) == 3 {
			body = args[2]
		}
		return doHTTP(i, strings.ToUpper(method), args[1], body, kwargs)
	})
	registerHTTP(i)
	registerSQLite(i)
}

func stdBool(kwargs map[string]Value, name string, def bool) bool {
	if v, ok := kwargs[name]; ok {
		return Truthy(v)
	}
	return def
}

func doHTTP(i *Interp, method string, urlVal Value, body Value, kwargs map[string]Value) (Value, error) {
	rawURL, ok := asString(urlVal)
	if !ok || strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("%s: url 必须是非空字符串", strings.ToLower(method))
	}
	timeout := 15.0
	if v, ok := kwargs["timeout"]; ok {
		if f, ok := asNumber(v); ok && f > 0 {
			timeout = f
		} else {
			return nil, fmt.Errorf("timeout 必须是正数（秒）")
		}
	}
	bodyText := ""
	if body != nil {
		bodyText = ToString(body)
	}
	req, err := http.NewRequest(method, rawURL, strings.NewReader(bodyText))
	if err != nil {
		return nil, fmt.Errorf("%s: %v", strings.ToLower(method), err)
	}
	if ctx := i.TaskContext(); ctx != nil {
		req = req.WithContext(ctx)
	}
	headers, err := headerMap(kwargs["headers"])
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}
	client := &http.Client{Timeout: time.Duration(timeout * float64(time.Second))}
	var resp *http.Response
	var data []byte
	err = i.Blocking(func() error {
		var reqErr error
		resp, reqErr = client.Do(req)
		if reqErr != nil {
			return reqErr
		}
		defer resp.Body.Close()
		data, reqErr = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		return reqErr
	})
	if err != nil {
		if ctx := i.TaskContext(); ctx != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("%s 已取消", strings.ToLower(method))
		}
		return nil, fmt.Errorf("%s %s: %v", method, rawURL, err)
	}
	out := NewMap()
	out.Set("status", float64(resp.StatusCode))
	out.Set("ok", resp.StatusCode >= 200 && resp.StatusCode < 300)
	out.Set("body", string(data))
	h := NewMap()
	for k, vals := range resp.Header {
		h.Set(k, strings.Join(vals, ", "))
	}
	out.Set("headers", h)
	out.Set("url", resp.Request.URL.String())
	return out, nil
}

func headerMap(v Value) (map[string]string, error) {
	out := map[string]string{}
	if v == nil {
		return out, nil
	}
	m, ok := v.(*Map)
	if !ok {
		return nil, fmt.Errorf("headers 必须是字典")
	}
	for _, k := range m.Keys {
		out[k] = ToString(m.M[k])
	}
	return out, nil
}

func toJSONValue(v Value, depth int) (any, error) {
	if depth > 100 {
		return nil, fmt.Errorf("json_encode: 数据嵌套太深")
	}
	switch x := v.(type) {
	case nil:
		return nil, nil
	case bool:
		return x, nil
	case float64:
		return x, nil
	case string:
		return x, nil
	case *List:
		out := make([]any, 0, len(x.Items))
		for _, item := range x.Items {
			j, err := toJSONValue(item, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, j)
		}
		return out, nil
	case *Map:
		out := make(map[string]any, len(x.M))
		for _, k := range x.Keys {
			j, err := toJSONValue(x.M[k], depth+1)
			if err != nil {
				return nil, err
			}
			out[k] = j
		}
		return out, nil
	case *Instance:
		out := make(map[string]any, len(x.Fields.M))
		for _, k := range x.Fields.Keys {
			j, err := toJSONValue(x.Fields.M[k], depth+1)
			if err != nil {
				return nil, err
			}
			out[k] = j
		}
		return out, nil
	default:
		return nil, fmt.Errorf("json_encode: 不支持的类型 %s", TypeName(v))
	}
}

// EncodeJSON converts an Agui value to JSON bytes. It is used by the GUI game
// helpers (save_json) so both paths share exactly the same conversion rules.
func EncodeJSON(v Value) ([]byte, error) {
	x, err := toJSONValue(v, 0)
	if err != nil {
		return nil, err
	}
	return json.Marshal(x)
}

// DecodeJSON parses JSON bytes into Agui values.
func DecodeJSON(data []byte) (Value, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("JSON 后面还有多余内容")
		}
		return nil, err
	}
	return fromJSONValue(raw), nil
}

func fromJSONValue(v any) Value {
	switch x := v.(type) {
	case nil:
		return nil
	case bool:
		return x
	case string:
		return x
	case json.Number:
		f, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			return 0.0
		}
		return f
	case float64:
		return x
	case []any:
		out := &List{Items: make([]Value, 0, len(x))}
		for _, item := range x {
			out.Items = append(out.Items, fromJSONValue(item))
		}
		return out
	case map[string]any:
		out := NewMap()
		for k, item := range x {
			out.Set(k, fromJSONValue(item))
		}
		return out
	default:
		return nil
	}
}
