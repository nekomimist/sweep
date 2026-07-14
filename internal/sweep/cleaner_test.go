package sweep

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

type mockFileInfo struct {
	name    string
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
	infoErr error
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 100 }
func (m mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }

type mockDirEntry struct {
	name string
	info *mockFileInfo
}

func (m mockDirEntry) Name() string      { return m.name }
func (m mockDirEntry) IsDir() bool       { return m.info.isDir }
func (m mockDirEntry) Type() fs.FileMode { return m.info.mode }
func (m mockDirEntry) Info() (fs.FileInfo, error) {
	if m.info.infoErr != nil {
		return nil, m.info.infoErr
	}
	return m.info, nil
}

type mockFileSystem struct {
	files       map[string]*mockFileInfo
	removed     []string
	removeErr   map[string]error
	walkErr     error
	walkEntries []walkEntry
	absErr      map[string]error
}

type walkEntry struct {
	path string
	err  error
	nil  bool
}

func newMockFileSystem() *mockFileSystem {
	return &mockFileSystem{
		files:     map[string]*mockFileInfo{},
		removeErr: map[string]error{},
		absErr:    map[string]error{},
	}
}

func (m *mockFileSystem) addFile(path string, modTime time.Time) {
	m.files[path] = &mockFileInfo{
		name:    filepath.Base(path),
		mode:    0o644,
		modTime: modTime,
	}
}

func (m *mockFileSystem) addDir(path string) {
	m.files[path] = &mockFileInfo{
		name:  filepath.Base(path),
		mode:  fs.ModeDir,
		isDir: true,
	}
}

func (m *mockFileSystem) WalkDir(root string, fn fs.WalkDirFunc) error {
	if m.walkErr != nil {
		return m.walkErr
	}
	if len(m.walkEntries) > 0 {
		for _, item := range m.walkEntries {
			var entry fs.DirEntry
			if !item.nil {
				if fileInfo, ok := m.files[item.path]; ok {
					entry = mockDirEntry{name: filepath.Base(item.path), info: fileInfo}
				}
			}
			if err := fn(item.path, entry, item.err); err != nil && !errors.Is(err, filepath.SkipDir) {
				return err
			}
		}
		return nil
	}

	paths := make([]string, 0, len(m.files))
	for path := range m.files {
		if strings.HasPrefix(path, root) {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		fileInfo := m.files[path]
		entry := mockDirEntry{name: filepath.Base(path), info: fileInfo}
		if err := fn(path, entry, nil); err != nil && !errors.Is(err, filepath.SkipDir) {
			return err
		}
	}
	return nil
}

func (m *mockFileSystem) Remove(path string) error {
	if err := m.removeErr[path]; err != nil {
		return err
	}
	if _, ok := m.files[path]; !ok {
		return fmt.Errorf("file not found: %s", path)
	}
	m.removed = append(m.removed, path)
	delete(m.files, path)
	return nil
}

func (m *mockFileSystem) Abs(path string) (string, error) {
	if err := m.absErr[path]; err != nil {
		return "", err
	}
	if strings.HasPrefix(path, "/") {
		return path, nil
	}
	return filepath.Join("/test", path), nil
}

func testConfig() Config {
	config := NewConfig()
	config.Directory = "/test"
	config.ExcludeRegexp = regexp.MustCompile(defaultExcludePattern)
	return config
}

func TestShouldDelete(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "tilde suffix", path: "test.txt~", want: true},
		{name: "bak suffix", path: "test.bak", want: true},
		{name: "normal", path: "test.txt", want: false},
		{name: "backup word", path: "test.backup", want: false},
		{name: "tilde prefix", path: "~test.txt", want: false},
		{name: "nested bak", path: "/tmp/test.txt.bak", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldDelete(tt.path); got != tt.want {
				t.Fatalf("shouldDelete(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestEmacsNumberedBackup(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		wantKey        string
		wantGeneration int
		wantOK         bool
	}{
		{name: "numbered", path: "/tmp/test.txt.~12~", wantKey: "/tmp/test.txt", wantGeneration: 12, wantOK: true},
		{name: "leading zero", path: "/tmp/test.txt.~01~", wantKey: "/tmp/test.txt", wantGeneration: 1, wantOK: true},
		{name: "zero", path: "/tmp/test.txt.~0~", wantKey: "/tmp/test.txt", wantGeneration: 0, wantOK: true},
		{name: "simple tilde", path: "/tmp/test.txt~", wantOK: false},
		{name: "non numeric", path: "/tmp/test.txt.~abc~", wantOK: false},
		{name: "missing trailing tilde", path: "/tmp/test.txt.~1", wantOK: false},
		{name: "bak", path: "/tmp/test.bak", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotGeneration, gotOK := emacsNumberedBackup(tt.path)
			if gotOK != tt.wantOK {
				t.Fatalf("emacsNumberedBackup(%q) ok = %v, want %v", tt.path, gotOK, tt.wantOK)
			}
			if gotKey != tt.wantKey || gotGeneration != tt.wantGeneration {
				t.Fatalf("emacsNumberedBackup(%q) = (%q, %d), want (%q, %d)",
					tt.path, gotKey, gotGeneration, tt.wantKey, tt.wantGeneration)
			}
		})
	}
}

func TestCleanerWithAgeRestriction(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	config.MinAgeDays = 7
	config.MinAge = 7 * 24 * time.Hour

	now := time.Now()
	fileSystem.addFile("/test/old.txt~", now.Add(-10*24*time.Hour))
	fileSystem.addFile("/test/new.txt~", now.Add(-3*24*time.Hour))
	fileSystem.addFile("/test/old.bak", now.Add(-10*24*time.Hour))
	fileSystem.addFile("/test/new.bak", now.Add(-3*24*time.Hour))
	fileSystem.addFile("/test/normal.txt", now.Add(-10*24*time.Hour))

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader(""), &out, &out).Clean()

	if err := result.Err(); err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	want := []string{"/test/old.bak", "/test/old.txt~"}
	if !reflect.DeepEqual(fileSystem.removed, want) {
		t.Fatalf("removed = %v, want %v", fileSystem.removed, want)
	}
	if result.SkippedNew != 2 {
		t.Fatalf("SkippedNew = %d, want 2", result.SkippedNew)
	}
}

func TestCleanerKeepsNewestEmacsNumberedBackups(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	config.MinAgeDays = 7
	config.MinAge = 7 * 24 * time.Hour
	config.KeepEmacsBackups = 2

	old := time.Now().Add(-10 * 24 * time.Hour)
	fileSystem.addFile("/test/foo.txt.~1~", old)
	fileSystem.addFile("/test/foo.txt.~2~", old)
	fileSystem.addFile("/test/foo.txt.~3~", old)
	fileSystem.addFile("/test/foo.txt.~4~", old)
	fileSystem.addFile("/test/bar.txt.~1~", old)
	fileSystem.addFile("/test/bar.txt.~2~", old)
	fileSystem.addFile("/test/plain.txt~", old)
	fileSystem.addFile("/test/plain.bak", old)
	fileSystem.addFile("/test/not-numbered.~abc~", old)

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader(""), &out, &out).Clean()

	if err := result.Err(); err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	want := []string{"/test/foo.txt.~1~", "/test/foo.txt.~2~", "/test/not-numbered.~abc~", "/test/plain.bak", "/test/plain.txt~"}
	if !reflect.DeepEqual(fileSystem.removed, want) {
		t.Fatalf("removed = %v, want %v", fileSystem.removed, want)
	}
	if result.SkippedKept != 4 {
		t.Fatalf("SkippedKept = %d, want 4", result.SkippedKept)
	}
}

func TestCleanerKeepsEmacsNumberedBackupsBeforeAgeCheck(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	config.MinAgeDays = 7
	config.MinAge = 7 * 24 * time.Hour
	config.KeepEmacsBackups = 1

	now := time.Now()
	fileSystem.addFile("/test/foo.txt.~1~", now.Add(-10*24*time.Hour))
	fileSystem.addFile("/test/foo.txt.~2~", now.Add(-3*24*time.Hour))

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader(""), &out, &out).Clean()

	if err := result.Err(); err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	want := []string{"/test/foo.txt.~1~"}
	if !reflect.DeepEqual(fileSystem.removed, want) {
		t.Fatalf("removed = %v, want %v", fileSystem.removed, want)
	}
	if result.SkippedKept != 1 {
		t.Fatalf("SkippedKept = %d, want 1", result.SkippedKept)
	}
	if result.SkippedNew != 0 {
		t.Fatalf("SkippedNew = %d, want 0", result.SkippedNew)
	}
}

func TestCleanerDryRun(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	config.DryRun = true
	fileSystem.addFile("/test/test.txt~", time.Now())
	fileSystem.addFile("/test/test.bak", time.Now())

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader(""), &out, &out).Clean()

	if len(fileSystem.removed) != 0 {
		t.Fatalf("removed = %v, want none", fileSystem.removed)
	}
	if result.WouldRemove != 2 {
		t.Fatalf("WouldRemove = %d, want 2", result.WouldRemove)
	}
	if !strings.Contains(out.String(), "Would remove: /test/test.bak") {
		t.Fatalf("dry-run output missing file: %q", out.String())
	}
}

func TestCleanerExcludesElmoDirectory(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	fileSystem.addDir("/test/.elmo")
	fileSystem.addFile("/test/.elmo/skip.bak", time.Now().Add(-24*time.Hour))
	fileSystem.addFile("/test/remove.bak", time.Now().Add(-24*time.Hour))
	fileSystem.walkEntries = []walkEntry{
		{path: "/test/.elmo"},
		{path: "/test/.elmo/skip.bak"},
		{path: "/test/remove.bak"},
	}

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader(""), &out, &out).Clean()

	if err := result.Err(); err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	want := []string{"/test/remove.bak"}
	if !reflect.DeepEqual(fileSystem.removed, want) {
		t.Fatalf("removed = %v, want %v", fileSystem.removed, want)
	}
}

func TestCleanerConfirmCancel(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	config.Confirm = true
	fileSystem.addFile("/test/test.bak", time.Now().Add(-24*time.Hour))

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader("no\n"), &out, &out).Clean()

	if err := result.Err(); err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	if len(fileSystem.removed) != 0 {
		t.Fatalf("removed = %v, want none", fileSystem.removed)
	}
	if !strings.Contains(out.String(), "Operation cancelled.") {
		t.Fatalf("output = %q, want cancellation message", out.String())
	}
}

func TestCleanerInteractiveDenied(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	config.Interactive = true
	fileSystem.addFile("/test/test.bak", time.Now().Add(-24*time.Hour))

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader("n\n"), &out, &out).Clean()

	if len(fileSystem.removed) != 0 {
		t.Fatalf("removed = %v, want none", fileSystem.removed)
	}
	if result.SkippedDenied != 1 {
		t.Fatalf("SkippedDenied = %d, want 1", result.SkippedDenied)
	}
}

func TestCleanerReportsRemoveFailureAndContinues(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	fileSystem.addFile("/test/a.bak", time.Now().Add(-24*time.Hour))
	fileSystem.addFile("/test/b.bak", time.Now().Add(-24*time.Hour))
	fileSystem.removeErr["/test/a.bak"] = errors.New("permission denied")

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader(""), &out, &out).Clean()

	if result.Err() == nil {
		t.Fatal("Clean() error = nil, want remove failure")
	}
	want := []string{"/test/b.bak"}
	if !reflect.DeepEqual(fileSystem.removed, want) {
		t.Fatalf("removed = %v, want %v", fileSystem.removed, want)
	}
}

func TestCleanerHandlesWalkErrorWithNilEntry(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	fileSystem.walkEntries = []walkEntry{{path: "/test/missing", err: errors.New("boom"), nil: true}}

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader(""), &out, &out).Clean()

	if result.Err() == nil {
		t.Fatal("Clean() error = nil, want walk error")
	}
}

func TestCleanerHandlesMissingEntry(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	fileSystem.walkEntries = []walkEntry{{path: "/test/missing", nil: true}}

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader(""), &out, &out).Clean()

	if result.Err() == nil {
		t.Fatal("Clean() error = nil, want missing entry error")
	}
}

func TestCleanerHandlesFileInfoError(t *testing.T) {
	fileSystem := newMockFileSystem()
	config := testConfig()
	fileSystem.addFile("/test/test.bak", time.Now().Add(-24*time.Hour))
	fileSystem.files["/test/test.bak"].infoErr = errors.New("stat failed")

	var out bytes.Buffer
	result := NewCleaner(fileSystem, config, strings.NewReader(""), &out, &out).Clean()

	if result.Err() == nil {
		t.Fatal("Clean() error = nil, want info error")
	}
}
