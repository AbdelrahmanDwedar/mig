package yamlconv_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/yamlconv"
)

func TestYAMLToJSON_BasicRoundTrip(t *testing.T) {
	yamlDoc := "up:\n  - op: create_table\n    table: users\ndown: []\n"
	jsonBytes, err := yamlconv.YAMLToJSON([]byte(yamlDoc))
	if err != nil {
		t.Fatal(err)
	}

	var v map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &v); err != nil {
		t.Fatalf("expected valid JSON with string keys, got unmarshal error: %v", err)
	}
	up, ok := v["up"].([]interface{})
	if !ok || len(up) != 1 {
		t.Fatalf("expected up to be a one-element array, got %#v", v["up"])
	}
	op, ok := up[0].(map[string]interface{})
	if !ok || op["table"] != "users" {
		t.Errorf("expected nested map with string keys, got %#v", up[0])
	}
}

func TestYAMLToJSON_RejectsMultiDocument(t *testing.T) {
	if _, err := yamlconv.YAMLToJSON([]byte("a: 1\n---\nb: 2\n")); err == nil {
		t.Fatal("expected error for multi-document YAML, got nil")
	}
}

func TestYAMLToJSON_MalformedSecondDocument(t *testing.T) {
	if _, err := yamlconv.YAMLToJSON([]byte("a: 1\n---\n\tbad: [\n")); err == nil {
		t.Fatal("expected error for malformed second YAML document, got nil")
	}
}

func TestYAMLToJSON_InvalidYAML(t *testing.T) {
	if _, err := yamlconv.YAMLToJSON([]byte("up:\n\tbad indentation\n")); err == nil {
		t.Fatal("expected error for invalid YAML syntax, got nil")
	}
}

func TestYAMLToJSON_EmptyContent(t *testing.T) {
	jsonBytes, err := yamlconv.YAMLToJSON([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if string(jsonBytes) != "null" {
		t.Errorf("expected empty YAML to convert to JSON null, got %q", jsonBytes)
	}
}

func TestJSONToYAML_InvalidJSON(t *testing.T) {
	if _, err := yamlconv.JSONToYAML([]byte("{not json")); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestJSONToYAML_BasicRoundTrip(t *testing.T) {
	original := []byte(`{"up":[{"op":"create_table","table":"users"}],"down":[]}`)

	yamlBytes, err := yamlconv.JSONToYAML(original)
	if err != nil {
		t.Fatal(err)
	}

	roundTripped, err := yamlconv.YAMLToJSON(yamlBytes)
	if err != nil {
		t.Fatal(err)
	}

	var want, got interface{}
	if err := json.Unmarshal(original, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(roundTripped, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("round trip mismatch:\ngot:  %#v\nwant: %#v", got, want)
	}
}
