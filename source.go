package migrate

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Migration represents a single migration.
type Migration struct {
	Version     string
	Content     []byte
	DownContent []byte
}

// Source is an interface for migration sources.
type Source interface {
	GetMigrations() ([]Migration, error)
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

func (s *FsSource) GetMigrations() ([]Migration, error) {
	migrations := make(map[string]*Migration)

	err := fs.WalkDir(s.fs, s.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		baseName := filepath.Base(path)
		if strings.HasSuffix(baseName, ".down.sql") {
			version := strings.TrimSuffix(baseName, ".down.sql")
			if migrations[version] == nil {
				migrations[version] = &Migration{Version: version}
			}
			content, err := fs.ReadFile(s.fs, path)
			if err != nil {
				return err
			}
			migrations[version].DownContent = content
		} else if strings.HasSuffix(baseName, ".sql") {
			// support both .up.sql and .sql
			version := strings.TrimSuffix(strings.TrimSuffix(baseName, ".sql"), ".up")
			if migrations[version] == nil {
				migrations[version] = &Migration{Version: version}
			}
			content, err := fs.ReadFile(s.fs, path)
			if err != nil {
				return err
			}
			migrations[version].Content = content
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	var files []Migration
	for _, m := range migrations {
		files = append(files, *m)
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
