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

var _Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x94\xc1\x6e\x9c\x30\x10\x86\xef\xfb\x14\x16\x77\xd3\x44\xbd\x71\xed\x03\xe4\xd6\xfb\x60\xfe\x85\x69\xcd\x8c\x65\x0f\xb4\x55\xd5\x77\xaf\x4c\x08\xd9\x55\xd5\xaa\x89\xc4\x6d\xe5\xf5\x7c\xff\xcf\x37\x02\x4a\xfc\x19\xb9\xb0\x4a\xe7\x82\xca\x95\xc7\x56\x13\xa4\x4c\x7c\xb5\x96\xf5\xc3\xfa\x78\xf9\xca\x32\x74\xee\x53\x5c\x8a\x21\x3f\x25\x64\x32\xcd\x97\x19\x46\x03\x19\x75\x17\xe7\x84\x66\x74\xae\xa7\x8c\x7a\x1a\x2f\xce\x91\x88\x1a\x19\xab\x94\x7a\xc1\xb9\x40\x89\x7a\x8e\x6c\x3f\xee\xf9\x7f\x8e\x3a\x87\xef\x21\x2e\x03\xda\x8c\x08\x2a\xb8\x1f\x60\x31\x64\xa1\xe8\x8f\x53\x3f\x69\x31\x0c\x9d\x6b\x2c\x2f\x68\x36\x04\xcb\x3f\x10\x05\xf1\xea\x67\x12\x1a\x31\xf8\x89\xc7\xc9\xd3\x4a\x1c\xf7\x7e\x6f\xe0\xb0\x8c\x11\x5e\x74\x80\x1f\xb0\x22\x6a\x42\x3e\xc6\x4b\x42\xe8\xdc\xcf\x5f\x97\x62\x64\xcb\xa6\x61\x7d\x36\xbd\xfd\xf6\xbb\x35\x7d\x11\x5a\xe3\xd6\x97\x55\x34\x0f\xed\x43\xfb\xe8\x8b\x50\x2a\x93\x5a\x2d\x93\x11\xc9\x30\x3c\xf5\x5f\x10\x6c\x47\x8c\x59\x97\xd4\xb9\xe6\xb9\xec\x01\xdc\xbd\xcc\x14\x26\x16\x78\x4a\xbc\xfd\x9f\x51\x74\xc9\x01\xdd\x76\xb3\x24\x0a\x28\x77\x98\x6d\x05\x1f\x5b\xd6\x5b\xde\x0e\x7f\x1d\x6e\x8e\x6d\x55\xf1\xe5\xf5\xee\x46\xfc\x7b\x81\x37\xe7\xa4\xac\x2b\x57\x1f\x2c\x63\xf3\x1e\x40\xed\x77\xe5\x3c\x7f\xa3\x8c\x02\x33\x96\xf1\xc4\xba\x47\x52\x98\x30\xd3\xa9\x5e\x70\xab\x86\x67\x1a\x71\x62\x5c\x3f\x07\xac\x10\x2b\x4b\x5f\x42\xe6\xb4\xbd\xd5\xe7\xc5\xdd\x2e\x2d\xe8\x9c\x54\x6a\xf6\x79\x79\xf5\x23\x76\xb6\xc2\xfa\x4c\x4b\x1a\xc8\x90\x34\x72\xe0\xff\xcd\xfa\x1d\x00\x00\xff\xff\x4f\x5a\x07\x67\x9e\x05\x00\x00")

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
