package mock

import "testing"

func TestExtensionSnapshotAndObserverUseCanonicalMockState(t *testing.T) {
	ExtensionClearHits()
	t.Cleanup(func() { ConfigureExtensionObserver(nil); ExtensionClearHits() })
	received := make(chan map[string]any, 1)
	ConfigureExtensionObserver(func(payload map[string]any) { received <- payload })
	recordHitDetailed("GET", "/health", true, "response-1", 200, mockHitDetails{EndpointID: "endpoint-1"})
	select {
	case payload := <-received:
		if payload["path"] != "/health" || payload["status"] != 200 {
			t.Fatalf("payload=%#v", payload)
		}
	default:
		t.Fatal("mock observer was not notified")
	}
	snapshot := ExtensionSnapshot()
	hits, ok := snapshot["hits"].([]mockHitEntry)
	if !ok || len(hits) != 1 || hits[0].Path != "/health" {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	ExtensionClearHits()
	if hits := ExtensionSnapshot()["hits"].([]mockHitEntry); len(hits) != 0 {
		t.Fatalf("hits were not cleared: %#v", hits)
	}
}
