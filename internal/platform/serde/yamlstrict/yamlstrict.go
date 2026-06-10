package yamlstrict

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

func NormalizeYAML(v any) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			n, err := NormalizeYAML(v)
			if err != nil {
				return nil, err
			}
			out[k] = n
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			ks, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string YAML key %v", k)
			}
			n, err := NormalizeYAML(v)
			if err != nil {
				return nil, err
			}
			out[ks] = n
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			n, err := NormalizeYAML(v)
			if err != nil {
				return nil, err
			}
			out[i] = n
		}
		return out, nil
	default:
		return x, nil
	}
}

func DecodeStrict(data []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(out)
}

func DecodeMap(data []byte) (map[string]any, error) {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	n, err := NormalizeYAML(raw)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return map[string]any{}, nil
	}
	m, ok := n.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("document root must be an object")
	}
	return m, nil
}
