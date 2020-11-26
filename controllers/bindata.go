// Code generated for package controllers by go-bindata DO NOT EDIT. (@generated)
// sources:
// ../manifests/0000_31_cluster-baremetal-operator_07_clusteroperator.cr.yaml
package controllers

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x8c\x90\xb1\x4e\xc4\x30\x0c\x86\xf7\x3e\x85\xd5\x3d\xe5\x6e\xcd\xca\x03\xb0\xb1\xfb\x9a\xbf\xad\x45\x6a\x47\x89\x1b\x09\x21\xde\x1d\xf5\xe0\x90\xd8\xd8\x2c\xcb\xf6\xe7\xef\xe7\x22\xaf\xa8\x4d\x4c\x23\xcd\xa6\x8b\xac\x93\x15\x68\xdb\x64\xf1\x49\xec\xa9\x5f\x87\x37\xd1\x14\xe9\x39\x1f\xcd\x51\x5f\x0a\x2a\xbb\xd5\x61\x87\x73\x62\xe7\x38\x10\x29\xef\x88\x74\xe3\x8a\xb3\x9b\x07\x22\x56\x35\x67\x17\xd3\x76\x0e\x10\x89\xce\xf9\x48\x98\x2a\x32\xb8\xe1\x2f\xa4\x21\x2f\x61\x67\xe5\x15\x29\x6c\xb2\x6e\x81\x3b\x4b\xe6\x9b\x64\xf1\xf7\x48\xa3\xd7\x03\xe3\x3f\xee\x88\xae\x19\x41\x2d\x21\x24\x74\x64\x2b\xa8\xbf\xeb\xad\x60\x8e\xf4\xf1\x39\x34\x67\x3f\xee\x7f\xf5\x6f\xf5\x7b\x1d\x7e\x34\xec\x61\x78\xe2\xfa\x23\x9b\xf1\x32\x5d\xa6\x6b\x68\xca\xa5\x6d\xe6\xe3\x57\x00\x00\x00\xff\xff\xe6\x80\xc6\x8a\x38\x01\x00\x00")

func Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYamlBytes() ([]byte, error) {
	return bindataRead(
		_Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml,
		"../manifests/0000_31_cluster-baremetal-operator_07_clusteroperator.cr.yaml",
	)
}

func Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml() (*asset, error) {
	bytes, err := Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "../manifests/0000_31_cluster-baremetal-operator_07_clusteroperator.cr.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"../manifests/0000_31_cluster-baremetal-operator_07_clusteroperator.cr.yaml": Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"..": {nil, map[string]*bintree{
		"manifests": {nil, map[string]*bintree{
			"0000_31_cluster-baremetal-operator_07_clusteroperator.cr.yaml": {Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml, map[string]*bintree{}},
		}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
