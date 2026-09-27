package extensions

import "adomnia/internal/plugins"

// LegacyAdapter keeps v1 execution behind the extension orchestration boundary
// without changing its manifest, storage, JavaScript globals, or Wails manager.
// It is deliberately an adapter rather than an automatic v1-to-v2 conversion.
type LegacyAdapter struct{ manager *plugins.PluginManager }

func NewLegacyAdapter(manager *plugins.PluginManager) *LegacyAdapter {
	if manager == nil {
		return nil
	}
	return &LegacyAdapter{manager: manager}
}

func (a *LegacyAdapter) Init() error { return a.manager.Init() }

func (a *LegacyAdapter) ApplyEventJSON(event, payloadJSON string) (string, error) {
	return a.manager.ApplyEventJSON(event, payloadJSON)
}

func (a *LegacyAdapter) FireStartup() {
	a.manager.FireEvent(plugins.PluginEvent{Type: "onStartup", Payload: map[string]interface{}{}})
}

func (a *LegacyAdapter) Shutdown() {
	a.manager.EmitEvent(plugins.PluginEvent{Type: "onShutdown", Payload: map[string]interface{}{}})
	a.manager.Shutdown()
}
