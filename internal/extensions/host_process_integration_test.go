package extensions

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func buildTestDesktopExecutable(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds the desktop executable")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "adomnia-host-test")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	command := exec.Command("go", "build", "-o", binary, ".")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build test executable: %v\n%s", err, output)
	}
	return binary
}

func TestRealExecutableExtensionHostHandshake(t *testing.T) {
	host, err := StartHostProcess(buildTestDesktopExecutable(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var result InitializeHostResult
	if err := host.Request(ctx, "initialize", InitializeHostRequest{ProtocolVersion: HostProtocolVersion}, &result); err != nil {
		t.Fatal(err)
	}
	if result.ProtocolVersion != HostProtocolVersion || result.Runtime != "goja" {
		t.Fatalf("unexpected handshake: %#v", result)
	}
}
