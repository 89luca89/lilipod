package procutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDetachedInitializesMissingSysProcAttr(t *testing.T) {
	if os.Getenv("LILIPOD_RUN_DETACHED_HELPER") == "1" {
		cmd := exec.Command("/bin/sleep", "1")
		logfile := filepath.Join(t.TempDir(), "current-logs")
		if err := RunDetached(cmd, logfile); err != nil {
			t.Fatalf("RunDetached returned error: %v", err)
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestRunDetachedInitializesMissingSysProcAttr")
	cmd.Env = append(os.Environ(), "LILIPOD_RUN_DETACHED_HELPER=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("RunDetached subprocess failed: %v\n%s", err, strings.TrimSpace(string(output)))
	}
}
