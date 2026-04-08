package output

import (
	"bytes"
	"encoding/json"
)

// PrettyFormat formats a JSON byte slice with indentation and returns the result.
func PrettyFormat(data []byte) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encode appends a trailing \n, so return as a slice.
	return buf.Bytes(), nil
}
