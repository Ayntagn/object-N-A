package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const vaultWelcomeFile = "Welcome.md"
const vaultWelcomeText = "# Welcome to your vault\n\nThis is your new writing vault. Create notes, link them with [[wikilinks]], and organize with #tags.\n"

// VaultSetupPlan names the new Vault and its immediate child directories.
type VaultSetupPlan struct {
	Name    string
	Folders []string
}

// PrepareVaultSetup trims the form fields and counts valid, distinct folders
// even when an invalid field prevents creation.
func PrepareVaultSetup(name string, folders []string) (VaultSetupPlan, error) {
	plan := VaultSetupPlan{Name: strings.TrimSpace(name)}
	var err error
	if !vaultSetupComponent(plan.Name) {
		err = fmt.Errorf("invalid Vault name %q", plan.Name)
	}
	for _, input := range folders {
		folder := strings.TrimSpace(input)
		if folder == "" {
			continue
		}
		if !vaultSetupComponent(folder) || strings.EqualFold(folder, vaultWelcomeFile) ||
			strings.EqualFold(folder, TrashDirectory) {
			err = errors.Join(err, fmt.Errorf("invalid Vault folder name %q", folder))
			continue
		}
		if !slices.ContainsFunc(plan.Folders, func(existing string) bool { return strings.EqualFold(existing, folder) }) {
			plan.Folders = append(plan.Folders, folder)
		}
	}
	return plan, err
}

func vaultSetupComponent(name string) bool {
	_, err := filepath.Localize(name)
	return err == nil && name != "." && !strings.ContainsRune(name, '/')
}

// CreateVault creates the plan under parent, using a numbered name when the
// preferred destination already exists.
func CreateVault(parent string, plan VaultSetupPlan) (_ string, err error) {
	plan, err = PrepareVaultSetup(plan.Name, plan.Folders)
	if err != nil {
		return "", err
	}
	if parent == "" {
		return "", errors.New("choose a parent folder")
	}
	parent, err = filepath.Abs(parent)
	if err != nil {
		return "", err
	}
	parentRoot, err := os.OpenRoot(parent)
	if err != nil {
		return "", err
	}
	defer parentRoot.Close()

	name := plan.Name
	for suffix := 2; ; suffix++ {
		err = parentRoot.Mkdir(name, 0o755)
		if !errors.Is(err, fs.ErrExist) {
			break
		}
		name = fmt.Sprintf("%s %d", plan.Name, suffix)
	}
	if err != nil {
		return "", fmt.Errorf("create %q: %w", name, err)
	}
	root, err := parentRoot.OpenRoot(name)
	if err != nil {
		return "", errors.Join(err, parentRoot.Remove(name))
	}
	vault := &Vault{root: root}
	var created []string
	defer func() {
		if err == nil {
			vault.Close()
			return
		}
		for _, folder := range created {
			err = errors.Join(err, root.Remove(folder))
		}
		err = errors.Join(err, vault.Close(), parentRoot.Remove(name))
	}()
	for _, folder := range plan.Folders {
		if err := vault.CreateFolder(folder); err != nil {
			return "", fmt.Errorf("create %q: %w", folder, err)
		}
		created = append(created, folder)
	}
	if _, err := vault.CreateDocument(vaultWelcomeFile, vaultWelcomeText); err != nil {
		return "", fmt.Errorf("create the welcome note: %w", err)
	}
	return filepath.Join(parent, name), nil
}
