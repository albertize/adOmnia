package adomniacli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"adomnia/internal/extensions"
	"adomnia/internal/storage"
	extensionsdk "adomnia/sdk/extensions"
)

// AppVersion is set by the desktop entry point before dispatching CLI commands.
var AppVersion = "dev"

type doctorResult struct {
	OK              bool                            `json:"ok"`
	Command         string                          `json:"command"`
	SDKVersion      string                          `json:"sdkVersion"`
	ProtocolVersion string                          `json:"protocolVersion"`
	Executable      string                          `json:"executable"`
	Host            extensions.InitializeHostResult `json:"host"`
	Validation      *extensions.ValidationReport    `json:"validation,omitempty"`
	Error           string                          `json:"error,omitempty"`
}

func extensionInstall(args []string, stdout, stderr io.Writer) int {
	flagArgs, target, err := splitExtensionTarget(args, nil, map[string]bool{"json": true, "development": true})
	if err != nil {
		fmt.Fprintf(stderr, "adomnia extension install: %v\n", err)
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	development := fs.Bool("development", false, "link a development directory instead of copying it")
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if target == "" {
		fmt.Fprintln(stderr, "usage: adomnia extension install <folder|package.adomnia-extension> [--development] [--json]")
		return 2
	}
	registry, closeStore, err := openCLIRegistry()
	if err != nil {
		return writeExtensionFailure("install", *asJSON, err, stdout, stderr)
	}
	defer closeStore()
	var instance extensions.ExtensionInstance
	if strings.HasSuffix(strings.ToLower(target), ".adomnia-extension") {
		if *development {
			return writeExtensionFailure("install", *asJSON, fmt.Errorf("archives cannot be linked as development directories"), stdout, stderr)
		}
		instance, err = registry.InstallArchive(target)
	} else {
		instance, err = registry.InstallDirectory(target, *development)
	}
	if err != nil {
		return writeExtensionFailure("install", *asJSON, err, stdout, stderr)
	}
	if err := extensions.CheckEngineCompatibility(AppVersion, instance.Manifest.Engines.Adomnia); err != nil {
		_ = registry.Uninstall(instance.Manifest.ID)
		return writeExtensionFailure("install", *asJSON, err, stdout, stderr)
	}
	payload := extensionResult{OK: true, Command: "install", SDKVersion: extensionsdk.Version, Extension: &instance}
	if *asJSON {
		writeExtensionJSON(stdout, payload)
	} else {
		fmt.Fprintf(stdout, "Installed %s v%s disabled; review permissions in adOmnia before enabling it.\n", instance.Manifest.ID, instance.Manifest.Version)
	}
	return 0
}

func extensionList(args []string, stdout, stderr io.Writer) int {
	flagArgs, target, err := splitExtensionTarget(args, nil, map[string]bool{"json": true})
	if err != nil || target != "" {
		fmt.Fprintln(stderr, "usage: adomnia extension list [--json]")
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	registry, closeStore, err := openCLIRegistry()
	if err != nil {
		return writeExtensionFailure("list", *asJSON, err, stdout, stderr)
	}
	defer closeStore()
	items := registry.List()
	if *asJSON {
		writeExtensionJSON(stdout, extensionResult{OK: true, Command: "list", SDKVersion: extensionsdk.Version, Extensions: items})
	} else {
		if len(items) == 0 {
			fmt.Fprintln(stdout, "No Extension Platform v2 packages installed.")
		}
		for _, item := range items {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%v\n", item.Manifest.ID, item.Manifest.Version, item.InstallKind, item.Enabled)
		}
	}
	return 0
}

func extensionInspect(args []string, stdout, stderr io.Writer) int {
	flagArgs, id, err := splitExtensionTarget(args, nil, map[string]bool{"json": true})
	if err != nil {
		fmt.Fprintf(stderr, "adomnia extension inspect: %v\n", err)
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if id == "" {
		fmt.Fprintln(stderr, "usage: adomnia extension inspect <extension-id> [--json]")
		return 2
	}
	registry, closeStore, err := openCLIRegistry()
	if err != nil {
		return writeExtensionFailure("inspect", *asJSON, err, stdout, stderr)
	}
	defer closeStore()
	item, err := registry.Get(id)
	if err != nil {
		return writeExtensionFailure("inspect", *asJSON, err, stdout, stderr)
	}
	if *asJSON {
		writeExtensionJSON(stdout, extensionResult{OK: true, Command: "inspect", SDKVersion: extensionsdk.Version, Extension: &item})
	} else {
		fmt.Fprintf(stdout, "%s v%s\nEnabled: %v\nActive: %v\nSource: %s\nPermissions: %s\n", item.Manifest.Name, item.Manifest.Version, item.Enabled, item.Active, item.Source, strings.Join(item.Manifest.Permissions, ", "))
	}
	return 0
}

func extensionDoctor(args []string, stdout, stderr io.Writer) int {
	flagArgs, target, err := splitExtensionTarget(args, nil, map[string]bool{"json": true})
	if err != nil {
		fmt.Fprintf(stderr, "adomnia extension doctor: %v\n", err)
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	executable, err := os.Executable()
	if err != nil {
		return writeExtensionFailure("doctor", *asJSON, err, stdout, stderr)
	}
	result := doctorResult{OK: true, Command: "doctor", SDKVersion: extensionsdk.Version, ProtocolVersion: extensions.HostProtocolVersion, Executable: executable}
	if target != "" {
		report := extensions.CheckDirectory(target)
		result.Validation = &report
		if !report.Valid {
			result.OK = false
			result.Error = "extension validation failed"
		}
	}
	if result.OK {
		host, err := extensions.StartHostProcess(executable, func(context.Context, string, json.RawMessage) (interface{}, error) {
			return map[string]bool{"ok": true}, nil
		})
		if err != nil {
			result.OK = false
			result.Error = err.Error()
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = host.Request(ctx, "initialize", extensions.InitializeHostRequest{ProtocolVersion: extensions.HostProtocolVersion}, &result.Host)
			cancel()
			_ = host.Close()
		}
	}
	if *asJSON {
		writeExtensionJSON(stdout, result)
	} else if result.OK {
		fmt.Fprintf(stdout, "Extension host %s ready with SDK %s.\n", result.Host.ProtocolVersion, result.SDKVersion)
	} else {
		fmt.Fprintf(stderr, "adomnia extension doctor: %s\n", result.Error)
	}
	if result.OK {
		return 0
	}
	return 1
}

func openCLIRegistry() (*extensions.Registry, func(), error) {
	dataDir := cliApplicationDataDir()
	if err := storage.OpenApplicationDataDir(dataDir); err != nil {
		return nil, func() {}, fmt.Errorf("open adOmnia storage (is the desktop app running?): %w", err)
	}
	registry := extensions.NewRegistry(dataDir)
	if err := registry.Init(); err != nil {
		storage.Close()
		return nil, func() {}, err
	}
	return registry, storage.Close, nil
}

func cliApplicationDataDir() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "adomnia")
}
