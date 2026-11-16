//go:build !darwin

package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
)

func readAIReferencePDFPages(ctx context.Context, pdf []byte) ([]string, error) {
	if !mygo.App.IsReady() || !mygo.WebViewAvailable() {
		return nil, errors.New("reading PDF text requires the system WebView")
	}
	dir, err := os.MkdirTemp("", "object-na-pdf-text-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	filename := filepath.Join(dir, "extract.html")
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	output := bufio.NewWriter(file)
	if err := errors.Join(writeAIReferencePDFPage(ctx, output, pdf), output.Flush(), file.Close(), ctx.Err()); err != nil {
		return nil, err
	}

	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "PDF text reader", Hidden: true, SkipTaskbar: true,
		Page: mygo.PageOptions{Untrusted: true, DevTools: mygo.DevToolsDisabled, BackgroundThrottling: mygo.ThrottlingOff},
	})
	defer win.Destroy()
	page := win.Page()
	loaded := make(chan error, 1)
	notify := func(err error) {
		select {
		case loaded <- err:
		default:
		}
	}
	page.OnDidFinishLoad(func() { notify(nil) })
	page.OnDidFailLoad(func(err *mygo.LoadError) { notify(err) })
	page.OnRenderProcessGone(func(reason string) { notify(fmt.Errorf("PDF text reader stopped: %s", reason)) })
	page.OnWillNavigate(func(event *mygo.NavigateEvent) { event.PreventDefault() })
	win.OnClosed(func() { notify(errors.New("the PDF text reader closed")) })
	if err := page.LoadFile(filename); err != nil {
		return nil, err
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case err := <-loaded:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, errors.New("timed out loading the PDF text reader")
	}
	value, err := page.EvalContext(ctx, "window.extractAIReferencePDFPages()")
	if err != nil {
		return nil, err
	}
	result, ok := value.([]any)
	if !ok {
		return nil, errors.New("the PDF text reader returned an invalid result")
	}
	pages := make([]string, len(result))
	for index, item := range result {
		text, ok := item.(string)
		if !ok {
			return nil, errors.New("the PDF text reader returned an invalid page")
		}
		pages[index] = text
	}
	return pages, nil
}
