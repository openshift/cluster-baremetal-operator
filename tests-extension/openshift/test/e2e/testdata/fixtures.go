package testdata

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	fixtureDir    string
	fixtureMutex  sync.Map // Map of path -> *sync.Mutex for per-fixture synchronization
)

func init() {
	var err error
	fixtureDir, err = ioutil.TempDir("", "testdata-fixtures-")
	if err != nil {
		panic(fmt.Sprintf("failed to create fixture directory: %v", err))
	}
	// Ensure fixture directory has proper permissions for all users
	if err := os.Chmod(fixtureDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to set fixture directory permissions: %v", err))
	}
}

func FixturePath(elem ...string) string {
	// Validate input elements to prevent path traversal attacks
	for _, e := range elem {
		// Check for absolute paths
		if filepath.IsAbs(e) {
			panic(fmt.Sprintf("absolute path not allowed in fixture path: %s", e))
		}
		// Check for parent directory references
		if e == ".." || strings.Contains(e, ".."+string(filepath.Separator)) || strings.HasPrefix(e, ".."+string(filepath.Separator)) || strings.HasSuffix(e, string(filepath.Separator)+"..") {
			panic(fmt.Sprintf("parent directory references not allowed in fixture path: %s", e))
		}
	}

	relativePath := filepath.Join(elem...)
	targetPath := filepath.Join(fixtureDir, relativePath)

	// Verify the canonical path stays within fixtureDir boundary
	// This prevents traversal attacks even if filepath.Join doesn't fully sanitize
	canonicalTarget, err := filepath.Abs(targetPath)
	if err != nil {
		panic(fmt.Sprintf("failed to get absolute path for %s: %v", targetPath, err))
	}
	canonicalFixtureDir, err := filepath.Abs(fixtureDir)
	if err != nil {
		panic(fmt.Sprintf("failed to get absolute path for fixture dir: %v", err))
	}
	// Check if canonicalTarget is within canonicalFixtureDir
	relPath, err := filepath.Rel(canonicalFixtureDir, canonicalTarget)
	if err != nil || strings.HasPrefix(relPath, "..") {
		panic(fmt.Sprintf("path traversal detected: %s escapes fixture directory", relativePath))
	}

	// Get or create a mutex for this specific fixture path to prevent race conditions
	mutexInterface, _ := fixtureMutex.LoadOrStore(targetPath, &sync.Mutex{})
	mu := mutexInterface.(*sync.Mutex)

	// Lock to ensure only one goroutine extracts this fixture at a time
	mu.Lock()
	defer mu.Unlock()

	// Check again if file exists (might have been created while waiting for lock)
	if _, err := os.Stat(targetPath); err == nil {
		return targetPath
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		panic(fmt.Sprintf("failed to create directory for %s: %v", relativePath, err))
	}

	bindataPath := relativePath
	tempDir, err := os.MkdirTemp("", "bindata-extract-")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp directory: %v", err))
	}
	defer os.RemoveAll(tempDir)

	if err := RestoreAsset(tempDir, bindataPath); err != nil {
		if err := RestoreAssets(tempDir, bindataPath); err != nil {
			panic(fmt.Sprintf("failed to restore fixture %s: %v", relativePath, err))
		}
	}

	extractedPath := filepath.Join(tempDir, bindataPath)

	// Set permissions on extracted files/directories before moving
	if err := filepath.Walk(extractedPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := os.Chmod(path, 0755); err != nil {
				return fmt.Errorf("failed to chmod directory %s: %v", path, err)
			}
		} else {
			if err := os.Chmod(path, 0644); err != nil {
				return fmt.Errorf("failed to chmod file %s: %v", path, err)
			}
		}
		return nil
	}); err != nil {
		panic(fmt.Sprintf("failed to set permissions on extracted files: %v", err))
	}

	// Use os.Rename to move the extracted file
	// With mutex protection, no race condition should occur
	if err := os.Rename(extractedPath, targetPath); err != nil {
		panic(fmt.Sprintf("failed to move extracted files: %v", err))
	}

	// Ensure final path has correct permissions
	if info, err := os.Stat(targetPath); err == nil {
		if info.IsDir() {
			if err := os.Chmod(targetPath, 0755); err != nil {
				panic(fmt.Sprintf("failed to set final directory permissions: %v", err))
			}
		} else {
			if err := os.Chmod(targetPath, 0644); err != nil {
				panic(fmt.Sprintf("failed to set final file permissions: %v", err))
			}
		}
	}

	return targetPath
}

func CleanupFixtures() error {
	if fixtureDir != "" {
		return os.RemoveAll(fixtureDir)
	}
	return nil
}

func GetFixtureData(elem ...string) ([]byte, error) {
	relativePath := filepath.Join(elem...)
	cleanPath := relativePath
	if len(cleanPath) > 0 && cleanPath[0] == '/' {
		cleanPath = cleanPath[1:]
	}
	return Asset(cleanPath)
}

func MustGetFixtureData(elem ...string) []byte {
	data, err := GetFixtureData(elem...)
	if err != nil {
		panic(fmt.Sprintf("failed to get fixture data: %v", err))
	}
	return data
}

func FixtureExists(elem ...string) bool {
	relativePath := filepath.Join(elem...)
	cleanPath := relativePath
	if len(cleanPath) > 0 && cleanPath[0] == '/' {
		cleanPath = cleanPath[1:]
	}
	_, err := Asset(cleanPath)
	return err == nil
}

func ListFixtures() []string {
	names := AssetNames()
	fixtures := make([]string, 0, len(names))
	for _, name := range names {
		// Filter out generated Go source files (bindata.go, fixtures.go)
		// Include only fixture files (YAML, JSON, etc.)
		if !strings.HasSuffix(name, ".go") {
			fixtures = append(fixtures, name)
		}
	}
	sort.Strings(fixtures)
	return fixtures
}
