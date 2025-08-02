package migrate

import (
	"io/fs"
	"os"
	"sort"
	"strings"
)

// Migration represents a single migration.
type Migration struct {
	Version string
	Content []byte
}

// Source is an interface for migration sources.
type Source interface {
	GetMigrationFiles() ([]Migration, error)
}

// FsSource is a migration source that reads from a filesystem.
type FsSource struct {
	fs   fs.FS
	path string
}

// NewFsSource creates a new FsSource.
func NewFsSource(fs fs.FS, path string) *FsSource {
	return &FsSource{fs: fs, path: path}
}

func (s *FsSource) GetMigrationFiles() ([]Migration, error) {
	var files []Migration
	err := fs.WalkDir(s.fs, s.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".sql") {
			content, err := fs.ReadFile(s.fs, path)
			if err != nil {
				return err
			}
			files = append(files, Migration{
				Version: d.Name(),
				Content: content,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Version < files[j].Version
	})

	return files, nil
}

// OsSource is a convenience wrapper for reading from the OS filesystem.
type OsSource struct {
	*FsSource
}

// NewOsSource creates a new OsSource.
func NewOsSource(path string) *OsSource {
	return &OsSource{
		FsSource: NewFsSource(os.DirFS("/"), path),
	}
}
