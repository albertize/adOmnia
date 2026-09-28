package mock

import (
	"context"
	"sync"
	"time"
)

var (
	extensionObserverMu sync.RWMutex
	extensionObserver   func(map[string]any)
)

// ConfigureExtensionObserver connects bounded mock hit notifications to the
// extension broker without making the mock runtime depend on it.
func ConfigureExtensionObserver(observer func(map[string]any)) {
	extensionObserverMu.Lock()
	extensionObserver = observer
	extensionObserverMu.Unlock()
}

func notifyExtensionObserver(entry mockHitEntry) {
	extensionObserverMu.RLock()
	observer := extensionObserver
	extensionObserverMu.RUnlock()
	if observer == nil {
		return
	}
	observer(map[string]any{
		"id": entry.ID, "timestamp": entry.Timestamp, "method": entry.Method,
		"path": entry.Path, "matched": entry.Matched, "responseId": entry.ResponseID,
		"endpointId": entry.EndpointID, "endpointPath": entry.EndpointPath,
		"responseName": entry.ResponseName, "reason": entry.Reason, "status": entry.Status,
	})
}

// ExtensionSnapshot returns a bounded copy of the canonical mock runtime state.
func ExtensionSnapshot() map[string]any {
	mockSrvMu.Lock()
	running, port := mockSrv != nil, mockSrvPort
	mockSrvMu.Unlock()
	mockCfgMu.RLock()
	endpoints := append([]mockEndpoint(nil), mockCfg.Endpoints...)
	mockCfgMu.RUnlock()
	mockHitsMu.Lock()
	hits := append([]mockHitEntry(nil), mockHits...)
	mockHitsMu.Unlock()
	return map[string]any{"running": running, "port": port, "endpoints": endpoints, "hits": hits}
}

// ExtensionClearHits clears only captured hit history, not endpoint definitions.
func ExtensionClearHits() {
	mockHitsMu.Lock()
	mockHits = nil
	mockHitsMu.Unlock()
}

// ExtensionStop stops the active local mock listener.
func ExtensionStop() error {
	mockSrvMu.Lock()
	defer mockSrvMu.Unlock()
	if mockSrv == nil {
		return nil
	}
	if mockSrvLn != nil {
		_ = mockSrvLn.Close()
		mockSrvLn = nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := mockSrv.Shutdown(ctx)
	mockSrv = nil
	mockSrvPort = 0
	return err
}
