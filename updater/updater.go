package updater

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type githubRelease struct {
	TagName    string    `json:"tag_name"`
	CreatedAt  time.Time `json:"created_at"`
	ZipballURL string    `json:"zipball_url"`
}

func (release *githubRelease) download(zipFile *os.File) (*os.File, error) {
	defer zipFile.Close()

	zipFileResponse, err := http.Get(release.ZipballURL)
	if err != nil {
		return zipFile, err
	}
	defer zipFileResponse.Body.Close()

	_, err = io.Copy(zipFile, zipFileResponse.Body)
	if err != nil {
		return zipFile, err
	}
	return zipFile, nil
}

// GetLatestRelease downloads a latest release from projectName ("username/reponame") to destinationDir
func GetLatestRelease(projectName string, destinationDir string) []string {
	releaseURL := fmt.Sprintf("https://api.github.com/repos/%v/releases/latest", projectName)
	tmpFile, err := ioutil.TempFile(os.TempDir(), strings.Replace(projectName, string(os.PathSeparator), "_", -1))
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	var files []string
	log.Printf("saving release to %v", destinationDir)

	releaseAPIResponse, err := http.Get(releaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer releaseAPIResponse.Body.Close()

	var release *githubRelease
	decoder := json.NewDecoder(releaseAPIResponse.Body)
	decoder.UseNumber()
	err = decoder.Decode(&release)
	if err != nil {
		panic(err)
	}
	zipFile, err := release.download(tmpFile)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(zipFile.Name())

	files, err = Unzip(zipFile.Name(), destinationDir)
	if err != nil {
		log.Fatal(err)
	}
	if err != nil {
		log.Fatal(err)
	}
	return files
}

// Unzip `src` file into `dest` destination
func Unzip(src string, dest string) ([]string, error) {

	var filenames []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()

	baseName := ""
	for i, f := range r.File {
		// Trim archive file name from resulting path structure
		if i == 0 {
			baseName = f.Name
			continue
		}
		f.Name = strings.TrimPrefix(f.Name, baseName)

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}
		defer rc.Close()

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return filenames, fmt.Errorf("%s: illegal file path", fpath)
		}

		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)

		} else {

			// Make File
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return filenames, err
			}

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return filenames, err
			}

			_, err = io.Copy(outFile, rc)

			// Close the file without defer to close before next iteration of loop
			outFile.Close()

			if err != nil {
				return filenames, err
			}

		}
	}
	return filenames, nil
}
