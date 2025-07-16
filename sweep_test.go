package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// MockFileInfo implements fs.FileInfo
type MockFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (m *MockFileInfo) Name() string       { return m.name }
func (m *MockFileInfo) Size() int64        { return m.size }
func (m *MockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m *MockFileInfo) ModTime() time.Time { return m.modTime }
func (m *MockFileInfo) IsDir() bool        { return m.isDir }
func (m *MockFileInfo) Sys() interface{}   { return nil }

// MockDirEntry implements fs.DirEntry
type MockDirEntry struct {
	name     string
	isDir    bool
	fileInfo *MockFileInfo
}

func (m *MockDirEntry) Name() string               { return m.name }
func (m *MockDirEntry) IsDir() bool                { return m.isDir }
func (m *MockDirEntry) Type() fs.FileMode          { return m.fileInfo.Mode() }
func (m *MockDirEntry) Info() (fs.FileInfo, error) { return m.fileInfo, nil }

// MockFileSystem implements FileSystem for testing
type MockFileSystem struct {
	files        map[string]*MockFileInfo
	removedFiles []string
	walkError    error
}

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files:        make(map[string]*MockFileInfo),
		removedFiles: make([]string, 0),
	}
}

func (m *MockFileSystem) AddFile(path string, modTime time.Time, isDir bool) {
	m.files[path] = &MockFileInfo{
		name:    filepath.Base(path),
		size:    100,
		mode:    0644,
		modTime: modTime,
		isDir:   isDir,
	}
}

func (m *MockFileSystem) WalkDir(root string, fn fs.WalkDirFunc) error {
	if m.walkError != nil {
		return m.walkError
	}

	for path, fileInfo := range m.files {
		if strings.HasPrefix(path, root) {
			dirEntry := &MockDirEntry{
				name:     filepath.Base(path),
				isDir:    fileInfo.isDir,
				fileInfo: fileInfo,
			}
			if err := fn(path, dirEntry, nil); err != nil {
				if err == filepath.SkipDir {
					continue
				}
				return err
			}
		}
	}
	return nil
}

func (m *MockFileSystem) Remove(path string) error {
	if _, exists := m.files[path]; !exists {
		return fmt.Errorf("file not found: %s", path)
	}
	m.removedFiles = append(m.removedFiles, path)
	delete(m.files, path)
	return nil
}

func (m *MockFileSystem) Abs(path string) (string, error) {
	if strings.HasPrefix(path, "/") {
		return path, nil
	}
	return filepath.Join("/test", path), nil
}

func (m *MockFileSystem) GetRemovedFiles() []string {
	return m.removedFiles
}

func TestShouldDelete(t *testing.T) {
	fs := NewMockFileSystem()
	config := NewConfig()
	cleaner := NewCleaner(fs, config)

	tests := []struct {
		filename string
		expected bool
	}{
		{"test.txt~", true},
		{"test.bak", true},
		{"test.txt", false},
		{"test.backup", false},
		{"~test.txt", false},
		{"test.txt.bak", true},
		{"test.txt.backup", false},
	}

	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			result := cleaner.shouldDelete(test.filename)
			if result != test.expected {
				t.Errorf("shouldDelete(%s) = %v; expected %v", test.filename, result, test.expected)
			}
		})
	}
}

func TestCleanerWithAgeRestriction(t *testing.T) {
	fs := NewMockFileSystem()
	config := NewConfig()
	config.MinAgeDays = 7
	config.MinAge = 7 * 24 * time.Hour
	config.Directory = "/test"
	config.ExcludeRegexp = regexp.MustCompile(`[\\/]\.elmo[\\/]`)

	// Add files with different ages
	now := time.Now()
	oldFile := now.Add(-10 * 24 * time.Hour) // 10 days old
	newFile := now.Add(-3 * 24 * time.Hour)  // 3 days old

	fs.AddFile("/test/old.txt~", oldFile, false)
	fs.AddFile("/test/new.txt~", newFile, false)
	fs.AddFile("/test/old.bak", oldFile, false)
	fs.AddFile("/test/new.bak", newFile, false)
	fs.AddFile("/test/normal.txt", oldFile, false)

	cleaner := NewCleaner(fs, config)
	err := cleaner.Clean()

	if err != nil {
		t.Fatalf("Clean() failed: %v", err)
	}

	removed := fs.GetRemovedFiles()
	expectedRemoved := []string{"/test/old.txt~", "/test/old.bak"}

	if len(removed) != len(expectedRemoved) {
		t.Errorf("Expected %d files to be removed, got %d", len(expectedRemoved), len(removed))
	}

	for _, expected := range expectedRemoved {
		found := false
		for _, actual := range removed {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file %s to be removed", expected)
		}
	}
}

func TestCleanerDryRun(t *testing.T) {
	fs := NewMockFileSystem()
	config := NewConfig()
	config.DryRun = true
	config.Directory = "/test"
	config.ExcludeRegexp = regexp.MustCompile(`[\\/]\.elmo[\\/]`)

	now := time.Now()
	fs.AddFile("/test/test.txt~", now, false)
	fs.AddFile("/test/test.bak", now, false)

	cleaner := NewCleaner(fs, config)
	err := cleaner.Clean()

	if err != nil {
		t.Fatalf("Clean() failed: %v", err)
	}

	removed := fs.GetRemovedFiles()
	if len(removed) != 0 {
		t.Errorf("Expected no files to be removed in dry run mode, got %d", len(removed))
	}
}

func TestConfigParseFlags(t *testing.T) {
	config := NewConfig()

	// Test default values
	if config.Directory != "." {
		t.Errorf("Expected default directory to be '.', got '%s'", config.Directory)
	}

	if config.MinAgeDays != 0 {
		t.Errorf("Expected default MinAgeDays to be 0, got %d", config.MinAgeDays)
	}

	if config.ExcludePattern != `[\\/]\.elmo[\\/]` {
		t.Errorf("Expected default exclude pattern, got '%s'", config.ExcludePattern)
	}
}

func TestFileSystemInterface(t *testing.T) {
	// Test that RealFileSystem implements FileSystem
	var fs FileSystem = &RealFileSystem{}
	if fs == nil {
		t.Error("RealFileSystem should implement FileSystem interface")
	}

	// Test that MockFileSystem implements FileSystem
	var mockFs FileSystem = NewMockFileSystem()
	if mockFs == nil {
		t.Error("MockFileSystem should implement FileSystem interface")
	}
}
