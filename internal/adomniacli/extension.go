package adomniacli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"adomnia/internal/extensions"
	extensionsdk "adomnia/sdk/extensions"
)

type extensionResult struct {
	OK         bool                           `json:"ok"`
	Command    string                         `json:"command"`
	SDKVersion string                         `json:"sdkVersion"`
	Scaffold   *extensions.ScaffoldResult     `json:"scaffold,omitempty"`
	Package    *extensions.PackageResult      `json:"package,omitempty"`
	Build      *extensions.BuildResult        `json:"build,omitempty"`
	Validation *extensions.ValidationReport   `json:"validation,omitempty"`
	Extension  *extensions.ExtensionInstance  `json:"extension,omitempty"`
	Extensions []extensions.ExtensionInstance `json:"extensions,omitempty"`
	Path       string                         `json:"path,omitempty"`
	Error      string                         `json:"error,omitempty"`
}

func Extension(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printExtensionUsage(stderr)
		return 2
	}
	switch args[0] {
	case "init":
		return extensionInit(args[1:], stdout, stderr)
	case "check":
		return extensionCheck(args[1:], stdout, stderr)
	case "build":
		return extensionBuild(args[1:], stdout, stderr)
	case "pack":
		return extensionPack(args[1:], stdout, stderr)
	case "test":
		return extensionTest(args[1:], stdout, stderr)
	case "install":
		return extensionInstall(args[1:], stdout, stderr)
	case "list":
		return extensionList(args[1:], stdout, stderr)
	case "inspect":
		return extensionInspect(args[1:], stdout, stderr)
	case "doctor":
		return extensionDoctor(args[1:], stdout, stderr)
	case "migrate-v1":
		return extensionMigrateV1(args[1:], stdout, stderr)
	case "sdk", "sdk-path":
		return extensionSDK(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "adomnia extension: unknown command %q\n", args[0])
		printExtensionUsage(stderr)
		return 2
	}
}

func extensionInit(args []string, stdout, stderr io.Writer) int {
	flagArgs, target, err := splitExtensionTarget(args, map[string]bool{"id": true, "name": true, "publisher": true, "template": true}, map[string]bool{"json": true})
	if err != nil {
		fmt.Fprintf(stderr, "adomnia extension init: %v\n", err)
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "namespaced extension id")
	name := fs.String("name", "", "user-visible extension name")
	publisher := fs.String("publisher", "local", "publisher namespace")
	template := fs.String("template", "minimal", "template: minimal, view, request-hook, webview, or typescript")
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if target == "" {
		fmt.Fprintln(stderr, "usage: adomnia extension init <directory> [--id publisher.name] [--name name] [--publisher publisher] [--template minimal|view|request-hook|webview|typescript] [--json]")
		return 2
	}
	result, err := extensions.Scaffold(target, extensions.ScaffoldOptions{ID: *id, Name: *name, Publisher: *publisher, Template: *template})
	if err != nil {
		return writeExtensionFailure("init", *asJSON, err, stdout, stderr)
	}
	payload := extensionResult{OK: true, Command: "init", SDKVersion: extensionsdk.Version, Scaffold: &result}
	if *asJSON {
		writeExtensionJSON(stdout, payload)
	} else {
		fmt.Fprintf(stdout, "Created %s (%s) from %s template\n", result.Path, result.ID, result.Template)
		fmt.Fprintln(stdout, "Next: adomnia extension check", result.Path, "--json")
	}
	return 0
}

func extensionCheck(args []string, stdout, stderr io.Writer) int {
	flagArgs, target, err := splitExtensionTarget(args, nil, map[string]bool{"json": true})
	if err != nil {
		fmt.Fprintf(stderr, "adomnia extension check: %v\n", err)
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if target == "" {
		target = "."
	}
	report := extensions.CheckDirectory(target)
	if *asJSON {
		writeExtensionJSON(stdout, report)
	} else {
		writeValidationText(stdout, report)
	}
	if !report.Valid {
		return 1
	}
	return 0
}

func extensionBuild(args []string, stdout, stderr io.Writer) int {
	flagArgs, target, err := splitExtensionTarget(args, nil, map[string]bool{"json": true})
	if err != nil {
		fmt.Fprintf(stderr, "adomnia extension build: %v\n", err)
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if target == "" {
		target = "."
	}
	result, err := extensions.Build(target)
	if err != nil {
		return writeExtensionFailure("build", *asJSON, err, stdout, stderr)
	}
	if *asJSON {
		writeExtensionJSON(stdout, extensionResult{OK: true, Command: "build", SDKVersion: extensionsdk.Version, Build: &result})
	} else if result.Built {
		fmt.Fprintf(stdout, "Built %s → %s\n", result.Entry, result.Output)
	} else {
		fmt.Fprintln(stdout, "No source entry declared; existing JavaScript main is ready.")
	}
	return 0
}

func extensionPack(args []string, stdout, stderr io.Writer) int {
	flagArgs, target, err := splitExtensionTarget(args, map[string]bool{"out": true}, map[string]bool{"json": true})
	if err != nil {
		fmt.Fprintf(stderr, "adomnia extension pack: %v\n", err)
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension pack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "output .adomnia-extension path")
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if target == "" {
		target = "."
	}
	result, report, err := extensions.Pack(target, *out)
	if err != nil {
		return writeExtensionFailure("pack", *asJSON, err, stdout, stderr)
	}
	if !report.Valid {
		if *asJSON {
			writeExtensionJSON(stdout, extensionResult{OK: false, Command: "pack", SDKVersion: extensionsdk.Version, Validation: &report, Error: "extension validation failed"})
		} else {
			writeValidationText(stdout, report)
		}
		return 1
	}
	payload := extensionResult{OK: true, Command: "pack", SDKVersion: extensionsdk.Version, Package: &result, Validation: &report}
	if *asJSON {
		writeExtensionJSON(stdout, payload)
	} else {
		fmt.Fprintf(stdout, "Packed %s\nSHA-256 %s\n%d files, %d bytes\n", result.Path, result.SHA256, result.Files, result.Bytes)
	}
	return 0
}

func extensionSDK(args []string, stdout, stderr io.Writer) int {
	flagArgs, target, err := splitExtensionTarget(args, nil, map[string]bool{"json": true})
	if err != nil {
		fmt.Fprintf(stderr, "adomnia extension sdk: %v\n", err)
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension sdk", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if target == "" {
		target = filepath.Join(".", "adomnia-extension-sdk-"+extensionsdk.Version)
	}
	path, err := extensionsdk.Export(target)
	if err != nil {
		return writeExtensionFailure("sdk", *asJSON, err, stdout, stderr)
	}
	payload := extensionResult{OK: true, Command: "sdk", SDKVersion: extensionsdk.Version, Path: path}
	if *asJSON {
		writeExtensionJSON(stdout, payload)
	} else {
		fmt.Fprintf(stdout, "Exported adOmnia Extension SDK %s to %s\n", extensionsdk.Version, path)
	}
	return 0
}

func splitExtensionTarget(args []string, valueFlags, boolFlags map[string]bool) ([]string, string, error) {
	flagArgs := []string{}
	target := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			if target != "" {
				return nil, "", fmt.Errorf("unexpected argument %q", arg)
			}
			target = arg
			continue
		}
		nameValue := strings.TrimLeft(arg, "-")
		name, _, hasEquals := strings.Cut(nameValue, "=")
		switch {
		case boolFlags[name]:
			flagArgs = append(flagArgs, arg)
		case valueFlags[name]:
			flagArgs = append(flagArgs, arg)
			if !hasEquals {
				if i+1 >= len(args) {
					return nil, "", fmt.Errorf("flag %s requires a value", arg)
				}
				i++
				flagArgs = append(flagArgs, args[i])
			}
		default:
			flagArgs = append(flagArgs, arg)
		}
	}
	return flagArgs, target, nil
}

func writeValidationText(output io.Writer, report extensions.ValidationReport) {
	if report.Valid {
		fmt.Fprintf(output, "Extension is valid: %s (%d files, %d bytes)\n", report.Root, report.Files, report.Bytes)
		return
	}
	fmt.Fprintf(output, "Extension is invalid: %s\n", report.Root)
	for _, diagnostic := range report.Diagnostics {
		location := diagnostic.Path
		if location != "" {
			location += ": "
		}
		fmt.Fprintf(output, "[%s] %s%s (%s)\n", diagnostic.Level, location, diagnostic.Message, diagnostic.Code)
	}
}

func writeExtensionFailure(command string, asJSON bool, err error, stdout, stderr io.Writer) int {
	if asJSON {
		writeExtensionJSON(stdout, extensionResult{OK: false, Command: command, SDKVersion: extensionsdk.Version, Error: err.Error()})
	} else {
		fmt.Fprintf(stderr, "adomnia extension %s: %v\n", command, err)
	}
	return 1
}

func writeExtensionJSON(output io.Writer, value interface{}) {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func printExtensionUsage(output io.Writer) {
	fmt.Fprintln(output, "usage: adomnia extension <init|check|build|test|pack|install|list|inspect|doctor|migrate-v1|sdk|sdk-path> [options]")
}
