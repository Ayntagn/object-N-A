// Package externalcode exchanges the contents of a fenced code block with
// a separate local VS Code window through a temporary file.
package externalcode

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
)

// Result contains the last saved contents after the external file closes.
// When Err is non-nil, the caller can offer the temporary path for recovery.
type Result struct {
	Text string
	Err  error
}

// Session exposes the recovery file and the single result from VS Code.
// The receiving caller owns cleanup after the result has been handled.
type Session struct {
	Path   string
	Result <-chan Result

	directory string
	filename  string
	done      chan struct{}
	outcome   Result
}

// Open starts a separate VS Code window for the given language and code.
func Open(language, text string) (*Session, error) {
	launcher, err := vscodeLauncher()
	if err != nil {
		return nil, err
	}
	return openWith(launcher, language, text)
}

func openWith(launcher launchCommand, language, text string) (*Session, error) {
	directory, err := os.MkdirTemp("", "object-na-code-")
	if err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		_ = os.Remove(directory)
		return nil, err
	}
	filename := codeFilename(language)
	path := filepath.Join(absolute, filename)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		_ = removeCodeFile(absolute, path)
		return nil, err
	}

	arguments := append(append([]string(nil), launcher.arguments...), "--new-window", "--wait", path)
	command := exec.Command(launcher.executable, arguments...)
	command.Dir = absolute
	command.Env = append(command.Environ(), launcher.env...)
	if err := command.Start(); err != nil {
		_ = removeCodeFile(absolute, path)
		return nil, fmt.Errorf("start VS Code: %w", err)
	}

	result := make(chan Result, 1)
	session := &Session{Path: path, Result: result, directory: absolute, filename: filename, done: make(chan struct{})}
	go func() {
		if err := command.Wait(); err != nil {
			session.outcome.Err = fmt.Errorf("VS Code exited before returning the code: %w", err)
		} else {
			session.outcome.Text, session.outcome.Err = readCodeFile(path)
		}
		close(session.done)
		result <- session.outcome
		close(result)
	}()
	return session, nil
}

// Cleanup removes the acknowledged temporary file after VS Code exits. A
// failed session or later on-disk edits keep the path available for recovery.
func (s *Session) Cleanup() error {
	select {
	case <-s.done:
	default:
		return errors.New("the file is still open in VS Code")
	}
	if s.outcome.Err != nil {
		return s.outcome.Err
	}
	path := filepath.Join(s.directory, s.filename)
	text, err := readCodeFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && text != s.outcome.Text {
		return errors.New("the temporary code has been changed since VS Code closed")
	}
	return removeCodeFile(s.directory, path)
}

func readCodeFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("the temporary code path is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", errors.New("VS Code did not save the temporary file as UTF-8")
	}
	return string(data), nil
}

func removeCodeFile(directory, path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Other files created in the editor remain available.
	_ = os.Remove(directory)
	return nil
}

func codeFilename(language string) string {
	const stem = "Code Block"
	tag, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(language)), " ")
	switch tag {
	case "make", "makefile", "mk":
		return "Makefile"
	case "docker", "dockerfile":
		return "Dockerfile"
	case "powershell", "pwsh", "ps1":
		return stem + ".ps1"
	}
	if language := editor.LanguageNamed(tag); language != nil && len(language.Extensions) > 0 {
		// Keep variants such as TSX and JSX when the fence names them.
		for _, extension := range language.Extensions {
			if "."+tag == extension {
				return stem + extension
			}
		}
		return stem + language.Extensions[0]
	}
	return stem + ".txt"
}
