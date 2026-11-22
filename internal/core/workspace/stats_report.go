package workspace

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"unicode/utf8"
)

const VaultStatsFile = "Vault Stats.md"
const VaultHealthFile = "Vault Health.md"

var ErrUnmanagedVaultStats = errors.New("Vault Stats.md exists without the generated report marker")

// ReportEntries lists note and attachment types counted by the Vault report.
func (v *Vault) ReportEntries() ([]Entry, error) {
	return v.SidebarEntries(false)
}

// SidebarEntries lists the visible Vault tree. All also includes regular
// files outside the supported type list. The reports keep their own scope.
func (v *Vault) SidebarEntries(all bool) ([]Entry, error) {
	return readEntries(v.root.FS(), ".", -1, sidebarEntryFilter(all))
}

func sidebarEntryFilter(all bool) func(fs.DirEntry) bool {
	return func(file fs.DirEntry) bool {
		return !strings.HasPrefix(file.Name(), ".") && (all || file.IsDir() || isSupportedVaultFile(file.Name()))
	}
}

func isSupportedVaultFile(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".md", ".markdown", ".excalidraw", ".txt", ".csv", ".tsv", ".rtf", ".pdf",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".heic",
		".aac", ".m4a", ".mp3", ".wav", ".ogg", ".flac", ".mov", ".mp4", ".m4v", ".webm":
		return true
	}
	return false
}

// IsSourceTextPath reports whether an ordinary Vault click uses the editor
// for this extension. Open As separately accepts any visible regular file.
func IsSourceTextPath(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".txt", ".text", ".rtf", ".log", ".csv", ".tsv", ".html", ".htm", ".xml", ".json",
		".yaml", ".yml", ".toml", ".ini", ".conf", ".cfg", ".env", ".properties",
		".css", ".scss", ".less", ".sql", ".tex", ".bib", ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs",
		".swift", ".py", ".rb", ".go", ".rs", ".java", ".kt", ".kts", ".scala", ".clj",
		".c", ".h", ".cpp", ".hpp", ".cc", ".cxx", ".m", ".mm", ".sh", ".bash", ".zsh", ".fish",
		".pl", ".php", ".lua", ".r", ".dart", ".vue", ".svelte", ".astro", ".gradle", ".groovy",
		".dockerfile", ".makefile", ".gitignore":
		return true
	}
	return false
}

// CanBrowseVaultVersions reports whether a Vault row offers document versions.
func CanBrowseVaultVersions(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".pdf", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".heic",
		".aac", ".m4a", ".mp3", ".wav", ".ogg", ".flac", ".mov", ".mp4", ".m4v", ".webm":
		return false
	}
	return true
}

// WriteVaultStats creates the generated report, or atomically replaces an
// existing generated report. On ErrUnmanagedVaultStats it returns the
// observed revision for a caller that asks before replacing the file.
func (v *Vault) WriteVaultStats(source string) (Document, error) {
	if !utf8.ValidString(source) || !hasStatsMarker(source) {
		return Document{}, errors.New("invalid generated Vault statistics")
	}
	v.writes.Lock()
	defer v.writes.Unlock()
	current, info, err := readRegularFile(v.root, VaultStatsFile)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		err = v.writeNewLifecycleFile(VaultStatsFile, source, 0o600)
	case err != nil:
		return Document{}, err
	case !hasStatsMarker(string(current)):
		return lifecycleDocument(v, VaultStatsFile, string(current)), ErrUnmanagedVaultStats
	case info.Mode().Perm()&0o222 == 0:
		return Document{}, fmt.Errorf("%s is read-only: %w", VaultStatsFile, fs.ErrPermission)
	default:
		err = replaceFile(v.root, VaultStatsFile, info, sha256.Sum256(current), source)
	}
	if err != nil {
		return Document{}, err
	}
	return lifecycleDocument(v, VaultStatsFile, source), nil
}

func hasStatsMarker(source string) bool {
	properties, err := ReadProperties(source)
	if err != nil {
		return false
	}
	for _, row := range properties.Rows {
		if row.Key == "cyberwriter-generated" && row.Kind == PropertyCheckbox && row.Text == "true" {
			return true
		}
	}
	return false
}
