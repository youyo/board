package output

import (
	"encoding/json"
	"io"
)

// WriteJSON は v をコンパクト JSON として w に書き込む（末尾に改行あり）。
func WriteJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// WritePrettyJSON は v をインデント付き JSON として w に書き込む（末尾に改行あり）。
func WritePrettyJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Write は pretty フラグに応じて WriteJSON または WritePrettyJSON を呼ぶ。
func Write(w io.Writer, v interface{}, pretty bool) error {
	if pretty {
		return WritePrettyJSON(w, v)
	}
	return WriteJSON(w, v)
}
