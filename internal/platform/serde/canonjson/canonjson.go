package canonjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

func Marshal(v any) ([]byte, error) {
	var normalized any
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &normalized); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := writeValue(&out, normalized); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	b, err := Marshal(v)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, b, prefix, indent); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeValue(out *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		if x {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case string:
		b, _ := json.Marshal(x)
		out.Write(b)
	case float64:
		out.WriteString(strconv.FormatFloat(x, 'f', -1, 64))
	case []any:
		out.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeValue(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			key, _ := json.Marshal(k)
			out.Write(key)
			out.WriteByte(':')
			if err := writeValue(out, x[k]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return fmt.Errorf("unsupported normalized JSON type %T", v)
	}
	return nil
}
