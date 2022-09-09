package pullutils

import (
	"encoding/base64"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/schollz/progressbar/v3"
)

// ImageInfo is a struct that holds the informations
// of the image we want to pull.
type ImageInfo struct {
	Image       string
	RegistryURL string
	ManifestURL string
	ImageBase   string
	ImageName   string
	ImageTag    string
}

// StarterJSON is a template of an empty json config for a layer.
var StarterJSON string = `
{
	%s
	"created": "0001-01-01T00:00:00Z",
	"container_config": {
		"Hostname": "",
		"Domainname": "",
		"User": "",
		"AttachStdin": false,
		"AttachStdout": false,
		"AttachStderr": false,
		"Tty": false,
		"OpenStdin": false,
		"StdinOnce": false,
		"Env": null,
		"Cmd": null,
		"Image": "",
		"Volumes": null,
		"WorkingDir": "",
		"Entrypoint": null,
		"OnBuild": null,
		"Labels": null
	}
}
`

// ManifestJSON is a template of an empty json manifest for an image.
var ManifestJSON string = `
[
  {
	"Config": "%s",
	"RepoTags": [
	  "%s"
	],
	"Layers": [
%s
	]
  }
]
`

// ImageDir is the default location for downloaded images.
var ImageDir string = os.Getenv("HOME") + `/.local/share/scatman/images/`

// GetImageDir returns a base64 encoded string of the full name of the image:
// registry.com/basename/name:tag.
func GetImageDir(pullImage ImageInfo) string {
	//imageFullName := pullImage.RegistryURL + "/" +
	//	pullImage.ImageBase + "/" + pullImage.ImageName + ":" + pullImage.ImageTag

	imageFullName := pullImage.Image + ":" + pullImage.ImageTag
	targetDIR := ImageDir
	targetDIR += base64.StdEncoding.EncodeToString([]byte(imageFullName))
	targetDIR += "/"

	return targetDIR
}

// GetImagePrefixName will return the imagebase, imagename and tag
// given a string like docker.io/almalinux/almalinux-8:latest.
func GetImagePrefixName(image string) (string, string, string) {
	imageName := ""

	tag := ""

	// in case we do not have a prefix, on docker.io we add library
	imageBase := strings.Split(image, "/")[1:]
	if len(imageBase) == 1 && strings.Contains(image, "docker.io") {
		imageBase = append([]string{"library"}, imageBase...)
	}

	imageNameTag := strings.Split(imageBase[len(imageBase)-1], ":")
	imageName = imageNameTag[0]

	if len(imageNameTag) == 1 {
		tag = "latest"
	} else {
		tag = imageNameTag[1]
	}

	return strings.Join(imageBase[:len(imageBase)-1], "/"), imageName, tag
}

// GetManifestURL will strt fom a string like docker.io/almalinux/almalinux-8:latest
// and return an registry manifest API url like:
// https://{{registry url}}/v2/{{ name }}/{{ subname }}/{{ tag/digest }}
// and return it.
func GetManifestURL(image string) (string, string) {
	manifestURL := ""

	registryBase := strings.Split(image, "/")[0]
	if strings.Contains(registryBase, "docker") {
		registryBase = "registry-1.docker.io"
	}

	imageBase, imageName, tag := GetImagePrefixName(image)
	manifestURL = "https://" + registryBase +
		"/v2/" + imageBase + "/" +
		imageName + "/manifests/" +
		tag

	return registryBase, manifestURL
}

// GetDockerPullAuthToken will return a valit auth token from auth.docker.io when needed.
func GetDockerPullAuthToken(image string) (string, error) {
	url := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:" + image + ":pull"

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	file, err := ioutil.TempFile(os.TempDir(), "token")
	if err != nil {
		return "", err
	}
	// remember delete it
	defer os.Remove(file.Name())

	// Write the body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	inputFile, err := os.ReadFile(file.Name())
	if err != nil {
		return "", err
	}

	jsonParsed, err := gabs.ParseJSON(inputFile)
	if err != nil {
		return "", err
	}

	return jsonParsed.Path("token").Data().(string), nil
}

// DownloadBlobToFile will download the blob in the url given, and save
// it to gived filename.
func DownloadBlobToFile(url string, token string, filename string, progress bool) error {
	client := http.Client{}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	if token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	}

	// Get the data
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	downloadFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer downloadFile.Close()

	// Write the body to file
	if progress {
		bar := progressbar.NewOptions64(resp.ContentLength,
			// progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(30),
			progressbar.OptionSetDescription("Copying blob "+filepath.Base(filename)),
			progressbar.OptionOnCompletion(func() {
				println("")
				log.Printf("Copying blob %s done\n", filepath.Base(filename))
			}),
		)
		_, err = io.Copy(io.MultiWriter(downloadFile, bar), resp.Body)

		return err
	}

	_, err = io.Copy(downloadFile, resp.Body)

	return err
}

// DownloadManifest will download the json manifest in the url given, and save
// it to gived filename.
func DownloadManifest(url string, filename *os.File, token string) error {
	client := http.Client{}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header = http.Header{
		"Accept": {"application/vnd.docker.distribution.manifest.v2+json"},
	}

	if token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	}

	// Get the data
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(filename, resp.Body)

	return err
}
