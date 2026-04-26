package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
)

// RotateLog implements a 3-deep cascade rename. Cascading from oldest:
//
//	.log.3 (oldest) → deleted
//	.log.2 → .log.3
//	.log.1 → .log.2
//	.log   → .log.1
//
// Caller then opens a fresh .log for writing.
//
// Idempotent on a missing source file: the first session has no .log to
// rotate; RotateLog returns nil. Likewise tolerant of any missing .log.N
// in the cascade.
func RotateLog(path string) error {
	const keep = 3
	// Evict oldest.
	oldest := path + "." + strconv.Itoa(keep)
	if err := os.Remove(oldest); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", oldest, err)
	}
	// Cascade .log.N-1 → .log.N for N from keep down to 2.
	for n := keep; n > 1; n-- {
		from := path + "." + strconv.Itoa(n-1)
		to := path + "." + strconv.Itoa(n)
		if err := renameIfExists(from, to); err != nil {
			return err
		}
	}
	// Finally .log → .log.1.
	return renameIfExists(path, path+".1")
}

func renameIfExists(from, to string) error {
	if _, err := os.Stat(from); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", from, err)
	}
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", from, to, err)
	}
	return nil
}
