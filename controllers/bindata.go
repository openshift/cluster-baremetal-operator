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

var _Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x93\x41\x8e\x9c\x40\x0c\x45\xf7\x9c\xa2\xc4\xbe\xc8\x8c\xb2\x63\x9b\x03\xcc\x2e\x7b\x53\xfc\x06\x27\x85\xab\x64\x1b\xa2\x28\xca\xdd\x23\xe8\x9e\x9e\x69\x45\x91\x3a\x23\xf5\x0e\x81\xfd\xfe\xf7\x93\xa0\xca\x5f\xa1\xc6\x45\xfa\x90\x8a\x9c\x78\xea\x4a\x85\xd8\xcc\x27\xef\xb8\x7c\xda\x9e\x9b\xef\x2c\x63\x1f\xbe\xe4\xd5\x1c\xfa\x52\xa1\xe4\x45\x9b\x05\x4e\x23\x39\xf5\x4d\x08\x42\x0b\xfa\x30\x90\x62\x7f\x9b\x9b\x10\x48\xa4\x38\x39\x17\xb1\x7d\x20\x84\x44\x95\x06\xce\xec\x3f\x6f\xf9\x7f\xaf\x86\xc0\x92\xf2\x3a\xa2\x53\x64\x90\xe1\x76\xc1\x90\x4f\x71\x21\xa1\x09\x63\x9c\x79\x9a\x23\x6d\xc4\xf9\x02\xef\x43\xeb\xba\xa2\xbd\x83\xc3\x32\x65\x44\x29\x23\xe2\x88\x0d\xb9\x54\xe8\x75\xdd\x2a\x52\x1f\x7e\xfd\x6e\xcc\xc9\xd7\xe3\x86\xed\xac\xe9\x78\x8e\x97\x93\xcb\xab\x8d\x3d\x6e\x7b\xf5\xd8\x3e\x75\x4f\xdd\x73\x34\xa1\x6a\x73\xf1\xbd\x8c\x22\x93\x63\x7c\x19\xbe\x21\xf9\x05\x31\x69\x59\x6b\x1f\xda\x73\xd9\x2b\xf0\x5c\x31\x2e\x94\x66\x16\x44\xaa\x7c\x7c\x57\x58\x59\x35\xa1\x3f\x26\xad\x52\x82\xdd\x60\x0e\x7f\x9f\x3b\x2e\xef\x79\x17\xf8\xdb\x72\x7b\x55\x3d\x17\x73\x7b\x9b\x3d\x88\xff\x2e\xf0\xdf\x39\x55\xcb\xc6\xbb\x0f\x96\xa9\xfd\x08\x60\xef\x77\x62\x5d\x7e\x90\xc2\xe0\xce\x32\x3d\xb0\xee\x35\x29\xcd\x58\xe8\xa1\x5e\xf0\x5e\x0d\x2f\x34\xe1\x81\x71\xc3\x92\xb0\x41\xdc\xd6\xc1\x92\x72\x3d\x7e\xc9\x3b\xe3\xfe\x04\x00\x00\xff\xff\x62\x48\x96\xba\x1c\x04\x00\x00")

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
