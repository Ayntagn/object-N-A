package workspace

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cyberwriter/internal/core/presentation"
)

func (v *Vault) FlashcardProgress(relative string) (presentation.Progress, error) {
	native, err := v.flashcardProgressPath(relative)
	if err != nil {
		return nil, err
	}
	states, _, _, err := v.readFlashcardProgress(native)
	return states, err
}

// RateFlashcard merges a rating into the current companion file under the
// same write lock and conflict checks used for notes.
func (v *Vault) RateFlashcard(relative string, card presentation.Card, rating presentation.Rating, now time.Time) (presentation.Progress, error) {
	v.writes.Lock()
	defer v.writes.Unlock()
	native, err := v.flashcardProgressPath(relative)
	if err != nil {
		return nil, err
	}
	states, data, info, err := v.readFlashcardProgress(native)
	if err != nil {
		return nil, err
	}
	states.Rate(card, rating, now)
	encoded, err := json.Marshal(states)
	if err != nil {
		return nil, err
	}
	if info == nil {
		err = v.writeNewLifecycleFile(native, string(encoded), 0o600)
	} else {
		err = replaceFile(v.root, native, info, sha256.Sum256(data), string(encoded))
	}
	return states, err
}

func (v *Vault) ResetFlashcardProgress(relative string) error {
	v.writes.Lock()
	defer v.writes.Unlock()
	native, err := v.flashcardProgressPath(relative)
	if err != nil {
		return err
	}
	_, _, info, err := v.readFlashcardProgress(native)
	if err != nil || info == nil {
		return err
	}
	current, err := v.root.Lstat(native)
	if err != nil {
		return err
	}
	if !os.SameFile(info, current) || info.Size() != current.Size() || !info.ModTime().Equal(current.ModTime()) {
		return ErrChangedOnDisk
	}
	return v.root.Remove(native)
}

func (v *Vault) flashcardProgressPath(relative string) (string, error) {
	native, err := lifecycleNotePath(relative)
	if err != nil {
		return "", err
	}
	if err := v.checkLifecycleParent(native); err != nil {
		return "", err
	}
	info, err := v.root.Lstat(native)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%q is not a regular Markdown file", relative)
	}
	base := filepath.Base(native)
	return filepath.Join(filepath.Dir(native), "."+strings.TrimSuffix(base, filepath.Ext(base))+".flashcards.json"), nil
}

func (v *Vault) readFlashcardProgress(native string) (presentation.Progress, []byte, os.FileInfo, error) {
	data, info, err := readRegularFile(v.root, native)
	if errors.Is(err, fs.ErrNotExist) {
		return presentation.Progress{}, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	var states presentation.Progress
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, nil, nil, fmt.Errorf("read flashcard progress: %w", err)
	}
	if states == nil {
		return nil, nil, nil, errors.New("flashcard progress must be a JSON object")
	}
	for key, state := range states {
		if _, err := strconv.ParseUint(key, 16, 64); err != nil || state.Ease <= 0 || state.Interval < 0 || state.Repetitions < 0 {
			return nil, nil, nil, fmt.Errorf("invalid flashcard progress for %q", key)
		}
	}
	return states, data, info, nil
}
