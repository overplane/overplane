package yamlcanon

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	if err := write(&b, reflect.ValueOf(v), 0); err != nil {
		return nil, err
	}
	if b.Len() == 0 || b.Bytes()[b.Len()-1] != '\n' {
		b.WriteByte('\n')
	}
	return b.Bytes(), nil
}

func MarshalWithBanner(banner string, v any) ([]byte, error) {
	body, err := Marshal(v)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	for _, line := range strings.Split(strings.TrimSuffix(banner, "\n"), "\n") {
		b.WriteString("# ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.Write(body)
	return b.Bytes(), nil
}

func write(b *bytes.Buffer, v reflect.Value, depth int) error {
	if !v.IsValid() {
		b.WriteString("!!null null")
		return nil
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			b.WriteString("!!null null")
			return nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("yamlcanon: unsupported non-string map key %s", v.Type().Key())
		}
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		for _, k := range keys {
			indent(b, depth)
			b.WriteString(k.String())
			b.WriteString(": ")
			val := v.MapIndex(k)
			if isScalar(val) {
				if err := write(b, val, depth); err != nil {
					return err
				}
				b.WriteByte('\n')
				continue
			}
			b.WriteByte('\n')
			if err := write(b, val, depth+1); err != nil {
				return err
			}
		}
	case reflect.Struct:
		fields := exportedFields(v.Type())
		sort.Slice(fields, func(i, j int) bool { return fields[i].name < fields[j].name })
		for _, f := range fields {
			fv := v.FieldByIndex(f.index)
			if f.omitEmpty && isZero(fv) {
				continue
			}
			indent(b, depth)
			b.WriteString(f.name)
			b.WriteString(": ")
			if isScalar(fv) {
				if err := write(b, fv, depth); err != nil {
					return err
				}
				b.WriteByte('\n')
				continue
			}
			b.WriteByte('\n')
			if err := write(b, fv, depth+1); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			indent(b, depth)
			b.WriteString("- ")
			item := v.Index(i)
			if isScalar(item) {
				if err := write(b, item, depth); err != nil {
					return err
				}
				b.WriteByte('\n')
				continue
			}
			b.WriteByte('\n')
			if err := write(b, item, depth+1); err != nil {
				return err
			}
		}
	case reflect.String:
		b.WriteString("!!str ")
		b.WriteString(strconv.Quote(v.String()))
	case reflect.Bool:
		b.WriteString("!!bool ")
		b.WriteString(strconv.FormatBool(v.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b.WriteString("!!int ")
		b.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		b.WriteString("!!int ")
		b.WriteString(strconv.FormatUint(v.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		b.WriteString("!!float ")
		b.WriteString(strconv.FormatFloat(v.Float(), 'f', -1, 64))
	case reflect.Invalid:
		b.WriteString("!!null null")
	default:
		return fmt.Errorf("yamlcanon: unsupported kind %s", v.Kind())
	}
	return nil
}

type fieldInfo struct {
	name      string
	index     []int
	omitEmpty bool
}

func exportedFields(t reflect.Type) []fieldInfo {
	var out []fieldInfo
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		tag := f.Tag.Get("yaml")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		out = append(out, fieldInfo{name: name, index: f.Index, omitEmpty: strings.Contains(opts, "omitempty")})
	}
	return out
}

func isScalar(v reflect.Value) bool {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func isZero(v reflect.Value) bool {
	return !v.IsValid() || v.IsZero()
}

func indent(b *bytes.Buffer, depth int) {
	b.WriteString(strings.Repeat("  ", depth))
}
