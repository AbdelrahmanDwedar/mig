// Package yamlconv converts between YAML and JSON byte representations,
// letting YAML migrations and templates reuse the existing JSON-based
// sqlgen op machinery unchanged.
package yamlconv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// YAMLToJSON converts a YAML document to its equivalent JSON bytes. Empty or
// whitespace-only input converts to JSON "null" without error; callers that
// require a non-null top-level object should check for that themselves.
// Multi-document YAML (a second "---" separated document) is rejected.
func YAMLToJSON(data []byte) ([]byte, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))

	var v interface{}
	if err := dec.Decode(&v); err != nil {
		if errors.Is(err, io.EOF) {
			v = nil
		} else {
			return nil, err
		}
	}

	var extra interface{}
	if err := dec.Decode(&extra); err == nil {
		return nil, fmt.Errorf("multi-document YAML migration files are not supported")
	} else if !errors.Is(err, io.EOF) {
		return nil, err
	}

	return json.Marshal(v)
}

// JSONToYAML converts JSON bytes to their equivalent YAML representation.
func JSONToYAML(data []byte) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return yaml.Marshal(v)
}
