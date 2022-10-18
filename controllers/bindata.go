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

var _Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x93\xc1\x6e\xd4\x30\x10\x86\xef\x79\x0a\x2b\x77\x87\x56\xdc\x72\xe5\x01\x7a\xe3\x3e\x71\xfe\x4d\x06\x9c\xb1\xe5\x99\x04\x10\xe2\xdd\x51\xdc\x34\xed\x0a\x81\xd8\x4a\x7b\x5b\x79\x67\xbe\xff\xd7\x37\x0a\x65\xfe\x8c\xa2\x9c\xa4\x77\x21\xc9\x85\xa7\x2e\x65\x88\xce\x7c\xb1\x8e\xd3\x87\xed\xb1\xf9\xca\x32\xf6\xee\x53\x5c\xd5\x50\x9e\x32\x0a\x59\x2a\xcd\x02\xa3\x91\x8c\xfa\xc6\x39\xa1\x05\xbd\x1b\xa8\x60\x7f\x8d\x8d\x73\x24\x92\x8c\x8c\x93\xe8\x3e\xe0\x5c\xa0\x4c\x03\x47\xb6\x1f\xd7\xfc\x3f\x57\x9d\xc3\xf7\x10\xd7\x11\x5d\x41\x04\x29\xae\x17\x58\x0c\x45\x28\xfa\xf3\xd5\xcf\x49\x0d\x63\xef\x5a\x2b\x2b\xda\x8a\x60\xf9\x07\x42\x11\x2f\x7e\x21\xa1\x09\xa3\x9f\x79\x9a\x3d\x6d\xc4\xf1\xe8\x77\x03\x87\x65\x8a\xf0\x92\x46\xf8\x11\x1b\x62\xca\x28\xe7\xba\x66\x84\xde\xfd\xfc\xd5\xa8\x91\xad\x55\xc3\xf6\x6c\xba\xfe\xf6\x87\xb5\xf4\x22\x74\x8f\xdb\x5e\x4e\xd1\x3e\x74\x0f\xdd\xa3\x57\xa1\xac\x73\xb2\xbd\x4c\x41\x24\xc3\xf8\x34\x7c\x41\xb0\x03\x31\x95\xb4\xe6\xde\xb5\xcf\x65\x4f\xe0\xe1\x65\xa1\x30\xb3\xc0\x53\xe6\xfa\x7f\x81\xa6\xb5\x04\xf4\x75\x52\x33\x05\xe8\x15\xa6\x9e\xe0\x63\xc7\xe9\x2d\xef\x80\xbf\x2e\xb7\xe7\xb5\x76\xf1\xfa\x3a\x5b\x89\x7f\x2f\x70\x73\x4e\x2e\x69\xe3\xdd\x07\xcb\xd4\xbe\x07\xb0\xf7\xbb\x70\x59\xbe\x51\x81\xc2\x8c\x65\xba\x63\xdd\x33\x29\xcc\x58\xe8\xae\x5e\xf0\x56\x0d\x2f\x34\xe1\x8e\x71\xc3\x12\xb0\x41\x4c\xd7\x41\x43\xe1\x5c\xbf\xea\xff\x8c\xfb\x1d\x00\x00\xff\xff\xa7\x55\x92\x38\x5f\x04\x00\x00")

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
