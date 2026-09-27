package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

type BuildResult struct {
	Built    bool     `json:"built"`
	Entry    string   `json:"entry,omitempty"`
	Output   string   `json:"output,omitempty"`
	Warnings []string `json:"warnings"`
}

// Build compiles and bundles an optional manifest source entry with embedded
// esbuild. The resulting CommonJS bundle runs in the extension host without a
// Node.js installation or runtime package resolution.
func Build(root string) (BuildResult, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return BuildResult{}, err
	}
	manifest, err := LoadManifest(absolute)
	if err != nil {
		return BuildResult{}, err
	}
	if strings.TrimSpace(manifest.Source) == "" {
		return BuildResult{Built: false, Warnings: []string{}}, nil
	}
	if err := validateRelativePath(manifest.Source); err != nil {
		return BuildResult{}, fmt.Errorf("invalid source path: %w", err)
	}
	if err := validateRelativePath(manifest.Main); err != nil {
		return BuildResult{}, fmt.Errorf("invalid output path: %w", err)
	}
	entry := filepath.Join(absolute, filepath.FromSlash(manifest.Source))
	output := filepath.Join(absolute, filepath.FromSlash(manifest.Main))
	result := api.Build(api.BuildOptions{
		AbsWorkingDir:  absolute,
		EntryPoints:    []string{entry},
		Outfile:        output,
		Bundle:         true,
		Write:          false,
		Platform:       api.PlatformNeutral,
		Format:         api.FormatCommonJS,
		Target:         api.ES2020,
		Sourcemap:      api.SourceMapExternal,
		SourcesContent: api.SourcesContentInclude,
		LegalComments:  api.LegalCommentsNone,
		LogLevel:       api.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		messages := make([]string, 0, len(result.Errors))
		for _, item := range result.Errors {
			messages = append(messages, formatBuildMessage(item))
		}
		return BuildResult{}, fmt.Errorf("extension build failed: %s", strings.Join(messages, "; "))
	}
	warnings := make([]string, 0, len(result.Warnings))
	for _, item := range result.Warnings {
		warnings = append(warnings, formatBuildMessage(item))
	}
	for _, file := range result.OutputFiles {
		target := file.Path
		if target == "" {
			continue
		}
		relative, err := filepath.Rel(absolute, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return BuildResult{}, fmt.Errorf("compiler output escapes extension root: %s", target)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return BuildResult{}, err
		}
		temporary, err := os.CreateTemp(filepath.Dir(target), ".extension-build-*.tmp")
		if err != nil {
			return BuildResult{}, err
		}
		temporaryPath := temporary.Name()
		if _, err := temporary.Write(file.Contents); err != nil {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
			return BuildResult{}, err
		}
		if err := temporary.Close(); err != nil {
			_ = os.Remove(temporaryPath)
			return BuildResult{}, err
		}
		_ = os.Remove(target)
		if err := os.Rename(temporaryPath, target); err != nil {
			_ = os.Remove(temporaryPath)
			return BuildResult{}, err
		}
	}
	return BuildResult{Built: true, Entry: manifest.Source, Output: manifest.Main, Warnings: warnings}, nil
}

func formatBuildMessage(message api.Message) string {
	if message.Location == nil {
		return message.Text
	}
	return fmt.Sprintf("%s:%d:%d: %s", message.Location.File, message.Location.Line, message.Location.Column, message.Text)
}
