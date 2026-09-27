package adomniacli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"adomnia/internal/extensions"
	"adomnia/internal/httpexec"
	extensionsdk "adomnia/sdk/extensions"
)

type extensionTestResult struct {
	OK            bool                               `json:"ok"`
	Command       string                             `json:"command"`
	SDKVersion    string                             `json:"sdkVersion"`
	ExtensionID   string                             `json:"extensionId"`
	Activation    extensions.HostExecutionResult     `json:"activation"`
	Execution     *extensions.HostExecutionResult    `json:"execution,omitempty"`
	Event         *extensions.HostExecutionResult    `json:"event,omitempty"`
	Notifications []extensions.ExtensionNotification `json:"notifications"`
	Views         map[string]interface{}             `json:"views"`
	State         map[string]interface{}             `json:"state"`
	Error         string                             `json:"error,omitempty"`
}

func extensionTest(args []string, stdout, stderr io.Writer) int {
	flagArgs, target, err := splitExtensionTarget(args, map[string]bool{"command": true, "event": true, "payload": true}, map[string]bool{"json": true})
	if err != nil {
		fmt.Fprintf(stderr, "adomnia extension test: %v\n", err)
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension test", flag.ContinueOnError)
	fs.SetOutput(stderr)
	commandID := fs.String("command", "", "execute one contributed command after activation")
	eventName := fs.String("event", "", "dispatch one declared event after activation")
	payloadJSON := fs.String("payload", "{}", "JSON object passed to command or event")
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if target == "" {
		target = "."
	}
	report := extensions.CheckDirectory(target)
	if !report.Valid {
		if *asJSON {
			writeExtensionJSON(stdout, extensionResult{OK: false, Command: "test", SDKVersion: extensionsdk.Version, Validation: &report, Error: "extension validation failed"})
		} else {
			writeValidationText(stdout, report)
		}
		return 1
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(*payloadJSON), &payload); err != nil {
		return writeExtensionFailure("test", *asJSON, fmt.Errorf("decode payload: %w", err), stdout, stderr)
	}
	source, err := os.ReadFile(filepath.Join(report.Root, filepath.FromSlash(report.Manifest.Main)))
	if err != nil {
		return writeExtensionFailure("test", *asJSON, err, stdout, stderr)
	}
	executable, err := os.Executable()
	if err != nil {
		return writeExtensionFailure("test", *asJSON, err, stdout, stderr)
	}
	state := map[string]interface{}{}
	views := map[string]interface{}{}
	notifications := []extensions.ExtensionNotification{}
	var mu sync.Mutex
	handler := func(_ context.Context, method string, raw json.RawMessage) (interface{}, error) {
		var params map[string]interface{}
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		mu.Lock()
		defer mu.Unlock()
		switch method {
		case "state.get":
			key, _ := params["key"].(string)
			if value, ok := state[key]; ok {
				return value, nil
			}
			return params["fallback"], nil
		case "state.set":
			key, _ := params["key"].(string)
			state[key] = params["value"]
			return map[string]bool{"ok": true}, nil
		case "state.delete":
			delete(state, params["key"].(string))
			return map[string]bool{"ok": true}, nil
		case "window.notify":
			notifications = append(notifications, extensions.ExtensionNotification{ExtensionID: report.Manifest.ID, Message: fmt.Sprint(params["message"]), Type: fmt.Sprint(params["type"])})
			return map[string]bool{"ok": true}, nil
		case "views.setState":
			views[fmt.Sprint(params["viewId"])] = params["state"]
			return map[string]bool{"ok": true}, nil
		case "requests.getActive", "responses.getActive":
			return map[string]interface{}{}, nil
		case "requests.execute":
			requestJSON, err := json.Marshal(params["request"])
			if err != nil {
				return nil, err
			}
			var response interface{}
			if err := json.Unmarshal([]byte(httpexec.Execute(string(requestJSON))), &response); err != nil {
				return nil, err
			}
			return response, nil
		case "log.write":
			return map[string]bool{"ok": true}, nil
		default:
			return nil, fmt.Errorf("unsupported preview host call: %s", method)
		}
	}
	host, err := extensions.StartHostProcess(executable, handler)
	if err != nil {
		return writeExtensionFailure("test", *asJSON, err, stdout, stderr)
	}
	defer host.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result := extensionTestResult{OK: true, Command: "test", SDKVersion: extensionsdk.Version, ExtensionID: report.Manifest.ID, Views: views, State: state, Notifications: notifications}
	if err := host.Request(ctx, "activate", extensions.ActivateHostRequest{Manifest: *report.Manifest, Settings: configurationDefaultsForCLI(report.Manifest), Source: string(source)}, &result.Activation); err != nil || !result.Activation.Success {
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Error = result.Activation.Error
		}
		result.OK = false
		return renderExtensionTestResult(result, *asJSON, stdout, stderr)
	}
	if *commandID != "" {
		var execution extensions.HostExecutionResult
		err = host.Request(ctx, "executeCommand", extensions.ExecuteCommandRequest{ExtensionID: report.Manifest.ID, CommandID: *commandID, Args: payload, Source: "cli-test"}, &execution)
		result.Execution = &execution
		if err != nil || !execution.Success {
			result.OK = false
			if err != nil {
				result.Error = err.Error()
			} else {
				result.Error = execution.Error
			}
		}
	}
	if result.OK && *eventName != "" {
		var eventResult extensions.HostExecutionResult
		err = host.Request(ctx, "dispatchEvent", extensions.DispatchEventRequest{ExtensionID: report.Manifest.ID, Event: *eventName, Payload: payload}, &eventResult)
		result.Event = &eventResult
		if err != nil || !eventResult.Success {
			result.OK = false
			if err != nil {
				result.Error = err.Error()
			} else {
				result.Error = eventResult.Error
			}
		}
	}
	mu.Lock()
	result.Notifications = append([]extensions.ExtensionNotification{}, notifications...)
	result.Views = cloneAnyMap(views)
	result.State = cloneAnyMap(state)
	mu.Unlock()
	return renderExtensionTestResult(result, *asJSON, stdout, stderr)
}

func renderExtensionTestResult(result extensionTestResult, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
		writeExtensionJSON(stdout, result)
	} else if result.OK {
		fmt.Fprintf(stdout, "Extension %s activated successfully", result.ExtensionID)
		if result.Execution != nil {
			fmt.Fprintf(stdout, "; command completed in %.2f ms", result.Execution.TimeMS)
		}
		if result.Event != nil {
			fmt.Fprintf(stdout, "; event completed in %.2f ms", result.Event.TimeMS)
		}
		fmt.Fprintln(stdout)
	} else {
		fmt.Fprintf(stderr, "adomnia extension test: %s\n", result.Error)
	}
	if result.OK {
		return 0
	}
	return 1
}

func configurationDefaultsForCLI(manifest *extensions.Manifest) map[string]interface{} {
	result := map[string]interface{}{}
	for key, property := range manifest.Contributes.Configuration {
		if property.Default != nil {
			result[key] = property.Default
		}
	}
	return result
}

func cloneAnyMap(source map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
