package workspace

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const ExampleVaultName = "cyberWriter Example Vault"

//go:embed example_vault
var exampleVaultFiles embed.FS

func BuiltinExampleVault() (fs.FS, error) {
	return fs.Sub(exampleVaultFiles, "example_vault")
}

type ExampleVaultRestore struct {
	root     string
	previous fs.FileInfo
}

func PlanExampleVaultRestore(parent string) (ExampleVaultRestore, error) {
	if parent == "" {
		return ExampleVaultRestore{}, errors.New("the example Vault needs an application data directory")
	}
	root, err := filepath.Abs(filepath.Join(parent, ExampleVaultName))
	if err != nil {
		return ExampleVaultRestore{}, err
	}
	info, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return ExampleVaultRestore{root: root}, nil
	}
	if err != nil {
		return ExampleVaultRestore{}, err
	}
	if !info.IsDir() {
		return ExampleVaultRestore{}, fmt.Errorf("%q is not an example Vault directory", root)
	}
	return ExampleVaultRestore{root: root, previous: info}, nil
}

func (restore ExampleVaultRestore) Root() string   { return restore.root }
func (restore ExampleVaultRestore) Existing() bool { return restore.previous != nil }

// Apply stages the complete template on the destination volume. The returned
// bool reports whether publication succeeded even if cleanup produced an error.
func (restore ExampleVaultRestore) Apply(source fs.FS) (_ bool, err error) {
	if restore.root == "" || source == nil {
		return false, errors.New("an example Vault plan and template are required")
	}
	welcome, err := fs.Stat(source, "Welcome.md")
	if err != nil {
		return false, fmt.Errorf("open the example Vault welcome note: %w", err)
	}
	if !welcome.Mode().IsRegular() {
		return false, errors.New("Welcome.md must be a regular file")
	}
	parent := filepath.Dir(restore.root)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return false, err
	}
	stage, err := os.MkdirTemp(parent, ".example-vault-restore-")
	if err != nil {
		return false, err
	}
	removeStage := true
	defer func() {
		if removeStage {
			err = errors.Join(err, os.RemoveAll(stage))
		}
	}()

	fresh := filepath.Join(stage, "new")
	if err := os.Mkdir(fresh, 0o755); err != nil {
		return false, err
	}
	if err := copyExampleVault(source, fresh); err != nil {
		return false, fmt.Errorf("copy the example Vault template: %w", err)
	}

	current, err := os.Lstat(restore.root)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if restore.previous == nil {
		if current != nil {
			return false, errors.New("the example Vault appeared before installation")
		}
		return publishExampleVault(fresh, restore.root)
	}
	if current == nil || !os.SameFile(current, restore.previous) {
		return false, errors.New("the example Vault changed before restoration")
	}

	previous := filepath.Join(stage, "previous")
	if err := os.Rename(restore.root, previous); err != nil {
		return false, fmt.Errorf("move the existing example Vault: %w", err)
	}
	rollback := func(cause error) error {
		if _, statErr := os.Lstat(restore.root); errors.Is(statErr, fs.ErrNotExist) {
			if moveErr := os.Rename(previous, restore.root); moveErr == nil {
				return cause
			} else {
				cause = errors.Join(cause, moveErr)
			}
		}
		removeStage = false
		return fmt.Errorf("the prior example Vault is available at %q: %w", previous, cause)
	}
	moved, err := os.Lstat(previous)
	if err != nil || !os.SameFile(moved, restore.previous) {
		return false, rollback(errors.Join(errors.New("the example Vault changed during restoration"), err))
	}
	if published, err := publishExampleVault(fresh, restore.root); !published {
		return false, rollback(err)
	}
	return true, nil
}

func publishExampleVault(fresh, destination string) (bool, error) {
	if err := os.Rename(fresh, destination); err != nil {
		return false, fmt.Errorf("publish the example Vault: %w", err)
	}
	return true, nil
}

func copyExampleVault(source fs.FS, destination string) error {
	return fs.WalkDir(source, ".", func(relative string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || relative == "." {
			return walkErr
		}
		native, err := filepath.Localize(relative)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, native)
		if entry.IsDir() {
			return os.Mkdir(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%q is not a regular example Vault file", relative)
		}
		input, err := source.Open(relative)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		_, err = io.Copy(output, input)
		return errors.Join(err, output.Close())
	})
}
