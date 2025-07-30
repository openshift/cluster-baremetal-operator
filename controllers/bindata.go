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

var _Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x94\x41\x8e\x9c\x40\x0c\x45\xf7\x7d\x8a\x12\x7b\xc8\x8c\xb2\x63\x9b\x03\xcc\x2e\x7b\x53\xfc\x06\x27\x85\x5d\x2a\x1b\xa2\x28\xca\xdd\xa3\xa2\x7b\x7a\xba\x15\x45\x9a\x8c\xc4\x0e\x41\xf9\xfd\xef\x87\x80\x32\x7f\x45\x31\x56\xe9\x43\x54\x39\xf3\xd4\x69\x86\xd8\xcc\x67\xef\x58\x3f\x6d\xcf\xa7\xef\x2c\x63\x1f\xbe\xa4\xd5\x1c\xe5\x25\xa3\x90\x6b\x39\x2d\x70\x1a\xc9\xa9\x3f\x85\x20\xb4\xa0\x0f\x03\x15\xd4\xbb\xe9\x14\x02\x89\xa8\x93\xb3\x8a\xd5\x03\x21\x44\xca\x34\x70\x62\xff\xf9\xc8\xff\x7b\x34\x04\x96\x98\xd6\x11\x5d\x41\x02\x19\x1e\x07\x0c\xe9\xdc\x2e\x24\x34\x61\x6c\x67\x9e\xe6\x96\x36\xe2\x74\x85\xf7\xa1\xf1\xb2\xa2\x79\x07\x87\x65\x4a\x68\x45\x47\xb4\x23\x36\x24\xcd\x28\xb7\x71\xcb\x88\x7d\xf8\xf5\xfb\x64\x4e\xbe\xee\x3b\x6c\x17\x4d\xfb\x75\x7b\x5d\x59\x5f\x6d\xd4\xb8\xed\xd5\x63\xf3\xd4\x3d\x75\xcf\xad\x09\x65\x9b\xd5\x6b\x99\x82\x44\x8e\xf1\x65\xf8\x86\xe8\x57\xc4\x54\x74\xcd\x7d\x68\x2e\x65\x6f\xc0\x4b\xc5\x76\xa1\x38\xb3\xa0\xa5\xcc\xfb\xf3\x02\xd3\xb5\x44\xf4\xfb\x49\xcb\x14\x61\x0f\x98\xdd\xdf\xe7\x8e\xf5\x9e\x77\x85\xbf\x0d\x37\x37\xd5\xb3\x9a\xdb\xdb\xd9\x9d\xf8\xef\x02\xff\x9d\x93\x8b\x6e\x5c\x7d\xb0\x4c\xcd\x47\x00\xb5\xdf\x99\xcb\xf2\x83\x0a\x0c\xee\x2c\xd3\x81\x75\x6f\x49\x71\xc6\x42\x87\x7a\xc1\xbd\x1a\x5e\x68\xc2\x81\x71\xc3\x12\xb1\x41\xdc\xd6\xc1\x62\xe1\xbc\x7f\x92\xc7\xc5\xdd\xbf\xb4\xa8\x4b\x56\xa9\xd9\xc7\xe5\xd5\x3f\xd0\xd1\x0a\xeb\x4e\x6b\x1e\xc9\x91\x35\x71\xe4\x77\x67\xfd\x09\x00\x00\xff\xff\xd5\xa7\xaa\xc1\x5c\x05\x00\x00")

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
