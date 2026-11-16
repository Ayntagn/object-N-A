// Command assets restores the pinned third-party files embedded by object-N/A.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

//go:embed assets.json
var manifest []byte

const maxDownload = 192 << 20
const maxAssetSize = 16 << 20

type source struct {
	Name      string  `json:"name"`
	URL       string  `json:"url"`
	Format    string  `json:"format"`
	Integrity string  `json:"integrity"`
	Files     []asset `json:"files"`
}

type asset struct {
	Member string `json:"member,omitempty"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func main() {
	if err := restoreAssets(); err != nil {
		fmt.Fprintln(os.Stderr, "assets:", err)
		os.Exit(1)
	}
}

func restoreAssets() error {
	var sources []source
	if err := json.Unmarshal(manifest, &sources); err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	if err := validateManifest(sources); err != nil {
		return err
	}
	root, err := os.OpenRoot(".")
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := root.Stat("go.mod"); err != nil {
		return errors.New("run go run ./cmd/assets from the repository root")
	}
	if err := restoreMyGo(root); err != nil {
		return fmt.Errorf("MyGo: %w", err)
	}
	client := &http.Client{
		Timeout: 12 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 6 || !allowedURL(req.URL) {
				return errors.New("unexpected download redirect")
			}
			return nil
		},
	}
	jobs := make(chan source)
	failures := make(chan error, len(sources))
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for s := range jobs {
				if err := restoreSource(root, client, s); err != nil {
					failures <- fmt.Errorf("%s: %w", s.Name, err)
				}
			}
		})
	}
	for _, s := range sources {
		jobs <- s
	}
	close(jobs)
	workers.Wait()
	close(failures)

	var problems []error
	for err := range failures {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}

func validateManifest(sources []source) error {
	outputs := make(map[string]bool)
	for _, s := range sources {
		u, err := url.Parse(s.URL)
		if err != nil || !allowedURL(u) || u.RawQuery != "" || s.Name == "" || len(s.Files) == 0 {
			return fmt.Errorf("invalid source %q in asset manifest", s.Name)
		}
		if _, _, err := integrity(s.Integrity); err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
		switch s.Format {
		case "tar.gz", "zip", "raw":
		default:
			return fmt.Errorf("%s: unknown source format %q", s.Name, s.Format)
		}
		members := make(map[string]bool)
		for _, a := range s.Files {
			sha, err := hex.DecodeString(a.SHA256)
			if !allowedPath(a.Path) || outputs[a.Path] || a.Size <= 0 || a.Size > maxAssetSize || err != nil || len(sha) != sha256.Size {
				return fmt.Errorf("%s: invalid or duplicate asset path %q", s.Name, a.Path)
			}
			if s.Format == "raw" {
				if a.Member != "" || len(s.Files) != 1 {
					return fmt.Errorf("%s: raw source must name one asset", s.Name)
				}
			} else if !fs.ValidPath(a.Member) || strings.ContainsAny(a.Member, "\\:") || members[a.Member] {
				return fmt.Errorf("%s: invalid or duplicate archive member %q", s.Name, a.Member)
			}
			outputs[a.Path] = true
			members[a.Member] = true
		}
	}
	if len(outputs) == 0 {
		return errors.New("asset manifest is empty")
	}
	return nil
}

func allowedURL(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return false
	}
	switch u.Hostname() {
	case "registry.npmjs.org", "cdn.jsdelivr.net", "raw.githubusercontent.com", "github.com",
		"release-assets.githubusercontent.com", "objects.githubusercontent.com", "fonts.gstatic.com":
		return true
	}
	return false
}

func allowedPath(file string) bool {
	if !fs.ValidPath(file) || strings.ContainsAny(file, "\\:") {
		return false
	}
	switch path.Dir(file) {
	case "ui/fonts":
		return strings.HasSuffix(file, ".ttf.gz")
	case "ui/webassets/pdfjs":
		return strings.HasSuffix(file, ".mjs.gz")
	case "ui/webassets/pdf-lib", "ui/webassets/mermaid", "ui/webassets/vega":
		return strings.HasSuffix(file, ".js.gz")
	case "ui/webassets/katex/fonts":
		return strings.HasSuffix(file, ".woff2")
	case "ui/webassets/katex":
		return file == "ui/webassets/katex/katex.min.js" || file == "ui/webassets/katex/katex.min.css"
	}
	return false
}

func restoreSource(root *os.Root, client *http.Client, s source) error {
	missing := make(map[string]asset)
	for _, a := range s.Files {
		if err := prepareDirectory(root, path.Dir(a.Path)); err != nil {
			return err
		}
		ok, err := assetExists(root, a)
		if err != nil {
			return err
		}
		if !ok {
			missing[a.Member] = a
		}
	}
	if len(missing) == 0 {
		fmt.Printf("Verified %s (%d assets)\n", s.Name, len(s.Files))
		return nil
	}

	fmt.Printf("Downloading %s\n", s.Name)
	f, err := download(client, s)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	files, err := extract(f, s.Format, missing)
	if err != nil {
		return err
	}
	for _, a := range s.Files {
		if data, ok := files[a.Member]; ok {
			if err := writeAsset(root, a.Path, data); err != nil {
				return fmt.Errorf("write %s: %w", a.Path, err)
			}
		}
	}
	fmt.Printf("Restored %s (%d assets)\n", s.Name, len(files))
	return nil
}

func prepareDirectory(root *os.Root, directory string) error {
	var current string
	for _, part := range strings.Split(directory, "/") {
		current = path.Join(current, part)
		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			if err := root.Mkdir(current, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
			info, err = root.Lstat(current)
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("asset directory is not a regular directory: %s", current)
		}
	}
	return nil
}

func assetExists(root *os.Root, a asset) (bool, error) {
	info, err := root.Lstat(a.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("asset path is not a regular file: %s", a.Path)
	}
	f, err := root.Open(a.Path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(a.Path, ".gz") {
		z, err := gzip.NewReader(f)
		if err != nil {
			return false, nil
		}
		defer z.Close()
		r = z
	}
	sum := sha256.New()
	n, err := io.Copy(sum, io.LimitReader(r, a.Size+1))
	return err == nil && n == a.Size && hex.EncodeToString(sum.Sum(nil)) == a.SHA256, nil
}

func integrity(value string) (hash.Hash, []byte, error) {
	algorithm, encoded, _ := strings.Cut(value, "-")
	var h hash.Hash
	var expected []byte
	var err error
	switch algorithm {
	case "sha256":
		h = sha256.New()
		expected, err = hex.DecodeString(encoded)
	case "sha512":
		h = sha512.New()
		expected, err = base64.StdEncoding.DecodeString(encoded)
	default:
		return nil, nil, fmt.Errorf("unsupported integrity algorithm %q", algorithm)
	}
	if err != nil || len(expected) != h.Size() {
		return nil, nil, fmt.Errorf("invalid %s integrity digest", algorithm)
	}
	return h, expected, nil
}

func download(client *http.Client, s source) (*os.File, error) {
	sum, expected, err := integrity(s.Integrity)
	if err != nil {
		return nil, err
	}
	resp, err := client.Get(s.URL)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxDownload {
		return nil, errors.New("download exceeds size limit")
	}
	f, err := os.CreateTemp("", "object-na-asset-download-*")
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(f.Name())
		}
	}()
	n, err := io.Copy(io.MultiWriter(f, sum), io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, err
	}
	if n > maxDownload {
		return nil, errors.New("download exceeds size limit")
	}
	if !bytes.Equal(sum.Sum(nil), expected) {
		return nil, errors.New("download integrity mismatch")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	ok = true
	return f, nil
}

func extract(f *os.File, format string, wanted map[string]asset) (map[string][]byte, error) {
	result := make(map[string][]byte)
	add := func(name string, r io.Reader) error {
		a := wanted[name]
		if _, duplicate := result[name]; duplicate {
			return fmt.Errorf("duplicate archive member %q", name)
		}
		data, err := io.ReadAll(io.LimitReader(r, a.Size+1))
		if err != nil {
			return fmt.Errorf("%s: %w", a.Path, err)
		}
		sum := sha256.Sum256(data)
		if int64(len(data)) != a.Size || hex.EncodeToString(sum[:]) != a.SHA256 {
			return fmt.Errorf("%s: content verification failed", a.Path)
		}
		result[name] = data
		return nil
	}
	switch format {
	case "raw":
		if err := add("", f); err != nil {
			return nil, err
		}
	case "tar.gz":
		z, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer z.Close()
		t := tar.NewReader(z)
		for {
			h, err := t.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if _, ok := wanted[h.Name]; !ok {
				continue
			}
			if h.Typeflag != tar.TypeReg {
				return nil, fmt.Errorf("archive member is not regular: %q", h.Name)
			}
			if err := add(h.Name, t); err != nil {
				return nil, err
			}
		}
	case "zip":
		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		z, err := zip.NewReader(f, info.Size())
		if err != nil {
			return nil, err
		}
		for _, entry := range z.File {
			if _, ok := wanted[entry.Name]; !ok {
				continue
			}
			if !entry.Mode().IsRegular() {
				return nil, fmt.Errorf("archive member is not regular: %q", entry.Name)
			}
			r, err := entry.Open()
			if err != nil {
				return nil, err
			}
			if err := errors.Join(add(entry.Name, r), r.Close()); err != nil {
				return nil, err
			}
		}
	}
	if len(result) != len(wanted) {
		for member, a := range wanted {
			if _, ok := result[member]; !ok {
				return nil, fmt.Errorf("archive member for %s is absent: %q", a.Path, member)
			}
		}
	}
	return result, nil
}

func writeAsset(root *os.Root, file string, data []byte) error {
	temp := path.Join(path.Dir(file), ".asset-"+rand.Text())
	out, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	if strings.HasSuffix(file, ".gz") {
		writer := gzip.NewWriter(out)
		_, err = writer.Write(data)
		err = errors.Join(err, writer.Close())
	} else {
		_, err = out.Write(data)
	}
	if err := errors.Join(err, out.Close()); err != nil {
		return err
	}
	return root.Rename(temp, file)
}
