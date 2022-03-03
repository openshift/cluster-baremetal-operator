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

var _Manifests0000_31_clusterBaremetalOperator_07_clusteroperatorCrYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x94\x91\x41\x6a\x03\x31\x0c\x45\xf7\x3e\x85\x98\xbd\xa7\xc9\xd6\xdb\x1e\xa0\xbb\xee\x95\xf1\xcf\x8c\xa8\x23\x19\x5b\x33\xb4\x94\xde\xbd\x24\x4d\x02\xa1\x50\xe8\xce\x7c\xfc\x9f\xc4\x13\x57\x79\x45\xeb\x62\x9a\x68\x32\x3d\xca\x3c\x5a\x85\xf6\x45\x8e\x3e\x8a\x3d\x6d\xfb\xf0\x26\x9a\x13\x3d\x97\xb5\x3b\xda\x4b\x45\x63\xb7\x16\x4e\x70\xce\xec\x9c\x02\x91\xf2\x09\x89\x0e\xdc\x70\x4e\x4b\x20\x62\x55\x73\x76\x31\xed\xe7\x0f\x44\x13\x57\x3e\x48\x11\xff\x78\xe4\xff\xae\x12\xe1\x7d\x2a\x6b\xc6\xd8\x50\xc0\x1d\x8f\x05\x51\x47\x53\x2e\xf1\x9e\xc6\xc5\xba\x23\x27\x1a\xbc\xad\x18\x2e\x08\xd1\x3f\x10\x1d\xe5\x18\x4f\xac\x3c\x23\xc7\x45\xe6\x25\xf2\xc6\x52\xae\xfb\xfd\x83\x23\x3a\x17\x44\xb5\x8c\x98\xb1\xa1\x58\x45\xbb\xd7\x7b\xc5\x94\xe8\xf3\x2b\x74\x67\x5f\x2f\x1a\xb6\x1f\xd3\x97\x77\xbc\x5a\xb3\x9b\xd0\xf3\xb8\xed\x76\x8a\x61\x37\xee\xc6\x7d\xec\xca\xb5\x2f\xe6\x43\xf8\x0e\x00\x00\xff\xff\x79\x9c\x81\x80\xa8\x01\x00\x00")

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
