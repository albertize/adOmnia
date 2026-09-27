package extensions

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	extensionsdk "adomnia/sdk/extensions"
)

type ScaffoldOptions struct {
	ID        string
	Name      string
	Publisher string
	Template  string
}

type ScaffoldResult struct {
	Path     string   `json:"path"`
	ID       string   `json:"id"`
	Template string   `json:"template"`
	Files    []string `json:"files"`
}

func Scaffold(destination string, options ScaffoldOptions) (ScaffoldResult, error) {
	if options.Template == "" {
		options.Template = "minimal"
	}
	switch options.Template {
	case "minimal", "view", "request-hook", "webview", "typescript":
	default:
		return ScaffoldResult{}, fmt.Errorf("unknown template %q; expected minimal, view, request-hook, webview, or typescript", options.Template)
	}
	if options.Publisher == "" {
		options.Publisher = "local"
	}
	if options.Name == "" {
		options.Name = titleFromSlug(filepath.Base(destination))
	}
	if options.ID == "" {
		options.ID = options.Publisher + "." + slugify(filepath.Base(destination))
	}
	preview := Manifest{
		ManifestVersion: SupportedManifestVersion,
		ID:              options.ID,
		Name:            options.Name,
		Version:         "0.1.0",
		Publisher:       options.Publisher,
		Engines:         Engines{Adomnia: ">=1.0.0 <2"},
		APIVersion:      SupportedAPIVersion,
		Main:            "src/extension.js",
	}
	for _, diagnostic := range ValidateManifest(preview) {
		// Ignore contribution-independent errors that templates fill after rendering.
		if diagnostic.Path == "activationEvents" || strings.HasPrefix(diagnostic.Path, "contributes") {
			continue
		}
		return ScaffoldResult{}, fmt.Errorf("invalid scaffold option %s: %s", diagnostic.Path, diagnostic.Message)
	}

	absolute, err := filepath.Abs(destination)
	if err != nil {
		return ScaffoldResult{}, fmt.Errorf("resolve extension destination: %w", err)
	}
	if entries, readErr := os.ReadDir(absolute); readErr == nil && len(entries) > 0 {
		return ScaffoldResult{}, fmt.Errorf("extension destination is not empty: %s", absolute)
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return ScaffoldResult{}, fmt.Errorf("inspect extension destination: %w", readErr)
	}

	prefix := "templates/" + options.Template
	files := []string{}
	err = fs.WalkDir(extensionsdk.Files, prefix, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(path, prefix+"/")
		relative = strings.TrimSuffix(relative, ".tmpl")
		if relative == "" || strings.HasPrefix(relative, "../") {
			return fmt.Errorf("invalid embedded template path %s", path)
		}
		data, err := extensionsdk.Files.ReadFile(path)
		if err != nil {
			return err
		}
		content := strings.NewReplacer(
			"{{ID}}", options.ID,
			"{{NAME}}", options.Name,
			"{{PUBLISHER}}", options.Publisher,
		).Replace(string(data))
		target := filepath.Join(absolute, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(content), 0644); err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		_ = os.RemoveAll(absolute)
		return ScaffoldResult{}, fmt.Errorf("create extension: %w", err)
	}
	sort.Strings(files)
	buildResult, buildErr := Build(absolute)
	if buildErr != nil {
		_ = os.RemoveAll(absolute)
		return ScaffoldResult{}, fmt.Errorf("build generated extension: %w", buildErr)
	}
	if buildResult.Built {
		files = append(files, buildResult.Output, buildResult.Output+".map")
		sort.Strings(files)
	}
	report := CheckDirectory(absolute)
	if !report.Valid {
		_ = os.RemoveAll(absolute)
		return ScaffoldResult{}, fmt.Errorf("generated extension failed validation: %s", formatFirstDiagnostic(report.Diagnostics))
	}
	return ScaffoldResult{Path: absolute, ID: options.ID, Template: options.Template, Files: files}, nil
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			builder.WriteRune(r)
			lastDash = false
		} else if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func titleFromSlug(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		parts[i] = strings.ToUpper(string(runes[0])) + string(runes[1:])
	}
	if len(parts) == 0 {
		return "adOmnia Extension"
	}
	return strings.Join(parts, " ")
}

func formatFirstDiagnostic(diagnostics []Diagnostic) string {
	if len(diagnostics) == 0 {
		return "unknown validation error"
	}
	item := diagnostics[0]
	if item.Path == "" {
		return item.Message
	}
	return item.Path + ": " + item.Message
}
