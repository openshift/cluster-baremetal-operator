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

var _Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x54\xcd\x8e\xd4\x30\x0c\xbe\xf7\x29\xa2\xde\x53\x76\xc5\x05\xf5\xca\x03\xec\x8d\xbb\x9b\x7a\x5a\xb3\x49\x1c\xd9\x4e\x11\x20\xde\x1d\xb5\xd3\xe9\xee\x80\x10\xda\x95\x7a\xab\x52\x7f\x3f\xfe\x2c\x1b\x0a\x7d\x41\x51\xe2\xdc\xbb\xc0\xf9\x42\x53\xc7\x05\xb3\xce\x74\xb1\x8e\xf8\xc3\xf2\xd8\x3c\x53\x1e\x7b\xf7\x39\x56\x35\x94\xa7\x82\x02\xc6\xd2\x24\x34\x18\xc1\xa0\x6f\x9c\xcb\x90\xb0\x77\x03\x08\xae\xaf\xb1\x71\x0e\x72\x66\x03\x23\xce\xba\x16\x38\x17\xa0\xc0\x40\x91\xec\xfb\x3d\xff\xdf\x50\xe7\x28\x87\x58\x47\xec\x04\x23\x82\xe2\x3d\x80\x86\xe4\x43\xe4\x3a\xfa\x04\x19\x26\x1c\x7b\xd7\x9a\x54\x6c\xff\x0f\x55\x8c\x97\x1b\xca\xcf\x34\xcd\x1e\x16\xa0\xb8\xfb\x7a\x03\x0f\xe5\x29\xa2\xcf\x3c\xa2\x1f\x71\xc1\xc8\x05\xe5\x80\x6b\xc1\xd0\xbb\x9f\xbf\x1a\x35\xb0\xba\xb5\xbf\x5c\x13\xde\xbe\xfd\x9e\x16\xdf\x82\x5c\xe5\x96\xdb\x08\xda\x87\xee\xa1\x7b\xf4\x9a\xa1\xe8\xcc\xb6\x9a\x11\x8c\x60\x38\x3e\x0d\x5f\x31\xd8\x4e\x31\x09\xd7\xd2\xbb\xf6\x6a\xf6\x20\xbc\x5a\xf4\x09\xc2\x4c\x19\x3d\x14\xda\xfe\x0b\x2a\x57\x09\xd8\x6f\x95\x5a\x20\xa0\xde\xd1\x6c\xd1\x7f\xec\x88\x5f\xf3\xed\xe4\x2f\xe0\xf6\x98\xd2\xcc\x6a\xfa\x52\xbb\x31\xfe\xdb\xc0\x9b\x75\x8a\xf0\x42\x6b\x1e\x94\xa7\xf6\x3d\x04\xab\xbf\x0b\x49\xfa\x06\x82\x8a\x66\x94\xa7\x13\xed\x1e\x4a\x61\xc6\x04\xa7\xe6\x82\xaf\xa3\xa1\x04\x13\x9e\x28\x37\xa4\x80\x0b\x66\xd3\x3a\x68\x10\x2a\xdb\x36\x9f\x27\xf7\x7a\x68\x81\x53\xe1\xbc\x6a\x9f\xa7\xb7\x1e\xaf\xb3\x23\x5c\x7b\xaa\x65\x04\xc3\xc2\x91\x02\xbd\x47\x4b\x06\x08\x1d\x54\x9b\x59\xe8\xc7\x76\x52\xbb\xe7\x4f\xda\x11\xff\x21\x16\xae\x07\x5a\x38\x6e\xdb\x7d\x33\xb5\x3f\xfb\x63\x79\xfd\x71\x79\x9a\xdf\x01\x00\x00\xff\xff\x9f\x28\xa5\x9e\xfd\x05\x00\x00")

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
