package ui

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cyberwriter/internal/core/vaultcontext"
)

type aiVaultContextCache struct {
	Index         *vaultcontext.Index         `json:"index"`
	Files         map[string]aiVaultFileStamp `json:"files"`
	BuiltAt       time.Time                   `json:"builtAt"`
	BuildDuration time.Duration               `json:"buildDuration,omitempty"`
}

type aiVaultFileStamp struct {
	Modified int64 `json:"modified"`
	Size     int64 `json:"size"`
}

func aiVaultContextCacheFile(root string) (string, error) {
	settings, err := preferenceFile()
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(root))
	return filepath.Join(filepath.Dir(settings), "vault-context", fmt.Sprintf("%x.json", hash)), nil
}

func readAIVaultContextCache(root string) (aiVaultContextCache, error) {
	path, err := aiVaultContextCacheFile(root)
	if err != nil {
		return aiVaultContextCache{}, err
	}
	f, err := os.Open(aiVaultPackedPath(path))
	if err == nil {
		defer f.Close()
		return readAIVaultPacked(f)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return aiVaultContextCache{}, err
	}
	f, err = os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return aiVaultContextCache{}, nil
	}
	if err != nil {
		return aiVaultContextCache{}, err
	}
	defer f.Close()
	var cache aiVaultContextCache
	if err := json.NewDecoder(f).Decode(&cache); err != nil {
		return aiVaultContextCache{}, err
	}
	if cache.Index == nil {
		return aiVaultContextCache{}, errors.New("invalid Vault index")
	}
	return cache, nil
}

const aiVaultPackedHeader = "ONACACHE\x01"

func aiVaultPackedPath(jsonPath string) string {
	return strings.TrimSuffix(jsonPath, ".json") + ".bin"
}

func readAIVaultPacked(src io.Reader) (aiVaultContextCache, error) {
	var magic [len(aiVaultPackedHeader)]byte
	if _, err := io.ReadFull(src, magic[:]); err != nil {
		return aiVaultContextCache{}, err
	}
	if string(magic[:]) != aiVaultPackedHeader {
		return aiVaultContextCache{}, errors.New("invalid Vault cache header")
	}
	decoder := json.NewDecoder(src)
	var cache aiVaultContextCache
	if err := decoder.Decode(&cache); err != nil {
		return aiVaultContextCache{}, err
	}
	reader := bufio.NewReader(io.MultiReader(decoder.Buffered(), src))
	if newline, err := reader.ReadByte(); err != nil || newline != '\n' {
		return aiVaultContextCache{}, errors.New("invalid Vault cache metadata")
	}
	index, err := vaultcontext.ReadPacked(reader)
	if err != nil {
		return aiVaultContextCache{}, err
	}
	if _, err := reader.Peek(1); err != io.EOF {
		return aiVaultContextCache{}, errors.New("invalid Vault cache trailer")
	}
	cache.Index = index
	return cache, nil
}

func writeAIVaultPacked(dst io.Writer, cache aiVaultContextCache) error {
	index := cache.Index
	if index == nil {
		return errors.New("missing Vault index")
	}
	cache.Index = nil
	if _, err := io.WriteString(dst, aiVaultPackedHeader); err != nil {
		return err
	}
	if err := json.NewEncoder(dst).Encode(cache); err != nil {
		return err
	}
	return index.WritePacked(dst)
}

// Profiles before canonical Vault keys may contain a cache for this particular
// mount spelling. A deliberate Clear retires those old cache reads for the Vault.
func readAIVaultContextCacheForMount(root, mounted string, files map[string]aiVaultFileStamp) (aiVaultContextCache, bool, error) {
	cache, err := readAIVaultContextCache(root)
	if err != nil || cache.Index != nil || mounted == root {
		return cache, false, err
	}
	file, err := aiVaultContextCacheFile(root)
	if err != nil {
		return aiVaultContextCache{}, false, err
	}
	if _, err := os.Lstat(file + ".cleared"); !errors.Is(err, os.ErrNotExist) {
		return aiVaultContextCache{}, false, err
	}
	cache, legacy := matchingAIVaultLegacyCache(root, mounted, files)
	return cache, legacy, nil
}

// An old cache recorded only the mount spelling. Compare its file snapshot
// before using it, as that symbolic link may now point to a different Vault.
func matchingAIVaultLegacyCache(root, mounted string, files map[string]aiVaultFileStamp) (aiVaultContextCache, bool) {
	if len(files) == 0 || mounted == root || normalizedVaultEmbeddingOfferPath(mounted) != root {
		return aiVaultContextCache{}, false
	}
	cache, err := readAIVaultContextCache(mounted)
	if err != nil || cache.Index == nil || !maps.Equal(cache.Files, files) {
		return aiVaultContextCache{}, false
	}
	return cache, true
}

func clearAIVaultContextCache(root string, files map[string]aiVaultFileStamp, legacyRoots map[string]map[string]aiVaultFileStamp) (err error) {
	file, err := aiVaultContextCacheFile(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	marker := file + ".cleared"
	record, markerErr := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if !errors.Is(markerErr, os.ErrExist) {
		if markerErr != nil {
			return markerErr
		}
		defer func() {
			if err != nil {
				err = errors.Join(err, os.Remove(marker))
			}
		}()
		if err := record.Close(); err != nil {
			return err
		}
	}
	for legacy, adoptedFiles := range legacyRoots {
		if adoptedFiles == nil {
			adoptedFiles = files
		}
		if _, matches := matchingAIVaultLegacyCache(root, legacy, adoptedFiles); !matches {
			continue
		}
		cache, err := aiVaultContextCacheFile(legacy)
		if err != nil {
			return err
		}
		if err := removeAIVaultContextCachePair(cache); err != nil {
			return err
		}
	}
	return removeAIVaultContextCachePair(file)
}

func removeAIVaultContextCachePair(jsonPath string) error {
	if err := removeAIVaultContextCacheFile(jsonPath); err != nil {
		return err
	}
	return removeAIVaultContextCacheFile(aiVaultPackedPath(jsonPath))
}

func removeAIVaultContextCacheFile(file string) error {
	info, err := os.Lstat(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Vault index cache %q is not a regular file", file)
	}
	if err := os.Remove(file); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Stage beside the final file. Only the request still owned by the mounted Vault
// may publish it, so a cancelled build cannot recreate a cleared cache.
func stageAIVaultContextCache(root string, cache aiVaultContextCache) (target, stage string, err error) {
	target, err = aiVaultContextCacheFile(root)
	if err != nil {
		return "", "", err
	}
	target = aiVaultPackedPath(target)
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	f, err := os.CreateTemp(dir, ".index-*")
	if err != nil {
		return "", "", err
	}
	if err := errors.Join(writeAIVaultPacked(f, cache), f.Close()); err != nil {
		_ = os.Remove(f.Name())
		return "", "", err
	}
	return target, f.Name(), nil
}
