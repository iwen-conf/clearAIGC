package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Layout struct {
	RootDir string
}

func NewLayout(rootDir string) *Layout {
	return &Layout{RootDir: rootDir}
}

func (l *Layout) Ensure() error {
	return os.MkdirAll(l.RootDir, 0o755)
}

func (l *Layout) SessionDir(sessionID uuid.UUID) string {
	return filepath.Join(l.RootDir, "sessions", sessionID.String())
}

func (l *Layout) SessionInputDir(sessionID uuid.UUID) string {
	return filepath.Join(l.SessionDir(sessionID), "input")
}

func (l *Layout) SessionRoundDir(sessionID uuid.UUID, round int) string {
	return filepath.Join(l.SessionDir(sessionID), fmt.Sprintf("round-%d", round))
}

func (l *Layout) SessionExportDir(sessionID uuid.UUID) string {
	return filepath.Join(l.SessionDir(sessionID), "exports")
}

func (l *Layout) OriginalPath(sessionID uuid.UUID, filename string) string {
	return filepath.Join(l.SessionInputDir(sessionID), sanitizeFilename(filename))
}

func (l *Layout) RoundInputPath(sessionID uuid.UUID, round int) string {
	return filepath.Join(l.SessionRoundDir(sessionID, round), "input.txt")
}

func (l *Layout) RoundOutputPath(sessionID uuid.UUID, round int) string {
	return filepath.Join(l.SessionRoundDir(sessionID, round), "output.txt")
}

func (l *Layout) RoundExportPath(sessionID uuid.UUID, round int, format string) string {
	return filepath.Join(l.SessionExportDir(sessionID), fmt.Sprintf("round-%d.%s", round, sanitizeFilename(format)))
}

func (l *Layout) EnsureSession(sessionID uuid.UUID, rounds ...int) error {
	paths := []string{
		l.SessionInputDir(sessionID),
		l.SessionExportDir(sessionID),
	}
	for _, round := range rounds {
		paths = append(paths, l.SessionRoundDir(sessionID, round))
	}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if name == "." || name == "" {
		return "document"
	}
	return name
}
