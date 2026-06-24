package jsonschema

import (
	"bytes"
	"encoding/json"
)

// Meta holds schema metadata used when emitting documented default YAML:
// field descriptions keyed by dotted paths (for example "project.name") and
// property key order keyed by parent path ("" for the document root).
type Meta struct {
	Descriptions map[string]string
	KeyOrder     map[string][]string
}

// DocumentMeta extracts descriptions and property order from schemaJSON.
func DocumentMeta(schemaJSON []byte) (*Meta, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(schemaJSON, &root); err != nil {
		return nil, err
	}
	meta := &Meta{
		Descriptions: map[string]string{},
		KeyOrder:     map[string][]string{},
	}
	props, ok := root["properties"]
	if !ok {
		return meta, nil
	}
	if err := walkSchemaProps(props, "", meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func walkSchemaProps(propsRaw json.RawMessage, prefix string, meta *Meta) error {
	names, err := orderedObjectKeys(propsRaw)
	if err != nil {
		return err
	}
	meta.KeyOrder[prefix] = names
	for _, name := range names {
		childRaw, err := rawObjectField(propsRaw, name)
		if err != nil {
			return err
		}
		path := joinDocPath(prefix, name)
		if err := recordDescription(childRaw, path, meta); err != nil {
			return err
		}
		if err := walkNestedSchema(childRaw, path, meta); err != nil {
			return err
		}
	}
	return nil
}

func joinDocPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func recordDescription(nodeRaw json.RawMessage, path string, meta *Meta) error {
	var node map[string]json.RawMessage
	if err := json.Unmarshal(nodeRaw, &node); err != nil {
		return err
	}
	descRaw, ok := node["description"]
	if !ok {
		return nil
	}
	var desc string
	if err := json.Unmarshal(descRaw, &desc); err != nil {
		return err
	}
	if desc != "" {
		meta.Descriptions[path] = desc
	}
	return nil
}

func walkNestedSchema(nodeRaw json.RawMessage, path string, meta *Meta) error {
	var node map[string]json.RawMessage
	if err := json.Unmarshal(nodeRaw, &node); err != nil {
		return err
	}
	if props := node["properties"]; props != nil {
		return walkSchemaProps(props, path, meta)
	}
	items := node["items"]
	if items == nil {
		return nil
	}
	var itemNode map[string]json.RawMessage
	if err := json.Unmarshal(items, &itemNode); err != nil {
		return err
	}
	if itemNode["properties"] == nil {
		return nil
	}
	return walkSchemaProps(itemNode["properties"], path, meta)
}

func orderedObjectKeys(obj json.RawMessage) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(obj))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, err
	}
	var names []string
	for dec.More() {
		tok, err = dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			continue
		}
		names = append(names, key)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return names, nil
}

func rawObjectField(obj json.RawMessage, field string) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(obj))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, err
	}
	for dec.More() {
		tok, err = dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			continue
		}
		var val json.RawMessage
		if err := dec.Decode(&val); err != nil {
			return nil, err
		}
		if key == field {
			return val, nil
		}
	}
	return nil, nil
}
