package sweep

import (
	"io/fs"
	"os"
	"path/filepath"
)

// FileSystem abstracts file system operations for testing.
type FileSystem interface {
	WalkDir(root string, fn fs.WalkDirFunc) error
	Remove(path string) error
	Abs(path string) (string, error)
}

// RealFileSystem implements FileSystem using the host file system.
type RealFileSystem struct{}

func (rfs RealFileSystem) WalkDir(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, fn)
}

func (rfs RealFileSystem) Remove(path string) error {
	return os.Remove(path)
}

func (rfs RealFileSystem) Abs(path string) (string, error) {
	return filepath.Abs(path)
}
