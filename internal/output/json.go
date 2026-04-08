package output

import (
	"encoding/json"
	"io"
)

// WriteJSON writes v to w as compact JSON (with a trailing newline).
func WriteJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// WritePrettyJSON writes v to w as indented JSON (with a trailing newline).
func WritePrettyJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Write calls WriteJSON or WritePrettyJSON depending on the pretty flag.
func Write(w io.Writer, v interface{}, pretty bool) error {
	if pretty {
		return WritePrettyJSON(w, v)
	}
	return WriteJSON(w, v)
}
