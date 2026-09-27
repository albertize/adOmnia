package adomniacli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"adomnia/internal/extensions"
	extensionsdk "adomnia/sdk/extensions"
)

type legacyManifest struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Author        string   `json:"author"`
	Description   string   `json:"description"`
	License       string   `json:"license"`
	MinAppVersion string   `json:"minAppVersion"`
	EntryPoint    string   `json:"entryPoint"`
	Permissions   []string `json:"permissions"`
	Hooks         []struct {
		Event   string `json:"event"`
		Handler string `json:"handler"`
	} `json:"hooks"`
	Actions []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"actions"`
	Settings []struct {
		Key         string `json:"key"`
		Type        string `json:"type"`
		Default     any    `json:"default"`
		Description string `json:"description"`
	} `json:"settings"`
}

type migrationReport struct {
	OK                bool                `json:"ok"`
	Command           string              `json:"command"`
	SDKVersion        string              `json:"sdkVersion"`
	Source            string              `json:"source"`
	SuggestedManifest extensions.Manifest `json:"suggestedManifest"`
	Warnings          []string            `json:"warnings"`
	ManualSteps       []string            `json:"manualSteps"`
	Output            string              `json:"output,omitempty"`
}

var migrationSegmentPattern = regexp.MustCompile(`[^a-z0-9-]+`)

func extensionMigrateV1(args []string, stdout, stderr io.Writer) int {
	flagArgs, target, err := splitExtensionTarget(args, map[string]bool{"publisher": true, "out": true}, map[string]bool{"json": true})
	if err != nil {
		fmt.Fprintf(stderr, "adomnia extension migrate-v1: %v\n", err)
		return 2
	}
	fs := flag.NewFlagSet("adomnia extension migrate-v1", flag.ContinueOnError)
	fs.SetOutput(stderr)
	publisher := fs.String("publisher", "migrated", "publisher namespace for legacy IDs")
	output := fs.String("out", "", "optional migration report path")
	asJSON := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if target == "" {
		fmt.Fprintln(stderr, "usage: adomnia extension migrate-v1 <plugin-folder> [--publisher name] [--out report.json] [--json]")
		return 2
	}
	manifestPath := filepath.Join(target, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return writeExtensionFailure("migrate-v1", *asJSON, err, stdout, stderr)
	}
	var legacy legacyManifest
	if err := json.Unmarshal(data, &legacy); err != nil {
		return writeExtensionFailure("migrate-v1", *asJSON, err, stdout, stderr)
	}
	id := migrationID(*publisher, legacy.ID)
	commands := make([]extensions.CommandContribution, 0, len(legacy.Actions))
	activation := make([]string, 0, len(legacy.Actions)+len(legacy.Hooks))
	for _, action := range legacy.Actions {
		commandID := id + "." + migrationSegment(action.ID)
		commands = append(commands, extensions.CommandContribution{ID: commandID, Title: firstNonEmpty(action.Name, action.ID)})
		activation = append(activation, "onCommand:"+commandID)
	}
	permissions := append([]string(nil), legacy.Permissions...)
	for _, hook := range legacy.Hooks {
		activation = appendUnique(activation, hook.Event)
		switch hook.Event {
		case "onRequest", "onSend":
			permissions = appendUnique(permissions, "requests.read")
		case "onResponse":
			permissions = appendUnique(permissions, "responses.read")
		}
	}
	configuration := map[string]extensions.ConfigurationProperty{}
	for _, setting := range legacy.Settings {
		propertyType := setting.Type
		if propertyType == "" {
			propertyType = "string"
		}
		configuration[setting.Key] = extensions.ConfigurationProperty{Type: propertyType, Default: setting.Default, Description: setting.Description}
	}
	if len(activation) == 0 {
		activation = []string{"onStartup"}
	}
	manifest := extensions.Manifest{
		Schema: "https://adomnia.local/schemas/manifest-v2.json", ManifestVersion: 2,
		ID: id, Name: firstNonEmpty(legacy.Name, legacy.ID), Version: firstNonEmpty(legacy.Version, "0.1.0"), Publisher: migrationSegment(*publisher),
		Description: legacy.Description, License: legacy.License, Engines: extensions.Engines{Adomnia: ">=" + firstNonEmpty(legacy.MinAppVersion, "0.9.0")},
		APIVersion: "2.0", Main: "dist/extension.js", Source: legacy.EntryPoint, ActivationEvents: activation, Permissions: permissions,
		Contributes: extensions.Contributions{Commands: commands, Configuration: configuration},
	}
	report := migrationReport{OK: true, Command: "migrate-v1", SDKVersion: extensionsdk.Version, Source: manifestPath, SuggestedManifest: manifest,
		Warnings:    []string{"The report does not modify or disable the v1 plugin.", "Legacy JavaScript is not API-compatible with the injected v2 ExtensionAPI."},
		ManualSteps: []string{"Copy the suggested manifest into a new folder.", "Rewrite hooks as api.events handlers and actions as registered commands.", "Replace legacy globals with the injected api argument.", "Run extension build, check, test, and pack before installation."},
	}
	if *output != "" {
		encoded, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(*output, append(encoded, '\n'), 0644); err != nil {
			return writeExtensionFailure("migrate-v1", *asJSON, err, stdout, stderr)
		}
		report.Output = *output
	}
	if *asJSON {
		writeExtensionJSON(stdout, report)
	} else {
		fmt.Fprintf(stdout, "Migration report for %s: suggested v2 ID %s. %d manual steps required.\n", legacy.ID, id, len(report.ManualSteps))
	}
	return 0
}

func migrationID(publisher, legacyID string) string {
	publisher = migrationSegment(publisher)
	legacyID = strings.Trim(migrationSegment(strings.ReplaceAll(legacyID, ".", "-")), "-")
	if publisher == "" {
		publisher = "migrated"
	}
	if legacyID == "" {
		legacyID = "extension"
	}
	return publisher + "." + legacyID
}

func migrationSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Trim(migrationSegmentPattern.ReplaceAllString(value, "-"), "-")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
