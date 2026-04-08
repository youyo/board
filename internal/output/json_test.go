package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/youyo/board/internal/output"
)

func TestWriteJSON(t *testing.T) {
	t.Run("マップをコンパクトJSONで出力", func(t *testing.T) {
		var buf bytes.Buffer
		v := map[string]interface{}{"key": "value", "num": 42}
		err := output.WriteJSON(&buf, v)
		if err != nil {
			t.Fatalf("WriteJSON() error = %v", err)
		}
		got := buf.String()
		if !strings.HasSuffix(got, "\n") {
			t.Error("WriteJSON() output should end with newline")
		}
		// コンパクト形式なのでインデントなし
		if strings.Contains(got, "\n  ") {
			t.Error("WriteJSON() should not produce indented output")
		}
	})

	t.Run("nilをJSONで出力", func(t *testing.T) {
		var buf bytes.Buffer
		err := output.WriteJSON(&buf, nil)
		if err != nil {
			t.Fatalf("WriteJSON() error = %v", err)
		}
		got := strings.TrimSpace(buf.String())
		if got != "null" {
			t.Errorf("WriteJSON(nil) = %q, want %q", got, "null")
		}
	})
}

func TestWritePrettyJSON(t *testing.T) {
	t.Run("マップをインデント付きJSONで出力", func(t *testing.T) {
		var buf bytes.Buffer
		v := map[string]interface{}{"key": "value"}
		err := output.WritePrettyJSON(&buf, v)
		if err != nil {
			t.Fatalf("WritePrettyJSON() error = %v", err)
		}
		got := buf.String()
		if !strings.HasSuffix(got, "\n") {
			t.Error("WritePrettyJSON() output should end with newline")
		}
		// インデント形式
		if !strings.Contains(got, "  ") {
			t.Error("WritePrettyJSON() should produce indented output")
		}
	})
}

func TestWrite(t *testing.T) {
	v := map[string]interface{}{"x": 1}

	t.Run("pretty=falseでコンパクトJSON", func(t *testing.T) {
		var buf bytes.Buffer
		err := output.Write(&buf, v, false)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		got := buf.String()
		if strings.Contains(got, "\n  ") {
			t.Error("Write(pretty=false) should not produce indented output")
		}
	})

	t.Run("pretty=trueでインデント付きJSON", func(t *testing.T) {
		var buf bytes.Buffer
		err := output.Write(&buf, v, true)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		got := buf.String()
		if !strings.Contains(got, "  ") {
			t.Error("Write(pretty=true) should produce indented output")
		}
	})
}

func TestPrettyFormat(t *testing.T) {
	t.Run("JSONバイトをインデント付きに整形", func(t *testing.T) {
		input := []byte(`{"a":1,"b":"hello"}`)
		got, err := output.PrettyFormat(input)
		if err != nil {
			t.Fatalf("PrettyFormat() error = %v", err)
		}
		if !strings.Contains(string(got), "  ") {
			t.Error("PrettyFormat() should produce indented output")
		}
	})

	t.Run("不正なJSONはエラーを返す", func(t *testing.T) {
		input := []byte(`{invalid json}`)
		_, err := output.PrettyFormat(input)
		if err == nil {
			t.Error("PrettyFormat() should return error for invalid JSON")
		}
	})
}
