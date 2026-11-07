package lang

import (
	"encoding/hex"
	"sort"
	"strings"
)

// callMethod dispatches `obj.name(args)`.
func (i *Interp) callMethod(obj Value, name string, args []Value, kwargs map[string]Value) Value {
	if w, ok := obj.(Widget); ok {
		v, handled, err := w.CallMethod(name, args, kwargs)
		if err != nil {
			i.throw("%s", err.Error())
		}
		if !handled {
			i.throw("控件 %s 没有方法 %q", w.Kind(), name)
		}
		return v
	}
	if t, ok := obj.(*Task); ok {
		v, handled, err := t.CallMethod(i, name, args, kwargs)
		if err != nil {
			i.throw("%s", err.Error())
		}
		if !handled {
			i.throw("任务没有方法 %q", name)
		}
		return v
	}
	if len(kwargs) > 0 {
		i.throw("方法 %s 不支持关键字参数", name)
	}

	switch o := obj.(type) {
	case *List:
		return i.listMethod(o, name, args)
	case *Bytes:
		return i.bytesMethod(o, name, args)
	case string:
		return i.stringMethod(o, name, args)
	case *Map:
		return i.mapMethod(o, name, args)
	}
	i.throw("%s 没有方法 %q", TypeName(obj), name)
	return nil
}

func (i *Interp) bytesMethod(b *Bytes, name string, args []Value) Value {
	switch name {
	case "len":
		return float64(len(b.Data))
	case "push", "append":
		if len(args) == 0 {
			i.throw("push() 需要至少一个参数")
		}
		for _, v := range args {
			n, ok := asInt(v)
			if !ok || n < 0 || n > 255 {
				i.throw("bytes.push() 只接受 0 到 255 的整数")
			}
			b.Data = append(b.Data, byte(n))
		}
		return b
	case "pop":
		if len(b.Data) == 0 {
			i.throw("pop() 不能对空 bytes 使用")
		}
		v := b.Data[len(b.Data)-1]
		b.Data = b.Data[:len(b.Data)-1]
		return float64(v)
	case "clear":
		b.Data = nil
		return nil
	case "slice":
		start, end := 0, len(b.Data)
		if len(args) > 0 {
			n, ok := asInt(args[0])
			if !ok {
				i.throw("slice() 的下标必须是整数")
			}
			start = n
		}
		if len(args) > 1 {
			n, ok := asInt(args[1])
			if !ok {
				i.throw("slice() 的下标必须是整数")
			}
			end = n
		}
		if start < 0 {
			start += len(b.Data)
		}
		if end < 0 {
			end += len(b.Data)
		}
		if start < 0 {
			start = 0
		}
		if end > len(b.Data) {
			end = len(b.Data)
		}
		if start > end {
			start = end
		}
		return &Bytes{Data: append([]byte(nil), b.Data[start:end]...)}
	case "to_string", "string":
		return string(b.Data)
	case "hex":
		return hex.EncodeToString(b.Data)
	}
	i.throw("bytes 没有方法 %q", name)
	return nil
}

func (i *Interp) listMethod(l *List, name string, args []Value) Value {
	switch name {
	case "push", "append":
		if len(args) == 0 {
			i.throw("push() 需要至少一个参数")
		}
		l.Items = append(l.Items, args...)
		return l
	case "pop":
		if len(l.Items) == 0 {
			i.throw("pop() 不能对空列表使用")
		}
		idx := len(l.Items) - 1
		if len(args) > 0 {
			n, ok := asInt(args[0])
			if !ok {
				i.throw("pop() 的下标必须是整数")
			}
			idx = n
			if idx < 0 {
				idx += len(l.Items)
			}
		}
		if idx < 0 || idx >= len(l.Items) {
			i.throw("pop() 下标越界")
		}
		v := l.Items[idx]
		l.Items = append(l.Items[:idx], l.Items[idx+1:]...)
		return v
	case "insert":
		if len(args) != 2 {
			i.throw("insert(下标, 值) 需要两个参数")
		}
		idx, ok := asInt(args[0])
		if !ok {
			i.throw("insert() 的下标必须是整数")
		}
		if idx < 0 {
			idx += len(l.Items)
			if idx < 0 {
				idx = 0
			}
		}
		if idx > len(l.Items) {
			idx = len(l.Items)
		}
		l.Items = append(l.Items, nil)
		copy(l.Items[idx+1:], l.Items[idx:])
		l.Items[idx] = args[1]
		return l
	case "remove":
		if len(args) != 1 {
			i.throw("remove(值) 需要一个参数")
		}
		for idx, v := range l.Items {
			if Equal(v, args[0]) {
				l.Items = append(l.Items[:idx], l.Items[idx+1:]...)
				return true
			}
		}
		return false
	case "clear":
		l.Items = nil
		return nil
	case "sort":
		if err := sortValues(l.Items); err != nil {
			i.throw("%s", err.Error())
		}
		return l
	case "reverse":
		for a, b := 0, len(l.Items)-1; a < b; a, b = a+1, b-1 {
			l.Items[a], l.Items[b] = l.Items[b], l.Items[a]
		}
		return l
	case "index":
		if len(args) != 1 {
			i.throw("index(值) 需要一个参数")
		}
		for idx, v := range l.Items {
			if Equal(v, args[0]) {
				return float64(idx)
			}
		}
		return float64(-1)
	case "join":
		sep := ""
		if len(args) > 0 {
			s, ok := asString(args[0])
			if !ok {
				i.throw("join() 的分隔符必须是字符串")
			}
			sep = s
		}
		parts := make([]string, len(l.Items))
		for idx, v := range l.Items {
			parts[idx] = ToString(v)
		}
		return strings.Join(parts, sep)
	case "len":
		return float64(len(l.Items))
	case "slice":
		start, end := 0, len(l.Items)
		if len(args) > 0 {
			n, ok := asInt(args[0])
			if !ok {
				i.throw("slice() 的下标必须是整数")
			}
			start = n
		}
		if len(args) > 1 {
			n, ok := asInt(args[1])
			if !ok {
				i.throw("slice() 的下标必须是整数")
			}
			end = n
		}
		if start < 0 {
			start += len(l.Items)
		}
		if end < 0 {
			end += len(l.Items)
		}
		if start < 0 {
			start = 0
		}
		if end > len(l.Items) {
			end = len(l.Items)
		}
		if start > end {
			start = end
		}
		out := make([]Value, end-start)
		copy(out, l.Items[start:end])
		return &List{Items: out}
	case "map":
		if len(args) != 1 {
			i.throw("map(函数) 需要一个函数参数")
		}
		out := make([]Value, len(l.Items))
		for idx, v := range l.Items {
			out[idx] = i.call(args[0], []Value{v}, nil)
		}
		return &List{Items: out}
	case "filter":
		if len(args) != 1 {
			i.throw("filter(函数) 需要一个函数参数")
		}
		var out []Value
		for _, v := range l.Items {
			if Truthy(i.call(args[0], []Value{v}, nil)) {
				out = append(out, v)
			}
		}
		return &List{Items: out}
	case "each":
		if len(args) != 1 {
			i.throw("each(函数) 需要一个函数参数")
		}
		for idx, v := range l.Items {
			i.call(args[0], []Value{v, float64(idx)}, nil)
		}
		return nil
	}
	i.throw("列表没有方法 %q", name)
	return nil
}

func (i *Interp) stringMethod(s, name string, args []Value) Value {
	switch name {
	case "upper":
		return strings.ToUpper(s)
	case "lower":
		return strings.ToLower(s)
	case "trim":
		if len(args) == 0 {
			return strings.TrimSpace(s)
		}
		cut, ok := asString(args[0])
		if !ok {
			i.throw("trim() 的参数必须是字符串")
		}
		return strings.Trim(s, cut)
	case "split":
		if len(args) == 0 {
			fields := strings.Fields(s)
			out := make([]Value, len(fields))
			for idx, f := range fields {
				out[idx] = f
			}
			return &List{Items: out}
		}
		sep, ok := asString(args[0])
		if !ok {
			i.throw("split() 的分隔符必须是字符串")
		}
		parts := strings.Split(s, sep)
		out := make([]Value, len(parts))
		for idx, p := range parts {
			out[idx] = p
		}
		return &List{Items: out}
	case "replace":
		if len(args) != 2 {
			i.throw("replace(旧, 新) 需要两个参数")
		}
		old, ok1 := asString(args[0])
		new, ok2 := asString(args[1])
		if !ok1 || !ok2 {
			i.throw("replace() 的参数必须是字符串")
		}
		return strings.ReplaceAll(s, old, new)
	case "contains":
		if len(args) != 1 {
			i.throw("contains(子串) 需要一个参数")
		}
		sub, ok := asString(args[0])
		if !ok {
			i.throw("contains() 的参数必须是字符串")
		}
		return strings.Contains(s, sub)
	case "startswith":
		sub, ok := asString(Pos(args, 0, nil))
		if !ok {
			i.throw("startswith() 的参数必须是字符串")
		}
		return strings.HasPrefix(s, sub)
	case "endswith":
		sub, ok := asString(Pos(args, 0, nil))
		if !ok {
			i.throw("endswith() 的参数必须是字符串")
		}
		return strings.HasSuffix(s, sub)
	case "find":
		sub, ok := asString(Pos(args, 0, nil))
		if !ok {
			i.throw("find() 的参数必须是字符串")
		}
		return float64(runeIndex(s, sub))
	case "substr":
		runes := []rune(s)
		start := 0
		if len(args) > 0 {
			n, ok := asInt(args[0])
			if !ok {
				i.throw("substr() 的起始位置必须是整数")
			}
			start = n
		}
		if start < 0 {
			start += len(runes)
		}
		if start < 0 {
			start = 0
		}
		if start > len(runes) {
			start = len(runes)
		}
		end := len(runes)
		if len(args) > 1 {
			n, ok := asInt(args[1])
			if !ok {
				i.throw("substr() 的长度必须是整数")
			}
			end = start + n
			if end > len(runes) {
				end = len(runes)
			}
			if end < start {
				end = start
			}
		}
		return string(runes[start:end])
	case "repeat":
		n, ok := asInt(Pos(args, 0, nil))
		if !ok || n < 0 {
			i.throw("repeat() 需要非负整数")
		}
		if n > 1_000_000 {
			i.throw("repeat() 次数太大")
		}
		return strings.Repeat(s, n)
	case "len":
		return float64(len([]rune(s)))
	case "chars":
		runes := []rune(s)
		out := make([]Value, len(runes))
		for idx, r := range runes {
			out[idx] = string(r)
		}
		return &List{Items: out}
	}
	i.throw("字符串没有方法 %q", name)
	return nil
}

func (i *Interp) mapMethod(m *Map, name string, args []Value) Value {
	switch name {
	case "keys":
		out := make([]Value, len(m.Keys))
		for idx, k := range m.Keys {
			out[idx] = k
		}
		return &List{Items: out}
	case "values":
		out := make([]Value, len(m.Keys))
		for idx, k := range m.Keys {
			out[idx] = m.M[k]
		}
		return &List{Items: out}
	case "items":
		out := make([]Value, len(m.Keys))
		for idx, k := range m.Keys {
			out[idx] = &List{Items: []Value{k, m.M[k]}}
		}
		return &List{Items: out}
	case "has":
		if len(args) != 1 {
			i.throw("has(键) 需要一个参数")
		}
		k, ok := keyString(args[0])
		if !ok {
			return false
		}
		_, exists := m.Get(k)
		return exists
	case "get":
		if len(args) < 1 {
			i.throw("get(键, 默认值) 至少需要一个参数")
		}
		k, ok := keyString(args[0])
		if !ok {
			i.throw("字典的键必须是字符串或数字")
		}
		if v, exists := m.Get(k); exists {
			return v
		}
		return Pos(args, 1, nil)
	case "set":
		if len(args) != 2 {
			i.throw("set(键, 值) 需要两个参数")
		}
		k, ok := keyString(args[0])
		if !ok {
			i.throw("字典的键必须是字符串或数字")
		}
		m.Set(k, args[1])
		return m
	case "remove":
		if len(args) != 1 {
			i.throw("remove(键) 需要一个参数")
		}
		k, ok := keyString(args[0])
		if !ok {
			return false
		}
		return m.Delete(k)
	case "clear":
		m.Keys = nil
		m.M = map[string]Value{}
		return nil
	case "len":
		return float64(len(m.M))
	}
	i.throw("字典没有方法 %q", name)
	return nil
}

func sortValues(items []Value) error {
	allNum, allStr := true, true
	for _, v := range items {
		if _, ok := v.(float64); !ok {
			allNum = false
		}
		if _, ok := v.(string); !ok {
			allStr = false
		}
	}
	switch {
	case allNum:
		sort.SliceStable(items, func(a, b int) bool {
			return items[a].(float64) < items[b].(float64)
		})
	case allStr:
		sort.SliceStable(items, func(a, b int) bool {
			return items[a].(string) < items[b].(string)
		})
	default:
		return errString("列表排序只支持全是数字或全是字符串")
	}
	return nil
}

func runeIndex(s, sub string) int {
	if sub == "" {
		return 0
	}
	sr, subr := []rune(s), []rune(sub)
	for idx := 0; idx+len(subr) <= len(sr); idx++ {
		match := true
		for k := range subr {
			if sr[idx+k] != subr[k] {
				match = false
				break
			}
		}
		if match {
			return idx
		}
	}
	return -1
}

type errString string

func (e errString) Error() string { return string(e) }
