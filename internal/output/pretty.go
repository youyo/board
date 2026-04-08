package output

import (
	"bytes"
	"encoding/json"
)

// PrettyFormat は JSON バイト列をインデント付きに整形して返す。
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
	// Encode は末尾に \n を付けるので、スライスとして返す
	return buf.Bytes(), nil
}
