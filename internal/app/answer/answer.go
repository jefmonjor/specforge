// Package answer reads the structured part of an agent's answer: the JSON
// object it must end with, validated against an embedded JSON Schema. An
// answer that does not validate is not "almost right": the caller retries
// once and then fails closed.
package answer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"specforge/assets"
	"specforge/internal/jsontext"
)

// Schema is a compiled JSON Schema from the embedded assets.
type Schema struct {
	name   string
	schema *jsonschema.Schema
}

var (
	mu    sync.Mutex
	cache = map[string]*Schema{}
)

// Load compiles the schema at path in the embedded assets, once.
func Load(path string) (*Schema, error) {
	mu.Lock()
	defer mu.Unlock()
	if s, ok := cache[path]; ok {
		return s, nil
	}
	data, err := fs.ReadFile(assets.FS, path)
	if err != nil {
		return nil, fmt.Errorf("reading schema %s: %w", path, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parsing schema %s: %w", path, err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(path, doc); err != nil {
		return nil, fmt.Errorf("loading schema %s: %w", path, err)
	}
	compiled, err := c.Compile(path)
	if err != nil {
		return nil, fmt.Errorf("compiling schema %s: %w", path, err)
	}
	s := &Schema{name: path, schema: compiled}
	cache[path] = s
	return s, nil
}

// Decode stores into v the last JSON object of text that validates, and
// reports whether there was one.
func (s *Schema) Decode(text string, v any) bool {
	for _, c := range jsontext.Candidates(text) {
		inst, err := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(c)))
		if err != nil || s.schema.Validate(inst) != nil {
			continue
		}
		return json.Unmarshal([]byte(c), v) == nil
	}
	return false
}
