/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

type Images struct {
	BaremetalOperator            string `json:"baremetalOperator"`
	BaremetalIronic              string `json:"baremetalIronic"`
	BaremetalIronicInspector     string `json:"baremetalIronicInspector"`
	BaremetalIpaDownloader       string `json:"baremetalIpaDownloader"`
	BaremetalMachineOsDownloader string `json:"baremetalMachineOsDownloader"`
	BaremetalStaticIpManager     string `json:"baremetalStaticIpManager"`
}

func GetContainerImages(containerImages *Images, imagesFilePath string) error {
	//read images.json file
	jsonData, err := ioutil.ReadFile(filepath.Clean(imagesFilePath))
	if err != nil {
		// TODO: Not sure if this error msg should show the path and file name of the mounted file.
		return fmt.Errorf("unable to read file %s", imagesFilePath)
	}
	if err := json.Unmarshal(jsonData, containerImages); err != nil {
		return fmt.Errorf("unable to unmarshal image names from file %s ", imagesFilePath)
	}
	return nil
}
