package lang

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// registerHTTP installs the enhanced HTTP client. The old, small helpers in
// stdlib.go remain for compatibility but are overridden by these names.
func registerHTTP(i *Interp) {
	i.RegisterNative("http_get", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("http_get(url, ...) 需要 1 个参数")
		}
		return httpDo(i, "GET", args[0], nil, kwargs)
	})
	i.RegisterNative("http_post", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("http_post(url, body=\"\", ...) 接受 1 到 2 个参数")
		}
		var body Value = ""
		if len(args) == 2 {
			body = args[1]
		}
		return httpDo(i, "POST", args[0], body, kwargs)
	})
	i.RegisterNative("http_put", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		return httpBodyMethod(i, "PUT", args, kwargs)
	})
	i.RegisterNative("http_patch", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		return httpBodyMethod(i, "PATCH", args, kwargs)
	})
	i.RegisterNative("http_delete", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		return httpBodyMethod(i, "DELETE", args, kwargs)
	})
	i.RegisterNative("http_head", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("http_head(url, ...) 需要 1 个参数")
		}
		return httpDo(i, "HEAD", args[0], nil, kwargs)
	})
	i.RegisterNative("http_request", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("http_request(method, url, body=\"\", ...) 接受 2 到 3 个参数")
		}
		method, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("http_request(): method 必须是字符串")
		}
		var body Value = ""
		if len(args) == 3 {
			body = args[2]
		}
		return httpDo(i, strings.ToUpper(method), args[1], body, kwargs)
	})
	i.RegisterNative("http_json", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("http_json(url, data, method=\"POST\", ...) 接受 2 到 3 个参数")
		}
		method := "POST"
		if len(args) == 3 {
			s, _ := asString(args[2])
			if s != "" {
				method = strings.ToUpper(s)
			}
		}
		kw := cloneKwargs(kwargs)
		kw["json"] = args[1]
		return httpDo(i, method, args[0], nil, kw)
	})
	i.RegisterNative("http_download", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("http_download(url, path, ...) 需要 2 个参数")
		}
		path, ok := asString(args[1])
		if !ok || strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("http_download(): path 必须是非空字符串")
		}
		kw := cloneKwargs(kwargs)
		kw["raise"] = false
		res, err := httpDo(i, "GET", args[0], nil, kw)
		if err != nil {
			return res, err
		}
		m, _ := res.(*Map)
		if m == nil {
			return res, nil
		}
		if ok, _ := m.Get("ok"); !Truthy(ok) {
			return m, nil
		}
		body, _ := m.Get("body")
		data, _ := asString(body)
		if err := i.Blocking(func() error { return os.WriteFile(path, []byte(data), 0o644) }); err != nil {
			return nil, fmt.Errorf("http_download: %v", err)
		}
		m.Set("path", path)
		m.Set("size", float64(len(data)))
		return m, nil
	})
	i.RegisterNative("http_upload", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("http_upload(url, files=..., ...) 接受 1 到 2 个参数")
		}
		kw := cloneKwargs(kwargs)
		if len(args) == 2 {
			kw["files"] = args[1]
		}
		return httpDo(i, "POST", args[0], nil, kw)
	})

	i.RegisterNative("url_encode", func(_ *Interp, args []Value, _ map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("url_encode(字符串) 需要 1 个参数")
		}
		return url.QueryEscape(ToString(args[0])), nil
	})
	i.RegisterNative("url_decode", func(_ *Interp, args []Value, _ map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("url_decode(字符串) 需要 1 个参数")
		}
		s, err := url.QueryUnescape(ToString(args[0]))
		if err != nil {
			return nil, fmt.Errorf("url_decode: %v", err)
		}
		return s, nil
	})
	i.RegisterNative("query_encode", func(_ *Interp, args []Value, _ map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("query_encode(字典) 需要 1 个参数")
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, fmt.Errorf("query_encode(): 参数必须是字典")
		}
		vals := url.Values{}
		for _, k := range m.Keys {
			addQueryValue(vals, k, m.M[k])
		}
		return vals.Encode(), nil
	})
	i.RegisterNative("url_parse", func(_ *Interp, args []Value, _ map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("url_parse(字符串) 需要 1 个参数")
		}
		u, err := url.Parse(ToString(args[0]))
		if err != nil {
			return nil, fmt.Errorf("url_parse: %v", err)
		}
		out := NewMap()
		out.Set("scheme", u.Scheme)
		out.Set("host", u.Hostname())
		out.Set("port", u.Port())
		out.Set("path", u.Path)
		out.Set("query", u.RawQuery)
		out.Set("fragment", u.Fragment)
		out.Set("user", u.User.Username())
		return out, nil
	})
}

func httpBodyMethod(i *Interp, method string, args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("%s(url, body=\"\", ...) 接受 1 到 2 个参数", strings.ToLower(method))
	}
	var body Value = ""
	if len(args) == 2 {
		body = args[1]
	}
	return httpDo(i, method, args[0], body, kwargs)
}

type httpRequestOptions struct {
	timeout       time.Duration
	headers       map[string]string
	body          []byte
	contentType   string
	cookies       map[string]string
	user, pass    string
	follow        bool
	maxRedirects  int
	proxy         string
	insecure      bool
	retries       int
	retryDelay    time.Duration
	raise         bool
	multipartBody []byte
	boundary      string
}

func httpDo(i *Interp, method string, urlVal Value, body Value, kwargs map[string]Value) (Value, error) {
	rawURL, ok := asString(urlVal)
	if !ok || strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("%s: url 必须是非空字符串", strings.ToLower(method))
	}
	opts, err := parseHTTPOptions(body, kwargs)
	if err != nil {
		return nil, err
	}
	if opts.multipartBody != nil {
		opts.body = opts.multipartBody
		opts.contentType = "multipart/form-data; boundary=" + opts.boundary
	}
	if opts.contentType != "" && opts.headers["Content-Type"] == "" {
		opts.headers["Content-Type"] = opts.contentType
	}
	started := time.Now()
	var lastErr error
	var status int
	var data []byte
	var header http.Header
	var finalURL string
	var respCookies []*http.Cookie
	attempts := opts.retries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 && opts.retryDelay > 0 {
			select {
			case <-time.After(opts.retryDelay):
			case <-taskDone(i):
				return httpFailure(i, opts, started, fmt.Errorf("%s 已取消", strings.ToLower(method)))
			}
		}
		status, header, data, finalURL, respCookies, lastErr = doOneHTTP(i, method, rawURL, opts)
		if lastErr == nil && status < 500 {
			break
		}
		if lastErr == nil && attempt+1 >= attempts {
			break
		}
	}
	if lastErr != nil {
		if ctx := i.TaskContext(); ctx != nil && ctx.Err() != nil {
			return httpFailure(i, opts, started, fmt.Errorf("%s 已取消", strings.ToLower(method)))
		}
		return httpFailure(i, opts, started, fmt.Errorf("%s %s: %v", method, rawURL, lastErr))
	}
	out := NewMap()
	out.Set("status", float64(status))
	out.Set("ok", status >= 200 && status < 300)
	out.Set("body", string(data))
	h := NewMap()
	for k, vals := range header {
		h.Set(k, strings.Join(vals, ", "))
	}
	out.Set("headers", h)
	out.Set("url", finalURL)
	out.Set("content_type", header.Get("Content-Type"))
	out.Set("length", float64(len(data)))
	out.Set("elapsed", time.Since(started).Seconds())
	cookies := NewMap()
	for _, c := range respCookies {
		cookies.Set(c.Name, c.Value)
	}
	out.Set("cookies", cookies)
	if len(data) > 0 {
		var parsed any
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&parsed); err == nil {
			out.Set("json", fromJSONValue(parsed))
		} else {
			out.Set("json", nil)
		}
	} else {
		out.Set("json", nil)
	}
	if status >= 400 && opts.raise {
		return out, fmt.Errorf("HTTP %d: %s", status, strings.TrimSpace(string(data)))
	}
	return out, nil
}

func parseHTTPOptions(body Value, kwargs map[string]Value) (httpRequestOptions, error) {
	o := httpRequestOptions{timeout: 15 * time.Second, follow: true, maxRedirects: 10, raise: true}
	if v, ok := kwargs["timeout"]; ok {
		f, ok := asNumber(v)
		if !ok || f <= 0 {
			return o, fmt.Errorf("timeout 必须是正数（秒）")
		}
		o.timeout = time.Duration(f * float64(time.Second))
	}
	if v, ok := kwargs["headers"]; ok {
		h, err := headerMap(v)
		if err != nil {
			return o, err
		}
		o.headers = h
	}
	if o.headers == nil {
		o.headers = map[string]string{}
	}
	if v, ok := kwargs["cookies"]; ok {
		m, ok := v.(*Map)
		if !ok {
			return o, fmt.Errorf("cookies 必须是字典")
		}
		o.cookies = map[string]string{}
		for _, k := range m.Keys {
			o.cookies[k] = ToString(m.M[k])
		}
	}
	if v, ok := kwargs["auth"]; ok {
		switch a := v.(type) {
		case *List:
			if len(a.Items) != 2 {
				return o, fmt.Errorf("auth 必须是 [用户名, 密码] 或字典")
			}
			o.user, _ = asString(a.Items[0])
			o.pass, _ = asString(a.Items[1])
		case *Map:
			u, _ := a.Get("user")
			p, _ := a.Get("pass")
			o.user, _ = asString(u)
			o.pass, _ = asString(p)
		default:
			return o, fmt.Errorf("auth 必须是 [用户名, 密码] 或字典")
		}
	}
	o.follow = stdBool(kwargs, "follow_redirects", true)
	if v, ok := kwargs["max_redirects"]; ok {
		n, ok := asInt(v)
		if !ok || n < 0 {
			return o, fmt.Errorf("max_redirects 必须是非负整数")
		}
		o.maxRedirects = n
	}
	if v, ok := kwargs["proxy"]; ok {
		o.proxy, _ = asString(v)
	}
	o.insecure = stdBool(kwargs, "insecure", false)
	o.raise = stdBool(kwargs, "raise", true)
	if v, ok := kwargs["retry"]; ok {
		n, ok := asInt(v)
		if !ok || n < 0 || n > 20 {
			return o, fmt.Errorf("retry 必须是 0..20 的整数")
		}
		o.retries = n
	}
	if v, ok := kwargs["retry_delay"]; ok {
		f, ok := asNumber(v)
		if !ok || f < 0 {
			return o, fmt.Errorf("retry_delay 必须是非负数（秒）")
		}
		o.retryDelay = time.Duration(f * float64(time.Second))
	}
	if v, ok := kwargs["json"]; ok {
		j, err := toJSONValue(v, 0)
		if err != nil {
			return o, err
		}
		b, err := json.Marshal(j)
		if err != nil {
			return o, fmt.Errorf("json 序列化: %v", err)
		}
		o.body = b
		o.contentType = "application/json"
	}
	if v, ok := kwargs["form"]; ok {
		m, ok := v.(*Map)
		if !ok {
			return o, fmt.Errorf("form 必须是字典")
		}
		vals := url.Values{}
		for _, k := range m.Keys {
			addQueryValue(vals, k, m.M[k])
		}
		o.body = []byte(vals.Encode())
		o.contentType = "application/x-www-form-urlencoded"
	}
	if v, ok := kwargs["files"]; ok {
		body, boundary, err := buildMultipart(v)
		if err != nil {
			return o, err
		}
		if boundary != "" {
			o.multipartBody = body
			o.boundary = boundary
		}
	}
	if v, ok := kwargs["content_type"]; ok {
		o.contentType, _ = asString(v)
	}
	if body != nil && o.body == nil {
		o.body = []byte(ToString(body))
	}
	return o, nil
}

func doOneHTTP(i *Interp, method, rawURL string, o httpRequestOptions) (int, http.Header, []byte, string, []*http.Cookie, error) {
	var (
		status  int
		header  http.Header
		data    []byte
		final   string
		cookies []*http.Cookie
	)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if o.insecure {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	if o.proxy != "" {
		pu, err := url.Parse(o.proxy)
		if err != nil {
			return 0, nil, nil, "", nil, fmt.Errorf("proxy: %v", err)
		}
		transport.Proxy = http.ProxyURL(pu)
	}
	client := &http.Client{Transport: transport, Timeout: o.timeout}
	if !o.follow {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	} else if o.maxRedirects >= 0 {
		max := o.maxRedirects
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= max {
				return fmt.Errorf("stopped after %d redirects", max)
			}
			return nil
		}
	}
	err := i.Blocking(func() error {
		req, err := http.NewRequest(method, rawURL, bytes.NewReader(o.body))
		if err != nil {
			return err
		}
		if ctx := i.TaskContext(); ctx != nil {
			req = req.WithContext(ctx)
		}
		for k, v := range o.headers {
			req.Header.Set(k, v)
		}
		if o.user != "" || o.pass != "" {
			req.SetBasicAuth(o.user, o.pass)
		}
		for k, v := range o.cookies {
			req.AddCookie(&http.Cookie{Name: k, Value: v})
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		status = resp.StatusCode
		header = resp.Header
		final = resp.Request.URL.String()
		cookies = resp.Cookies()
		data, err = io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		return err
	})
	return status, header, data, final, cookies, err
}

func httpFailure(i *Interp, o httpRequestOptions, started time.Time, err error) (Value, error) {
	if o.raise {
		return nil, err
	}
	out := NewMap()
	out.Set("status", float64(0))
	out.Set("ok", false)
	out.Set("body", "")
	out.Set("headers", NewMap())
	out.Set("url", "")
	out.Set("error", err.Error())
	out.Set("elapsed", time.Since(started).Seconds())
	return out, nil
}

func taskDone(i *Interp) <-chan struct{} {
	if ctx := i.TaskContext(); ctx != nil {
		return ctx.Done()
	}
	return nil
}

func addQueryValue(vals url.Values, key string, v Value) {
	if list, ok := v.(*List); ok {
		for _, item := range list.Items {
			vals.Add(key, ToString(item))
		}
		return
	}
	vals.Add(key, ToString(v))
}

func cloneKwargs(in map[string]Value) map[string]Value {
	out := make(map[string]Value, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func buildMultipart(v Value) ([]byte, string, error) {
	m, ok := v.(*Map)
	if !ok {
		return nil, "", fmt.Errorf("files 必须是字典（字段名 -> 文件路径）")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, field := range m.Keys {
		path, filename := multipartFileSpec(m.M[field])
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("files.%s: %v", field, err)
		}
		if filename == "" {
			filename = filepath.Base(path)
		}
		part, err := w.CreateFormFile(field, filename)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(data); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.Boundary(), nil
}

func multipartFileSpec(v Value) (string, string) {
	switch x := v.(type) {
	case string:
		return x, ""
	case *List:
		if len(x.Items) == 0 {
			return "", ""
		}
		path, _ := asString(x.Items[0])
		name := ""
		if len(x.Items) > 1 {
			name, _ = asString(x.Items[1])
		}
		return path, name
	default:
		return "", ""
	}
}
