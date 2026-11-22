// spec: FEAT-001 / VS Code 围栏保存与冲突恢复
package externalcode

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

const editorFixtureEnv = "OBJECT_NA_TEST_EXTERNAL_EDITOR"
const initialCode = "let x: number = 1;\r\n"
const savedCode = "const Example = () => <p>你好</p>;\r\n"

func TestSessionReturnsSavedCodeAndKeepsRecoveryUntilCleanup(t *testing.T) {
	launcher, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	gate := filepath.Join(t.TempDir(), "close")
	t.Setenv(editorFixtureEnv, "1")
	session, err := openWith(launchCommand{launcher, []string{"-test.run=^TestEditorFixture$", "--", gate}, nil}, "tsx", initialCode)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(gate, nil, 0o600)
		_ = os.Remove(session.Path)
		_ = os.Remove(filepath.Dir(session.Path))
	})
	if filepath.Ext(session.Path) != ".tsx" {
		t.Fatalf("filename = %q, want a TSX file", session.Path)
	}
	if data, err := os.ReadFile(session.Path); err != nil || string(data) != initialCode {
		t.Fatalf("initial temporary code = %q with error %v", data, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(session.Path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("temporary file is readable by other users: %v", info.Mode())
		}
	}
	if err := session.Cleanup(); err == nil {
		t.Fatal("cleanup succeeded before the editor closed")
	}
	select {
	case beforeClose := <-session.Result:
		t.Fatalf("returned before the fixture editor closed: %+v", beforeClose)
	default:
	}
	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-session.Result:
		if result.Err != nil || result.Text != savedCode {
			t.Fatalf("final result = %+v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the fixture editor to close")
	}
	// A later write changes the file after the returned version. Keep both
	// the draft and the on-disk recovery path instead of deleting the new bytes.
	newer := []byte("/* another edit */\n" + savedCode)
	if err := os.WriteFile(session.Path, newer, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := session.Cleanup(); err == nil {
		t.Fatal("cleanup accepted unacknowledged changes")
	}
	if data, err := os.ReadFile(session.Path); err != nil || !bytes.Equal(data, newer) {
		t.Fatalf("recovery file = %q with error %v", data, err)
	}
	if err := os.WriteFile(session.Path, []byte(savedCode), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := session.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(session.Path); !os.IsNotExist(err) {
		t.Fatalf("temporary code still present after acknowledgment: %v", err)
	}
}

func TestEditorFixture(t *testing.T) {
	if os.Getenv(editorFixtureEnv) != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		t.Fatal("missing editor fixture arguments")
	}
	arguments := os.Args[separator+1:]
	if len(arguments) != 4 || !slices.Equal(arguments[1:3], []string{"--new-window", "--wait"}) {
		t.Fatalf("editor arguments = %v", arguments)
	}
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(arguments[0]); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no close request for the fixture editor")
		}
	}
	if err := os.WriteFile(arguments[3], []byte(savedCode), 0o600); err != nil {
		t.Fatal(err)
	}
}
