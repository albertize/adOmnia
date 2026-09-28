package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

func BenchmarkExtensionManifestDiscovery100(b *testing.B) {
	manifests := make([][]byte, 100)
	for index := range manifests {
		manifest := validTestManifest()
		manifest.ID = fmt.Sprintf("benchmark.extension-%d", index)
		manifest.Name = fmt.Sprintf("Benchmark %d", index)
		manifest.Publisher = "benchmark"
		manifest.Contributes.Commands[0].ID = manifest.ID + ".run"
		manifest.ActivationEvents[0] = "onCommand:" + manifest.ID + ".run"
		manifests[index], _ = json.Marshal(manifest)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		for _, data := range manifests {
			manifest, err := DecodeManifest(data)
			if err != nil || hasErrors(ValidateManifest(manifest)) {
				b.Fatalf("invalid benchmark manifest: %v", err)
			}
		}
	}
}

func BenchmarkDeclarativeViewValidation1000Rows(b *testing.B) {
	rows := make([]any, 1000)
	for index := range rows {
		rows[index] = map[string]any{"id": index, "status": "ok", "durationMs": index % 100}
	}
	state := map[string]any{
		"kind":  "table",
		"title": "Performance",
		"columns": []any{
			map[string]any{"key": "id", "title": "ID"},
			map[string]any{"key": "status", "title": "Status"},
			map[string]any{"key": "durationMs", "title": "Duration"},
		},
		"rows": rows,
	}
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		if err := ValidateDeclarativeViewState(state); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHostEventDispatch(b *testing.B) {
	client, closeHost := newBenchmarkHost(b)
	defer closeHost()
	manifest := validTestManifest()
	manifest.Permissions = []string{"responses.read"}
	manifest.ActivationEvents = append(manifest.ActivationEvents, "onResponse")
	source := `export function activate(api) { api.events.onResponse(() => undefined); api.commands.registerCommand('test.extension.run', () => undefined) }`
	var activation HostExecutionResult
	if err := client.Request(context.Background(), "activate", ActivateHostRequest{Manifest: manifest, Source: source}, &activation); err != nil || !activation.Success {
		b.Fatalf("activate: result=%#v err=%v", activation, err)
	}
	request := DispatchEventRequest{ExtensionID: manifest.ID, Event: "onResponse", Payload: map[string]any{"status": 200}}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		var result HostExecutionResult
		if err := client.Request(context.Background(), "dispatchEvent", request, &result); err != nil || !result.Success {
			b.Fatalf("dispatch: result=%#v err=%v", result, err)
		}
	}
}

func BenchmarkExtensionHostColdStart(b *testing.B) {
	executable := os.Getenv("ADOMNIA_EXTENSION_PERF_EXECUTABLE")
	if executable == "" {
		b.Skip("set ADOMNIA_EXTENSION_PERF_EXECUTABLE to a production adOmnia binary")
	}
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		host, err := StartHostProcess(executable, nil)
		if err != nil {
			b.Fatal(err)
		}
		if err := host.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func newBenchmarkHost(b *testing.B) (*HostClient, func()) {
	b.Helper()
	token := "0123456789abcdef0123456789abcdef"
	b.Setenv("ADOMNIA_EXTENSION_HOST_TOKEN", token)
	parentInput, childOutput := io.Pipe()
	childInput, parentOutput := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- RunExtensionHost(childInput, childOutput) }()
	client := NewHostClient(parentInput, parentOutput, token, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var initialized InitializeHostResult
	if err := client.Request(ctx, "initialize", InitializeHostRequest{ProtocolVersion: HostProtocolVersion}, &initialized); err != nil {
		b.Fatal(err)
	}
	return client, func() {
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			b.Error("benchmark host did not stop")
		}
	}
}
