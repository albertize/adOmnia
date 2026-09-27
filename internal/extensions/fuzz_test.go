package extensions

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func FuzzDecodeAndValidateManifest(f *testing.F) {
	f.Add([]byte(`{"manifestVersion":2,"id":"test.extension"}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"permissions":["network:https://example.com"]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		manifest, err := DecodeManifest(data)
		if err == nil {
			_ = ValidateManifest(manifest)
		}
	})
}

func FuzzExtractExtensionArchive(f *testing.F) {
	var valid bytes.Buffer
	writer := zip.NewWriter(&valid)
	entry, _ := writer.Create(ManifestFileName)
	_, _ = entry.Write([]byte(`{"manifestVersion":2}`))
	_ = writer.Close()
	f.Add(valid.Bytes())
	f.Add([]byte("not a zip"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2<<20 {
			t.Skip()
		}
		root := t.TempDir()
		archive := filepath.Join(root, "input.adomnia-extension")
		if err := os.WriteFile(archive, data, 0600); err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(root, "output")
		_ = os.MkdirAll(destination, 0700)
		_ = extractArchive(archive, destination)
	})
}

func FuzzRPCMessageJSON(f *testing.F) {
	f.Add([]byte(`{"jsonrpc":"2.0","id":"1","method":"initialize","token":"token"}`))
	f.Add([]byte(`{"error":{"code":-32601,"message":"missing"}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxRPCMessageBytes {
			t.Skip()
		}
		var message rpcMessage
		if json.Unmarshal(data, &message) == nil {
			_, _ = json.Marshal(message)
		}
	})
}

func FuzzWhenClauseValidation(f *testing.F) {
	f.Add("activeTool == 'collections' && hasResponse")
	f.Add("globalThis.alert()")
	f.Add("unterminated == '")
	f.Fuzz(func(t *testing.T, expression string) { _ = validateWhenClause(expression) })
}
