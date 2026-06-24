package yamlcanon

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// DocumentOptions configures human-oriented YAML emission: schema field
// descriptions as comments, schema property order, and blank lines between
// top-level keys.
type DocumentOptions struct {
	Comments           map[string]string
	KeyOrder           map[string][]string
	TopLevelBlankLines bool
}

type docState struct {
	opts            DocumentOptions
	topLevelWritten int
}

func (d *docState) comment(path string) string {
	if d == nil || d.opts.Comments == nil {
		return ""
	}
	return d.opts.Comments[path]
}

func (d *docState) orderedKeys(parentPath string, keys []string) []string {
	if d == nil || d.opts.KeyOrder == nil {
		out := append([]string(nil), keys...)
		sort.Strings(out)
		return out
	}
	order, ok := d.opts.KeyOrder[parentPath]
	if !ok {
		out := append([]string(nil), keys...)
		sort.Strings(out)
		return out
	}
	seen := map[string]bool{}
	var out []string
	for _, k := range order {
		for _, key := range keys {
			if key == k {
				out = append(out, key)
				seen[key] = true
				break
			}
		}
	}
	extras := append([]string(nil), keys...)
	sort.Strings(extras)
	for _, key := range extras {
		if !seen[key] {
			out = append(out, key)
		}
	}
	return out
}

func (d *docState) beforeTopLevelKey(b *bytes.Buffer) {
	if d == nil || !d.opts.TopLevelBlankLines {
		return
	}
	if d.topLevelWritten > 0 {
		b.WriteByte('\n')
	}
	d.topLevelWritten++
}

func Marshal(v any) ([]byte, error) {
	return marshal(v, false)
}

// MarshalPlainDocumented is MarshalPlain with schema-driven comments and layout.
func MarshalPlainDocumented(v any, opts DocumentOptions) ([]byte, error) {
	return marshalDocumented(v, opts)
}

// MarshalPlainDocumentedWithBanner is MarshalPlainDocumented with a prepended banner.
func MarshalPlainDocumentedWithBanner(banner string, v any, opts DocumentOptions) ([]byte, error) {
	body, err := MarshalPlainDocumented(v, opts)
	if err != nil {
		return nil, err
	}
	return withBanner(banner, body), nil
}

func marshalDocumented(v any, opts DocumentOptions) ([]byte, error) {
	var b bytes.Buffer
	doc := &docState{opts: opts}
	if err := write(&b, reflect.ValueOf(v), 0, true, "", doc); err != nil {
		return nil, err
	}
	if b.Len() == 0 || b.Bytes()[b.Len()-1] != '\n' {
		b.WriteByte('\n')
	}
	return b.Bytes(), nil
}

// MarshalPlain is Marshal without explicit scalar tags (!!str etc.): scalars
// render as plain YAML, with strings quoted only when required for safe
// round-tripping. Output remains deterministic and key-sorted. Use it for
// files meant to be read and edited by humans.
func MarshalPlain(v any) ([]byte, error) {
	return marshal(v, true)
}

func marshal(v any, plain bool) ([]byte, error) {
	var b bytes.Buffer
	if err := write(&b, reflect.ValueOf(v), 0, plain, "", nil); err != nil {
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
	return withBanner(banner, body), nil
}

// MarshalPlainWithBanner is MarshalPlain with a prepended comment banner.
func MarshalPlainWithBanner(banner string, v any) ([]byte, error) {
	body, err := MarshalPlain(v)
	if err != nil {
		return nil, err
	}
	return withBanner(banner, body), nil
}

func withBanner(banner string, body []byte) []byte {
	var b bytes.Buffer
	for _, line := range strings.Split(strings.TrimSuffix(banner, "\n"), "\n") {
		b.WriteString("# ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.Write(body)
	return b.Bytes()
}

func write(b *bytes.Buffer, v reflect.Value, depth int, plain bool, path string, doc *docState) error {
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
		names := make([]string, len(keys))
		for i, k := range keys {
			names[i] = k.String()
		}
		for _, name := range doc.orderedKeys(path, names) {
			fieldPath := joinPath(path, name)
			if err := writeEntry(b, name, fieldPath, v.MapIndex(reflect.ValueOf(name)), depth, plain, doc); err != nil {
				return err
			}
		}
	case reflect.Struct:
		fields := exportedFields(v.Type())
		names := make([]string, len(fields))
		for i, f := range fields {
			names[i] = f.name
		}
		order := doc.orderedKeys(path, names)
		byName := map[string]fieldInfo{}
		for _, f := range fields {
			byName[f.name] = f
		}
		for _, name := range order {
			f, ok := byName[name]
			if !ok {
				continue
			}
			fv := v.FieldByIndex(f.index)
			if f.omitEmpty && isZero(fv) {
				continue
			}
			fieldPath := joinPath(path, f.name)
			if err := writeEntry(b, f.name, fieldPath, fv, depth, plain, doc); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			b.WriteString("[]")
			return nil
		}
		for i := 0; i < v.Len(); i++ {
			item := v.Index(i)
			indent(b, depth)
			if !inline(item) && doc != nil && structLike(item) {
				b.WriteString("-\n")
				var sub bytes.Buffer
				if err := write(&sub, item, depth+1, plain, path, doc); err != nil {
					return err
				}
				body := sub.Bytes()
				if startsWithCommentBlock(body) {
					b.WriteByte('\n')
				}
				b.Write(body)
				continue
			}
			b.WriteString("- ")
			if inline(item) {
				if err := write(b, item, depth, plain, path, doc); err != nil {
					return err
				}
				b.WriteByte('\n')
				continue
			}
			var sub bytes.Buffer
			if err := write(&sub, item, depth+1, plain, path, doc); err != nil {
				return err
			}
			body := sub.Bytes()
			prefix := strings.Repeat("  ", depth+1)
			if bytes.HasPrefix(body, []byte(prefix)) {
				b.Write(body[len(prefix):])
				continue
			}
			b.WriteByte('\n')
			b.Write(body)
		}
	case reflect.String:
		writeString(b, v.String(), plain)
	case reflect.Bool:
		writeScalar(b, "!!bool", strconv.FormatBool(v.Bool()), plain)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		writeScalar(b, "!!int", strconv.FormatInt(v.Int(), 10), plain)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		writeScalar(b, "!!int", strconv.FormatUint(v.Uint(), 10), plain)
	case reflect.Float32, reflect.Float64:
		writeScalar(b, "!!float", strconv.FormatFloat(v.Float(), 'f', -1, 64), plain)
	case reflect.Invalid:
		writeScalar(b, "!!null", "null", plain)
	default:
		return fmt.Errorf("yamlcanon: unsupported kind %s", v.Kind())
	}
	return nil
}

func writeEntry(b *bytes.Buffer, key, path string, val reflect.Value, depth int, plain bool, doc *docState) error {
	if depth == 0 {
		doc.beforeTopLevelKey(b)
	}
	if c := doc.comment(path); c != "" {
		ensureBlankBeforeComment(b)
		writeCommentBlock(b, depth, c)
	}
	indent(b, depth)
	b.WriteString(key)
	if inline(val) {
		b.WriteString(": ")
		if err := write(b, val, depth, plain, path, doc); err != nil {
			return err
		}
		b.WriteByte('\n')
		return nil
	}
	b.WriteString(":\n")
	return write(b, val, depth+1, plain, path, doc)
}

func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func writeCommentBlock(b *bytes.Buffer, depth int, text string) {
	prefix := strings.Repeat("  ", depth)
	for _, line := range wrapComment(text, 76) {
		b.WriteString(prefix)
		b.WriteString("# ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
}

// ensureBlankBeforeComment inserts a separating blank line when the buffer
// already has output and is not already preceded by one.
func ensureBlankBeforeComment(b *bytes.Buffer) {
	if b.Len() == 0 {
		return
	}
	data := b.Bytes()
	n := len(data)
	if n >= 2 && data[n-1] == '\n' && data[n-2] == '\n' {
		return
	}
	if data[n-1] == '\n' {
		b.WriteByte('\n')
	}
}

func startsWithCommentBlock(data []byte) bool {
	for _, c := range data {
		switch c {
		case ' ', '\t':
			continue
		case '#':
			return true
		default:
			return false
		}
	}
	return false
}

func wrapComment(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var cur strings.Builder
	for _, w := range words {
		if cur.Len() == 0 {
			cur.WriteString(w)
			continue
		}
		if cur.Len()+1+len(w) > width {
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(w)
			continue
		}
		cur.WriteByte(' ')
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

// inline reports whether the value renders on the same line as its key:
// scalars always, and empty sequences (rendered as []).
func inline(v reflect.Value) bool {
	if isScalar(v) {
		return true
	}
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	if v.IsValid() && (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) && v.Len() == 0 {
		return true
	}
	return false
}

func writeScalar(b *bytes.Buffer, tag, text string, plain bool) {
	if !plain {
		b.WriteString(tag)
		b.WriteByte(' ')
	}
	b.WriteString(text)
}

func writeString(b *bytes.Buffer, s string, plain bool) {
	if !plain {
		b.WriteString("!!str ")
		b.WriteString(strconv.Quote(s))
		return
	}
	if plainStringSafe(s) {
		b.WriteString(s)
		return
	}
	b.WriteString(strconv.Quote(s))
}

// plainStringSafe reports whether s can render unquoted in plain mode without
// changing meaning on re-parse. Deliberately conservative: anything that
// could read as a number, bool, null, date, or YAML syntax gets quoted.
func plainStringSafe(s string) bool {
	switch s {
	case "", "true", "false", "null", "yes", "no", "on", "off", "~":
		return false
	}
	if c := s[0]; c == '-' || c == '+' || c == ':' || (c >= '0' && c <= '9') {
		return false
	}
	if s[len(s)-1] == ':' {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool { return !plainSafeRune(r) })
}

// plainSafeRune whitelists characters that never need quoting inside a plain
// scalar. ':' is safe in non-edge positions because plain scalars only treat
// it specially when adjacent to whitespace, which is excluded entirely;
// plainStringSafe rejects leading/trailing ':' separately.
func plainSafeRune(r rune) bool {
	isAlnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
	return isAlnum || strings.ContainsRune("-._/:", r)
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

func structLike(v reflect.Value) bool {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return false
	}
	switch v.Kind() {
	case reflect.Struct, reflect.Map:
		return true
	default:
		return false
	}
}

func indent(b *bytes.Buffer, depth int) {
	b.WriteString(strings.Repeat("  ", depth))
}
