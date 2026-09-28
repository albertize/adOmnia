package proxy

import (
	"testing"
	"time"
)

func TestExtensionSnapshotAndObserverUseRedactedCanonicalTraffic(t *testing.T) {
	ExtensionClearTraffic()
	t.Cleanup(func() { ConfigureExtensionObserver(nil); ExtensionClearTraffic() })
	received := make(chan map[string]any, 1)
	ConfigureExtensionObserver(func(payload map[string]any) { received <- payload })
	recordEntry("https://example.test", "GET", map[string]string{"Authorization": "secret", "Accept": "application/json"}, "", 200, map[string]string{}, "ok", time.Millisecond, "", false)
	select {
	case payload := <-received:
		headers := payload["reqHeaders"].(map[string]string)
		if headers["Authorization"] != "***redacted***" || payload["url"] != "https://example.test" {
			t.Fatalf("payload=%#v", payload)
		}
	default:
		t.Fatal("proxy observer was not notified")
	}
	snapshot := ExtensionSnapshot()
	entries, ok := snapshot["entries"].([]trafficEntry)
	if !ok || len(entries) != 1 || entries[0].ReqHeaders["Authorization"] != "***redacted***" {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	ExtensionClearTraffic()
	if entries := ExtensionSnapshot()["entries"].([]trafficEntry); len(entries) != 0 {
		t.Fatalf("traffic was not cleared: %#v", entries)
	}
}
