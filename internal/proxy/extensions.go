package proxy

import "sync"

var (
	extensionObserverMu sync.RWMutex
	extensionObserver   func(map[string]any)
)

// ConfigureExtensionObserver connects bounded, redacted traffic notifications
// to the extension broker without coupling the proxy runtime to it.
func ConfigureExtensionObserver(observer func(map[string]any)) {
	extensionObserverMu.Lock()
	extensionObserver = observer
	extensionObserverMu.Unlock()
}

func notifyExtensionObserver(entry trafficEntry) {
	extensionObserverMu.RLock()
	observer := extensionObserver
	extensionObserverMu.RUnlock()
	if observer == nil {
		return
	}
	observer(map[string]any{
		"id": entry.ID, "timestamp": entry.Timestamp, "method": entry.Method,
		"url": entry.URL, "reqHeaders": entry.ReqHeaders, "reqBody": entry.ReqBody,
		"status": entry.Status, "respHeaders": entry.RespHeaders, "respBody": entry.RespBody,
		"durationMs": entry.DurationMs, "error": entry.Error, "matched": entry.Matched,
		"timing": entry.Timing,
	})
}

// ExtensionSnapshot returns the canonical bounded traffic and runtime status.
func ExtensionSnapshot() map[string]any {
	interceptProxyMu.Lock()
	running, port := interceptProxy != nil, interceptProxyPort
	interceptProxyMu.Unlock()
	trafficMu.RLock()
	entries := append([]trafficEntry(nil), trafficLog...)
	trafficMu.RUnlock()
	caMu.RLock()
	caExists := caCertPEM != nil
	caMu.RUnlock()
	httpsEnableMu.RLock()
	httpsOn := httpsEnabled
	httpsEnableMu.RUnlock()
	return map[string]any{"running": running, "port": port, "entries": entries, "count": len(entries), "caExists": caExists, "httpsEnabled": httpsOn}
}

// ExtensionClearTraffic clears captured traffic without changing proxy rules.
func ExtensionClearTraffic() {
	trafficMu.Lock()
	trafficLog = nil
	trafficMu.Unlock()
}

// ExtensionStop stops interception and releases pending breakpoints.
func ExtensionStop() error {
	interceptProxyMu.Lock()
	defer interceptProxyMu.Unlock()
	if interceptProxy == nil {
		return nil
	}
	clearPendingBreakpoints()
	err := interceptProxy.Close()
	interceptProxy = nil
	interceptProxyPort = 0
	return err
}
