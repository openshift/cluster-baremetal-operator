package testdata

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
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
	relativePath := filepath.Join(elem...)
	targetPath := filepath.Join(fixtureDir, relativePath)

	canonicalTarget, err := filepath.Abs(targetPath)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to get absolute path for %s", targetPath))

	canonicalFixtureDir, err := filepath.Abs(fixtureDir)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get absolute path for fixture dir")

	relPath, err := filepath.Rel(canonicalFixtureDir, canonicalTarget)
	if err != nil || strings.HasPrefix(relPath, "..") {
		g.Fail(fmt.Sprintf("path traversal detected: %s escapes fixture directory", relativePath))
	}

	mutexInterface, _ := fixtureMutex.LoadOrStore(targetPath, &sync.Mutex{})
	mu := mutexInterface.(*sync.Mutex)

	mu.Lock()
	defer mu.Unlock()

	if _, err := os.Stat(targetPath); err == nil {
		return targetPath
	}

	err = os.MkdirAll(filepath.Dir(targetPath), 0755)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to create directory for %s", relativePath))

	bindataPath := relativePath
	tempDir, err := os.MkdirTemp("", "bindata-extract-")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create temp directory")
	defer os.RemoveAll(tempDir)

	if err := RestoreAsset(tempDir, bindataPath); err != nil {
		restoreErr := RestoreAssets(tempDir, bindataPath)
		o.Expect(restoreErr).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to restore fixture %s", relativePath))
	}

	extractedPath := filepath.Join(tempDir, bindataPath)

	err = filepath.Walk(extractedPath, func(path string, info os.FileInfo, err error) error {
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
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to set permissions on extracted files")

	err = os.Rename(extractedPath, targetPath)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to move extracted files")

	if info, statErr := os.Stat(targetPath); statErr == nil {
		if info.IsDir() {
			err = os.Chmod(targetPath, 0755)
		} else {
			err = os.Chmod(targetPath, 0644)
		}
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to set final permissions on %s", targetPath))
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
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get fixture data")
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
