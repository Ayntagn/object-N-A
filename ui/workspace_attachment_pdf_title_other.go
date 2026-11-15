//go:build !darwin

package ui

func bindPDFDocumentTitle(*workspacePDFWindow, *workspaceApplication) {}

func setPDFDocumentURL(*workspacePDFWindow) {}
