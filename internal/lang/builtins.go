package lang

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func registerBuiltins(i *Interp) {
	reg := func(name string, fn func(*Interp, []Value, map[string]Value) (Value, error)) {
		i.RegisterNative(name, fn)
	}

	reg("print", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		parts := make([]string, len(args))
		for idx, v := range args {
			parts[idx] = ToString(v)
		}
		fmt.Fprintln(i.Out, strings.Join(parts, " "))
		return nil, nil
	})

	reg("warn", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		parts := make([]string, len(args))
		for idx, v := range args {
			parts[idx] = ToString(v)
		}
		fmt.Fprintln(os.Stderr, strings.Join(parts, " "))
		return nil, nil
	})

	reg("debug_vars", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("debug_vars() 不接受参数")
		}
		out := NewMap()
		names := make([]string, 0)
		snapshot := i.GlobalsSnapshot()
		for name := range snapshot {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if _, ok := snapshot[name].(*Native); ok {
				continue
			}
			out.Set(name, snapshot[name])
		}
		return out, nil
	})

	reg("str", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("str() 需要一个参数")
		}
		return ToString(args[0]), nil
	})

	reg("num", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("num() 需要一个参数")
		}
		switch v := args[0].(type) {
		case float64:
			return v, nil
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return nil, fmt.Errorf("num(): 无法把 %q 转成数字", v)
			}
			return f, nil
		case bool:
			if v {
				return float64(1), nil
			}
			return float64(0), nil
		}
		return nil, fmt.Errorf("num(): %s 不能转成数字", TypeName(args[0]))
	})

	reg("int", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("int() 需要一个参数")
		}
		switch v := args[0].(type) {
		case float64:
			return math.Trunc(v), nil
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return nil, fmt.Errorf("int(): 无法把 %q 转成数字", v)
			}
			return math.Trunc(f), nil
		}
		return nil, fmt.Errorf("int(): %s 不能转成整数", TypeName(args[0]))
	})

	reg("bool", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("bool() 需要一个参数")
		}
		return Truthy(args[0]), nil
	})

	reg("type", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("type() 需要一个参数")
		}
		return TypeName(args[0]), nil
	})

	reg("len", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("len() 需要一个参数")
		}
		switch v := args[0].(type) {
		case string:
			return float64(len([]rune(v))), nil
		case *List:
			return float64(len(v.Items)), nil
		case *Bytes:
			return float64(len(v.Data)), nil
		case *Map:
			return float64(len(v.M)), nil
		}
		return nil, fmt.Errorf("len(): %s 没有长度", TypeName(args[0]))
	})

	reg("bytes", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) == 0 {
			return &Bytes{}, nil
		}
		if len(args) > 1 {
			return nil, fmt.Errorf("bytes(值) 最多接受 1 个参数")
		}
		data, err := valueBytes("bytes", args[0])
		if err != nil {
			return nil, err
		}
		return &Bytes{Data: data}, nil
	})
	reg("to_bytes", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("to_bytes(值) 需要一个参数")
		}
		data, err := valueBytes("to_bytes", args[0])
		if err != nil {
			return nil, err
		}
		return &Bytes{Data: data}, nil
	})
	reg("bytes_to_string", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("bytes_to_string(值) 需要一个参数")
		}
		switch v := args[0].(type) {
		case *Bytes:
			return string(v.Data), nil
		case string:
			return v, nil
		}
		return nil, fmt.Errorf("bytes_to_string(): 需要 bytes 或字符串，收到 %s", TypeName(args[0]))
	})
	reg("hex_encode", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("hex_encode(值) 需要一个参数")
		}
		data, err := valueBytes("hex_encode", args[0])
		if err != nil {
			return nil, err
		}
		return hex.EncodeToString(data), nil
	})
	reg("hex_decode", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		s, err := oneString("hex_decode", args)
		if err != nil {
			return nil, err
		}
		data, decErr := hex.DecodeString(strings.TrimSpace(s))
		if decErr != nil {
			return nil, fmt.Errorf("hex_decode(): %v", decErr)
		}
		return &Bytes{Data: data}, nil
	})

	reg("range", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 3 {
			return nil, fmt.Errorf("range() 接受 1 到 3 个参数")
		}
		nums := make([]int, len(args))
		for idx, a := range args {
			n, ok := asInt(a)
			if !ok {
				return nil, fmt.Errorf("range() 的参数必须是整数")
			}
			nums[idx] = n
		}
		start, stop, step := 0, 0, 1
		switch len(nums) {
		case 1:
			stop = nums[0]
		case 2:
			start, stop = nums[0], nums[1]
		case 3:
			start, stop, step = nums[0], nums[1], nums[2]
		}
		if step == 0 {
			return nil, fmt.Errorf("range() 的步长不能是 0")
		}
		count := 0
		if step > 0 {
			if stop > start {
				count = (stop - start + step - 1) / step
			}
		} else {
			if stop < start {
				count = (start - stop + (-step) - 1) / (-step)
			}
		}
		if count > 10_000_000 {
			return nil, fmt.Errorf("range() 生成的列表太大")
		}
		items := make([]Value, 0, count)
		for v := start; (step > 0 && v < stop) || (step < 0 && v > stop); v += step {
			items = append(items, float64(v))
		}
		return &List{Items: items}, nil
	})

	reg("list", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) == 0 {
			return &List{}, nil
		}
		if len(args) > 1 {
			return &List{Items: append([]Value(nil), args...)}, nil
		}
		switch v := args[0].(type) {
		case *List:
			return &List{Items: append([]Value(nil), v.Items...)}, nil
		case string:
			runes := []rune(v)
			out := make([]Value, len(runes))
			for idx, r := range runes {
				out[idx] = string(r)
			}
			return &List{Items: out}, nil
		case *Map:
			out := make([]Value, len(v.Keys))
			for idx, k := range v.Keys {
				out[idx] = k
			}
			return &List{Items: out}, nil
		default:
			return nil, fmt.Errorf("list(): %s 不能转换成列表", TypeName(args[0]))
		}
	})

	reg("dict", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) > 1 {
			return nil, fmt.Errorf("dict() 接受 0 或 1 个位置参数")
		}
		out := NewMap()
		if len(args) == 1 {
			src, ok := args[0].(*Map)
			if !ok {
				return nil, fmt.Errorf("dict(): 参数必须是字典")
			}
			for _, k := range src.Keys {
				out.Set(k, src.M[k])
			}
		}
		names := make([]string, 0, len(kwargs))
		for k := range kwargs {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			out.Set(k, kwargs[k])
		}
		return out, nil
	})

	reg("append", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("append(列表, 值...) 至少需要两个参数")
		}
		l, ok := args[0].(*List)
		if !ok {
			return nil, fmt.Errorf("append(): 第一个参数必须是列表")
		}
		l.Items = append(l.Items, args[1:]...)
		return l, nil
	})

	reg("pop", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("pop(列表, 下标) 接受 1 到 2 个参数")
		}
		l, ok := args[0].(*List)
		if !ok {
			return nil, fmt.Errorf("pop(): 第一个参数必须是列表")
		}
		idx := len(l.Items) - 1
		if len(args) == 2 {
			n, ok := asInt(args[1])
			if !ok {
				return nil, fmt.Errorf("pop(): 下标必须是整数")
			}
			idx = n
			if idx < 0 {
				idx += len(l.Items)
			}
		}
		if idx < 0 || idx >= len(l.Items) {
			return nil, fmt.Errorf("pop(): 下标越界")
		}
		v := l.Items[idx]
		l.Items = append(l.Items[:idx], l.Items[idx+1:]...)
		return v, nil
	})

	reg("insert", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("insert(列表, 下标, 值) 需要三个参数")
		}
		return i.listMethod(args[0].(*List), "insert", args[1:]), nil
	})

	reg("remove", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("remove(列表, 值) 需要两个参数")
		}
		l, ok := args[0].(*List)
		if !ok {
			return nil, fmt.Errorf("remove(): 第一个参数必须是列表")
		}
		return i.listMethod(l, "remove", args[1:]), nil
	})

	reg("keys", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("keys() 需要一个参数")
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, fmt.Errorf("keys(): 参数必须是字典")
		}
		return i.mapMethod(m, "keys", nil), nil
	})

	reg("values", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("values() 需要一个参数")
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, fmt.Errorf("values(): 参数必须是字典")
		}
		return i.mapMethod(m, "values", nil), nil
	})

	reg("items", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("items() 需要一个参数")
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, fmt.Errorf("items(): 参数必须是字典")
		}
		return i.mapMethod(m, "items", nil), nil
	})

	reg("has", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("has(容器, 值) 需要两个参数")
		}
		return i.contains(args[1], args[0]), nil
	})

	reg("get", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("get(字典, 键, 默认值) 接受 2 到 3 个参数")
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, fmt.Errorf("get(): 第一个参数必须是字典")
		}
		k, ok := keyString(args[1])
		if !ok {
			return nil, fmt.Errorf("get(): 键必须是字符串或数字")
		}
		if v, exists := m.Get(k); exists {
			return v, nil
		}
		return Pos(args, 2, nil), nil
	})

	reg("set", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("set(字典, 键, 值) 需要三个参数")
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, fmt.Errorf("set(): 第一个参数必须是字典")
		}
		k, ok := keyString(args[1])
		if !ok {
			return nil, fmt.Errorf("set(): 键必须是字符串或数字")
		}
		m.Set(k, args[2])
		return m, nil
	})

	reg("sort", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("sort() 需要一个参数")
		}
		l, ok := args[0].(*List)
		if !ok {
			return nil, fmt.Errorf("sort(): 参数必须是列表")
		}
		if err := sortValues(l.Items); err != nil {
			return nil, err
		}
		return l, nil
	})

	reg("reverse", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("reverse() 需要一个参数")
		}
		l, ok := args[0].(*List)
		if !ok {
			return nil, fmt.Errorf("reverse(): 参数必须是列表")
		}
		for a, b := 0, len(l.Items)-1; a < b; a, b = a+1, b-1 {
			l.Items[a], l.Items[b] = l.Items[b], l.Items[a]
		}
		return l, nil
	})

	reg("join", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("join(列表, 分隔符) 接受 1 到 2 个参数")
		}
		l, ok := args[0].(*List)
		if !ok {
			return nil, fmt.Errorf("join(): 第一个参数必须是列表")
		}
		sep := ""
		if len(args) == 2 {
			s, ok := asString(args[1])
			if !ok {
				return nil, fmt.Errorf("join(): 分隔符必须是字符串")
			}
			sep = s
		}
		return i.listMethod(l, "join", []Value{sep}), nil
	})

	reg("split", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("split(字符串, 分隔符) 接受 1 到 2 个参数")
		}
		s, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("split(): 第一个参数必须是字符串")
		}
		return i.stringMethod(s, "split", args[1:]), nil
	})

	reg("upper", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		s, err := oneString("upper", args)
		if err != nil {
			return nil, err
		}
		return strings.ToUpper(s), nil
	})
	reg("lower", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		s, err := oneString("lower", args)
		if err != nil {
			return nil, err
		}
		return strings.ToLower(s), nil
	})
	reg("trim", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		s, err := oneString("trim", args)
		if err != nil {
			return nil, err
		}
		return strings.TrimSpace(s), nil
	})
	reg("replace", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("replace(字符串, 旧, 新) 需要三个参数")
		}
		s, ok1 := asString(args[0])
		old, ok2 := asString(args[1])
		new, ok3 := asString(args[2])
		if !ok1 || !ok2 || !ok3 {
			return nil, fmt.Errorf("replace() 的参数都必须是字符串")
		}
		return strings.ReplaceAll(s, old, new), nil
	})
	reg("contains", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("contains(字符串, 子串) 需要两个参数")
		}
		s, ok1 := asString(args[0])
		sub, ok2 := asString(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("contains() 的参数都必须是字符串")
		}
		return strings.Contains(s, sub), nil
	})
	reg("startswith", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("startswith(字符串, 前缀) 需要两个参数")
		}
		s, ok1 := asString(args[0])
		pre, ok2 := asString(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("startswith() 的参数都必须是字符串")
		}
		return strings.HasPrefix(s, pre), nil
	})
	reg("endswith", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("endswith(字符串, 后缀) 需要两个参数")
		}
		s, ok1 := asString(args[0])
		suf, ok2 := asString(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("endswith() 的参数都必须是字符串")
		}
		return strings.HasSuffix(s, suf), nil
	})
	reg("substr", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("substr(字符串, 起点, 长度) 接受 2 到 3 个参数")
		}
		s, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("substr(): 第一个参数必须是字符串")
		}
		return i.stringMethod(s, "substr", args[1:]), nil
	})
	reg("find", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("find(字符串, 子串) 需要两个参数")
		}
		s, ok1 := asString(args[0])
		sub, ok2 := asString(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("find() 的参数都必须是字符串")
		}
		return float64(runeIndex(s, sub)), nil
	})

	reg("format", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("format(模板, ...) 至少需要一个参数")
		}
		tpl, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("format(): 第一个参数必须是字符串")
		}
		return formatTemplate(tpl, args[1:], kwargs)
	})

	reg("abs", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		f, err := oneNumber("abs", args)
		if err != nil {
			return nil, err
		}
		return math.Abs(f), nil
	})
	reg("round", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		f, err := oneNumber("round", args)
		if err != nil {
			return nil, err
		}
		return math.Round(f), nil
	})
	reg("floor", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		f, err := oneNumber("floor", args)
		if err != nil {
			return nil, err
		}
		return math.Floor(f), nil
	})
	reg("ceil", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		f, err := oneNumber("ceil", args)
		if err != nil {
			return nil, err
		}
		return math.Ceil(f), nil
	})
	reg("sqrt", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		f, err := oneNumber("sqrt", args)
		if err != nil {
			return nil, err
		}
		if f < 0 {
			return nil, fmt.Errorf("sqrt(): 不能对负数开平方")
		}
		return math.Sqrt(f), nil
	})
	reg("pow", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("pow(底数, 指数) 需要两个参数")
		}
		a, ok1 := asNumber(args[0])
		b, ok2 := asNumber(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("pow() 的参数必须是数字")
		}
		return math.Pow(a, b), nil
	})

	reg("min", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		return minMax(args, false)
	})
	reg("max", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		return minMax(args, true)
	})

	reg("sum", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("sum(列表) 需要一个参数")
		}
		l, ok := args[0].(*List)
		if !ok {
			return nil, fmt.Errorf("sum(): 参数必须是列表")
		}
		total := 0.0
		for _, v := range l.Items {
			f, ok := asNumber(v)
			if !ok {
				return nil, fmt.Errorf("sum(): 列表里含有非数字")
			}
			total += f
		}
		return total, nil
	})

	reg("random", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		return rand.Float64(), nil
	})
	reg("randint", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("randint(最小, 最大) 需要两个参数")
		}
		a, ok1 := asInt(args[0])
		b, ok2 := asInt(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("randint() 的参数必须是整数")
		}
		if a > b {
			a, b = b, a
		}
		return float64(a + rand.Intn(b-a+1)), nil
	})

	reg("now", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		return time.Now().Format("2006-01-02 15:04:05"), nil
	})
	reg("timestamp", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		return float64(time.Now().UnixNano()) / 1e9, nil
	})
	reg("sleep", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		ms, ok := asInt(Pos(args, 0, nil))
		if !ok || ms < 0 {
			return nil, fmt.Errorf("sleep(毫秒) 需要一个非负整数")
		}
		if ms > 60_000 {
			return nil, fmt.Errorf("sleep(): 最多等待 60 秒")
		}
		i.Sleep(time.Duration(ms) * time.Millisecond)
		return nil, nil
	})
	reg("set_recursion_limit", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		n, ok := asInt(Pos(args, 0, nil))
		if !ok {
			return nil, fmt.Errorf("set_recursion_limit(层数) 需要一个整数")
		}
		if n < 8 || n > 100000 {
			return nil, fmt.Errorf("递归上限必须在 8 到 100000 之间")
		}
		old := i.maxDepth
		i.maxDepth = int(n)
		return float64(old), nil
	})
	reg("get_recursion_limit", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		return float64(i.maxDepth), nil
	})

	// ---- files ----
	reg("read_file", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		path, err := oneString("read_file", args)
		if err != nil {
			return nil, err
		}
		var data []byte
		err = i.Blocking(func() error {
			var readErr error
			data, readErr = i.ReadFile(path)
			return readErr
		})
		if err != nil {
			return nil, fmt.Errorf("read_file(): %v", err)
		}
		return string(data), nil
	})
	reg("write_file", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("write_file(路径, 内容) 需要两个参数")
		}
		path, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("write_file(): 路径必须是字符串")
		}
		content := []byte(ToString(args[1]))
		path = i.ResolvePath(path)
		if err := i.Blocking(func() error { return os.WriteFile(path, content, 0o644) }); err != nil {
			return nil, fmt.Errorf("write_file(): %v", err)
		}
		return nil, nil
	})
	reg("read_bytes", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		path, err := oneString("read_bytes", args)
		if err != nil {
			return nil, err
		}
		var data []byte
		err = i.Blocking(func() error {
			var readErr error
			data, readErr = i.ReadFile(path)
			return readErr
		})
		if err != nil {
			return nil, fmt.Errorf("read_bytes(): %v", err)
		}
		return &Bytes{Data: data}, nil
	})
	reg("write_bytes", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("write_bytes(路径, 数据) 需要两个参数")
		}
		path, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("write_bytes(): 路径必须是字符串")
		}
		data, err := valueBytes("write_bytes", args[1])
		if err != nil {
			return nil, err
		}
		path = i.ResolvePath(path)
		if err := i.Blocking(func() error { return os.WriteFile(path, data, 0o644) }); err != nil {
			return nil, fmt.Errorf("write_bytes(): %v", err)
		}
		return nil, nil
	})
	reg("append_bytes", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("append_bytes(路径, 数据) 需要两个参数")
		}
		path, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("append_bytes(): 路径必须是字符串")
		}
		data, err := valueBytes("append_bytes", args[1])
		if err != nil {
			return nil, err
		}
		path = i.ResolvePath(path)
		err = i.Blocking(func() error {
			f, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if openErr != nil {
				return openErr
			}
			defer f.Close()
			_, writeErr := f.Write(data)
			return writeErr
		})
		if err != nil {
			return nil, fmt.Errorf("append_bytes(): %v", err)
		}
		return nil, nil
	})
	reg("append_file", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("append_file(路径, 内容) 需要两个参数")
		}
		path, ok := asString(args[0])
		if !ok {
			return nil, fmt.Errorf("append_file(): 路径必须是字符串")
		}
		text := ToString(args[1])
		path = i.ResolvePath(path)
		err := i.Blocking(func() error {
			f, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if openErr != nil {
				return openErr
			}
			defer f.Close()
			_, writeErr := f.WriteString(text)
			return writeErr
		})
		if err != nil {
			return nil, fmt.Errorf("append_file(): %v", err)
		}
		return nil, nil
	})
	reg("file_exists", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		path, err := oneString("file_exists", args)
		if err != nil {
			return nil, err
		}
		return i.FileExists(path), nil
	})
	reg("delete_file", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		path, err := oneString("delete_file", args)
		if err != nil {
			return nil, err
		}
		path = i.ResolvePath(path)
		if removeErr := i.Blocking(func() error { return os.Remove(path) }); removeErr != nil {
			return nil, fmt.Errorf("delete_file(): %v", removeErr)
		}
		return nil, nil
	})
	reg("list_dir", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		path, err := oneString("list_dir", args)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		var names []string
		for _, name := range i.AssetNames(path) {
			base := filepath.Base(name)
			if name == normalizeAssetPath(path) {
				continue
			}
			if !seen[base] {
				seen[base] = true
				names = append(names, base)
			}
		}
		var entries []os.DirEntry
		readErr := i.Blocking(func() error {
			var dirErr error
			entries, dirErr = os.ReadDir(i.ResolvePath(path))
			return dirErr
		})
		if readErr != nil && len(names) == 0 {
			return nil, fmt.Errorf("list_dir(): %v", readErr)
		}
		for _, e := range entries {
			if !seen[e.Name()] {
				seen[e.Name()] = true
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		out := make([]Value, len(names))
		for idx, name := range names {
			out[idx] = name
		}
		return &List{Items: out}, nil
	})
	reg("mkdir", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		path, err := oneString("mkdir", args)
		if err != nil {
			return nil, err
		}
		path = i.ResolvePath(path)
		if mkErr := i.Blocking(func() error { return os.MkdirAll(path, 0o755) }); mkErr != nil {
			return nil, fmt.Errorf("mkdir(): %v", mkErr)
		}
		return nil, nil
	})
	reg("basename", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		path, err := oneString("basename", args)
		if err != nil {
			return nil, err
		}
		return filepath.Base(path), nil
	})
	reg("dirname", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		path, err := oneString("dirname", args)
		if err != nil {
			return nil, err
		}
		return filepath.Dir(path), nil
	})
	reg("join_path", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		parts := make([]string, len(args))
		for idx, a := range args {
			s, ok := asString(a)
			if !ok {
				return nil, fmt.Errorf("join_path() 的参数必须是字符串")
			}
			parts[idx] = s
		}
		return filepath.Join(parts...), nil
	})
	reg("abs_path", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		path, err := oneString("abs_path", args)
		if err != nil {
			return nil, err
		}
		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			return nil, fmt.Errorf("abs_path(): %v", absErr)
		}
		return abs, nil
	})
	reg("cwd", func(i *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		dir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("cwd(): %v", err)
		}
		return dir, nil
	})
}

func valueBytes(name string, v Value) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case *Bytes:
		return append([]byte(nil), x.Data...), nil
	case string:
		return []byte(x), nil
	case float64:
		n, ok := asInt(x)
		if !ok || n < 0 || n > 255 {
			return nil, fmt.Errorf("%s(): 数字必须是 0 到 255 的字节", name)
		}
		return []byte{byte(n)}, nil
	case *List:
		out := make([]byte, len(x.Items))
		for idx, item := range x.Items {
			n, ok := asInt(item)
			if !ok || n < 0 || n > 255 {
				return nil, fmt.Errorf("%s(): 列表元素必须是 0 到 255 的整数", name)
			}
			out[idx] = byte(n)
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s(): 不能把 %s 转成 bytes", name, TypeName(v))
}

func oneString(name string, args []Value) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("%s() 需要一个参数", name)
	}
	s, ok := asString(args[0])
	if !ok {
		return "", fmt.Errorf("%s(): 参数必须是字符串", name)
	}
	return s, nil
}

func oneNumber(name string, args []Value) (float64, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("%s() 需要一个参数", name)
	}
	f, ok := asNumber(args[0])
	if !ok {
		return 0, fmt.Errorf("%s(): 参数必须是数字", name)
	}
	return f, nil
}

func minMax(args []Value, wantMax bool) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("min/max 至少需要一个参数")
	}
	items := args
	if len(args) == 1 {
		if l, ok := args[0].(*List); ok {
			items = l.Items
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("min/max 不能用于空列表")
	}
	best := items[0]
	for _, v := range items[1:] {
		less, err := compareValues(v, best)
		if err != nil {
			return nil, err
		}
		if (wantMax && less > 0) || (!wantMax && less < 0) {
			best = v
		}
	}
	return best, nil
}

func compareValues(a, b Value) (int, error) {
	if af, ok := asNumber(a); ok {
		bf, ok := asNumber(b)
		if !ok {
			return 0, fmt.Errorf("不能比较数字和 %s", TypeName(b))
		}
		switch {
		case af < bf:
			return -1, nil
		case af > bf:
			return 1, nil
		}
		return 0, nil
	}
	if as, ok := asString(a); ok {
		bs, ok := asString(b)
		if !ok {
			return 0, fmt.Errorf("不能比较字符串和 %s", TypeName(b))
		}
		return strings.Compare(as, bs), nil
	}
	return 0, fmt.Errorf("%s 不能比较大小", TypeName(a))
}

func formatTemplate(tpl string, args []Value, kwargs map[string]Value) (string, error) {
	var b strings.Builder
	next := 0
	runes := []rune(tpl)
	for idx := 0; idx < len(runes); idx++ {
		ch := runes[idx]
		if ch == '{' {
			if idx+1 < len(runes) && runes[idx+1] == '{' {
				b.WriteRune('{')
				idx++
				continue
			}
			end := idx + 1
			for end < len(runes) && runes[end] != '}' {
				end++
			}
			if end >= len(runes) {
				b.WriteString(string(runes[idx:]))
				break
			}
			key := strings.TrimSpace(string(runes[idx+1 : end]))
			if key == "" {
				if next >= len(args) {
					return "", fmt.Errorf("format(): 缺少第 %d 个参数", next+1)
				}
				b.WriteString(ToString(args[next]))
				next++
			} else {
				v, ok := kwargs[key]
				if !ok {
					return "", fmt.Errorf("format(): 没有名为 %q 的参数", key)
				}
				b.WriteString(ToString(v))
			}
			idx = end
			continue
		}
		if ch == '}' && idx+1 < len(runes) && runes[idx+1] == '}' {
			b.WriteRune('}')
			idx++
			continue
		}
		b.WriteRune(ch)
	}
	return b.String(), nil
}
