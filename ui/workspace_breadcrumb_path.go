package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type workspaceBreadcrumb struct {
	label, target string
	file, home    bool
}

func (app *workspaceWindow) documentBreadcrumbs() (parts []workspaceBreadcrumb, collapsed string) {
	tab := app.workspaceTab
	if tab == nil || tab.drawing != nil || !app.permitsActiveDocument() {
		return nil, ""
	}
	absolute := app.absoluteDocument(tab)
	if !filepath.IsAbs(absolute) {
		return nil, ""
	}

	const visibleParts = 3
	vaultParts := 0
	var home string
	relative, inVault := app.relativeToVault(absolute)
	if inVault {
		// Count the components of the relative path and its Vault root.
		vaultParts = strings.Count(relative, "/") + 2
	} else {
		home, _ = os.UserHomeDir()
	}
	parts = make([]workspaceBreadcrumb, 0, visibleParts)
	for target := absolute; ; {
		parent := filepath.Dir(target)
		label := filepath.Base(target)
		file := len(parts) == 0
		if parent == target {
			label = target // Keep a volume root distinguishable from another drive.
		} else if file {
			if short := strings.TrimSuffix(label, ".md"); short != "" {
				label = short
			}
		}
		atHome := len(parts)+1 == vaultParts
		if !file && home != "" {
			relativeToHome, err := filepath.Rel(home, target)
			if err == nil && relativeToHome == "." {
				label, atHome = "~", true
			}
		}
		parts = append(parts, workspaceBreadcrumb{label: label, target: target, file: file, home: atHome})
		if atHome || parent == target {
			break
		}
		if len(parts) == visibleParts {
			collapsed = absolute
			break
		}
		target = parent
	}
	slices.Reverse(parts)
	return parts, collapsed
}
