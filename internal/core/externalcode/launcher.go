package externalcode

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ErrVSCodeNotFound indicates that a local VS Code launcher was not found.
var ErrVSCodeNotFound = errors.New("could not find the local VS Code command")

type launchCommand struct {
	executable string
	arguments  []string
	env        []string
}

func vscodeLauncher() (launchCommand, error) {
	if runtime.GOOS == "windows" {
		return windowsLauncher()
	}
	// An integrated remote terminal can inherit this hook. These files are
	// local and should open in the installed desktop app.
	env := []string{"VSCODE_IPC_HOOK_CLI="}
	if path, err := exec.LookPath("code"); err == nil {
		return launchCommand{executable: path, env: env}, nil
	}
	if runtime.GOOS == "darwin" {
		applications := []string{"/Applications"}
		if home, err := os.UserHomeDir(); err == nil {
			applications = append(applications, filepath.Join(home, "Applications"))
		}
		for _, directory := range applications {
			path := filepath.Join(directory, "Visual Studio Code.app", "Contents", "Resources", "app", "bin", "code")
			if executable, err := exec.LookPath(path); err == nil {
				return launchCommand{executable: executable, env: env}, nil
			}
		}
	}
	return launchCommand{}, ErrVSCodeNotFound
}

func windowsLauncher() (launchCommand, error) {
	var directories []string
	if path, err := exec.LookPath("code"); err == nil {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		directory := filepath.Dir(path)
		directories = append(directories, directory, filepath.Dir(directory))
	}
	if directory := os.Getenv("LOCALAPPDATA"); directory != "" {
		directories = append(directories, filepath.Join(directory, "Programs", "Microsoft VS Code"))
	}
	for _, name := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"} {
		if directory := os.Getenv(name); directory != "" {
			directories = append(directories, filepath.Join(directory, "Microsoft VS Code"))
		}
	}
	for _, directory := range directories {
		executable := filepath.Join(directory, "Code.exe")
		cli := filepath.Join(directory, "resources", "app", "out", "cli.js")
		if regular(executable) && regular(cli) {
			// This is the launcher in code.cmd with native argv, so a
			// temporary path never passes through cmd.exe expansions.
			return launchCommand{
				executable: executable,
				arguments:  []string{cli},
				env:        []string{"VSCODE_DEV=", "VSCODE_IPC_HOOK_CLI=", "ELECTRON_RUN_AS_NODE=1"},
			}, nil
		}
	}
	return launchCommand{}, ErrVSCodeNotFound
}

func regular(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
