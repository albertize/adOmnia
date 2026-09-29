package extensionsdk_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	extensionsdk "adomnia/sdk/extensions"

	"github.com/evanw/esbuild/pkg/api"
)

var markdownFencePattern = regexp.MustCompile("(?s)```(json|js|javascript|ts|typescript)\\s*\\n(.*?)```")

func TestMarkdownSDKSnippetsParseAndCompile(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "sdk")
	if _, err := extensionsdk.Export(destination); err != nil {
		t.Fatal(err)
	}
	checked := 0
	err := filepath.WalkDir(filepath.Join(destination, "docs"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for index, match := range markdownFencePattern.FindAllSubmatch(content, -1) {
			language, source := string(match[1]), string(match[2])
			checked++
			if language == "json" {
				var value any
				if err := json.Unmarshal([]byte(source), &value); err != nil {
					t.Errorf("%s snippet %d: invalid JSON: %v", path, index+1, err)
				}
				continue
			}
			loader := api.LoaderJS
			if language == "ts" || language == "typescript" {
				loader = api.LoaderTS
			}
			options := api.TransformOptions{Loader: loader, Target: api.ES2020, Format: api.FormatESModule, Sourcefile: fmt.Sprintf("%s#snippet-%d", path, index+1)}
			result := api.Transform(source, options)
			if len(result.Errors) > 0 && strings.Contains(result.Errors[0].Text, "Top-level await") {
				// Some pages intentionally show a statement that belongs inside an
				// async activation callback rather than a complete module.
				result = api.Transform("async function snippet() {\n"+source+"\n}", options)
			}
			for _, item := range result.Errors {
				t.Errorf("%s snippet %d: %s", path, index+1, item.Text)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no Markdown SDK snippets were checked")
	}
}
