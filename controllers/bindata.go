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

var _Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x94\xcd\xae\xd4\x30\x0c\x85\xf7\x7d\x8a\xa8\xfb\x94\x7b\xc5\xae\x5b\x1e\xe0\xee\xd8\xbb\xa9\xa7\x35\x24\x76\x14\x3b\x45\x08\xf1\xee\x28\x9d\x9f\x3b\x23\x84\x60\x90\xba\xab\x5a\x9f\xef\x9c\x1c\xab\x81\x4c\x9f\xb1\x28\x09\x8f\x2e\x08\x9f\x68\x19\x24\x23\xeb\x4a\x27\x1b\x48\x3e\x6c\xaf\xdd\x57\xe2\x79\x74\x9f\x62\x55\xc3\xf2\x96\xb1\x80\x49\xe9\x12\x1a\xcc\x60\x30\x76\xce\x31\x24\x1c\xdd\x04\x05\xdb\xdb\xd8\x39\x07\xcc\x62\x60\x24\xac\x6d\xc0\xb9\x00\x19\x26\x8a\x64\xdf\x1f\xf9\xbf\x4b\x9d\x23\x0e\xb1\xce\x38\x14\x8c\x08\x8a\x8f\x02\x9a\x92\x0f\x51\xea\xec\x13\x30\x2c\x38\x8f\xae\xb7\x52\xb1\xff\xbb\x54\x31\x9e\xae\x2a\xbf\xd2\xb2\x7a\xd8\x80\xe2\x25\xd7\x13\x1c\xe2\x25\xa2\x67\x99\xd1\xcf\xb8\x61\x94\x8c\xe5\x26\xd7\x8c\x61\x74\x3f\x7e\x76\x6a\x60\x75\x3f\xfe\x76\x6e\x78\x7f\xf6\x97\xb6\xe4\x5a\x64\xb3\xdb\xae\x2b\xe8\x5f\x86\x97\xe1\xd5\x2b\x43\xd6\x55\xac\x85\x29\x18\xc1\x70\x7e\x9b\xbe\x60\xb0\x0b\x62\x29\x52\xf3\xe8\xfa\x73\xd8\x1b\xf0\x1c\xd1\x27\x08\x2b\x31\x7a\xc8\xb4\x7f\x2f\xa8\x52\x4b\xc0\x71\x9f\xd4\x0c\x01\xf5\x01\xb3\x57\xff\x71\x20\xb9\xe7\x5d\xe0\xef\xe2\xfe\xb6\xa5\x55\xd4\xf4\x7d\x76\x27\xfe\x39\xc0\xd3\x3e\xb9\xc8\x46\xad\x0f\xe2\xa5\xff\x1f\x40\xcb\x77\xa2\x92\xbe\x41\x41\x45\x33\xe2\xe5\xc0\xb8\x37\xa7\xb0\x62\x82\x43\x7b\xc1\xfb\x6a\x28\xc1\x82\x07\xda\x4d\x29\xe0\x86\x6c\x5a\x27\x0d\x85\xf2\xfe\x37\x1f\x67\x77\xbf\xb4\x20\x29\x0b\x37\xef\xe3\xfc\xda\xe5\x75\x74\x85\xed\x4c\x35\xcf\x60\x98\x25\x52\xa0\x7f\xf6\xfa\x15\x00\x00\xff\xff\x34\xf1\x16\xea\x97\x05\x00\x00")

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
//
//	data/
//	  foo.txt
//	  img/
//	    a.png
//	    b.png
//
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
