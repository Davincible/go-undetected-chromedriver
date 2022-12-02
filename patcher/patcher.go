// Package patcher provides a patcher for the chromedriver.
package patcher

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repoURL = "https://chromedriver.storage.googleapis.com"
	// zipName with driver version. e.g. 'chromedriver_108.0.5359.71.zip'.
	zipName = "chromedriver_%s.zip"
	// binaryName of the patched chromedriver 'chromedriver_108-0-5359-71'.
	binaryName = "undetected_chromedriver_%s"
)

const (
	linux   platform = "linux64"
	darwin  platform = "mac64"
	windows platform = "win32"
)

var (
	re      = regexp.MustCompile("cdc_.{22}")
	letters = []byte("abcdefghijklmnopqrstuvwxyz")

	// RequestTimeout is the HTTP request timeout.
	RequestTimeout = 15 * time.Second
)

type platform string

// Patcher provides methods to patch a chromedriver.
type Patcher struct {
	client       *http.Client
	binaryPath   string
	zipName      string
	platform     platform
	dataDir      string
	version      string
	majorVersion int
}

// New returns a new patcher instance.
//
// binaryPath is an optional path to store write the patche binary to.
// version is the major chrome version driver to download and patch, e.g. '107'.
func New(binaryPath string, version int) (Patcher, error) {
	var p Patcher

	p.client = &http.Client{Timeout: RequestTimeout}

	if err := p.fetchLatestRelease(version); err != nil {
		return p, err
	}

	if err := p.setPath(binaryPath); err != nil {
		return p, err
	}

	return p, nil
}

// Patch will download and patch the latest chromedriver for the specified
// major version.
// Returns patched binary path.
func (p *Patcher) Patch() (string, error) {
	driverPath, err := p.downloadDriver(p.version)
	if err != nil {
		return "", err
	}

	driver, err := p.unzip(driverPath)
	if err != nil {
		return "", err
	}

	driver = patchDriver(driver)

	if _, err := os.Stat(p.binaryPath); err == nil {
		if err := os.Remove(p.binaryPath); err != nil {
			return "", fmt.Errorf("failed to remove old driver '%s': %w", p.binaryPath, err)
		}
	}

	if err := os.WriteFile(p.binaryPath, driver, 0755); err != nil { //nolint:gosec
		return "", err
	}

	return p.binaryPath, nil
}

// setPath sets all the required file paths.
func (p *Patcher) setPath(binaryPath string) error {
	if len(binaryPath) > 0 {
		if _, err := os.Stat(binaryPath); err != nil {
			return fmt.Errorf("error with provided binary path: %w", err)
		}

		p.binaryPath = binaryPath

		return nil
	}

	var binary string

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "linux":
		p.platform = linux
		binary = fmt.Sprintf(binaryName, p.version)
		p.dataDir = path.Join(home, ".local/share/undetected_chromedriver")
	case "darwin":
		p.platform = windows
		binary = fmt.Sprintf(binaryName, p.version)
		p.dataDir = path.Join(home, "appdata/roaming/undetected_chromedriver")
	case "Windows":
		p.platform = darwin
		binary = fmt.Sprintf(binaryName, p.version+".exe")
		p.dataDir = path.Join(home, "Library/Application Support/undetected_chromedriver")
	default:
		return fmt.Errorf("OS not supported: %s", runtime.GOOS)
	}

	if _, err := os.Stat(p.dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(p.dataDir, 0750); err != nil {
			return fmt.Errorf("failed to create temp dir '%s': %w", p.dataDir, err)
		}
	}

	p.binaryPath = path.Join(p.dataDir, binary)
	p.zipName = fmt.Sprintf(zipName, p.version)

	return nil
}

// fetchLatestRelease gets the latest full release number e.g. '108.0.5359.71'.
//
// Version param is the major version number to check. If 0 latest version will
// be used.
func (p *Patcher) fetchLatestRelease(version int) error {
	path := "/LATEST_RELEASE"

	if version > 0 {
		path += "_" + strconv.Itoa(version)
	}

	b, err := p.makeRequest(http.MethodGet, repoURL+path, nil)
	if err != nil {
		return fmt.Errorf("fetch latest release: %w", err)
	}

	p.majorVersion = version
	p.version = string(b)

	return nil
}

// downloadDriver downloads a specific version e.g. '108.0.5359.71'.
func (p *Patcher) downloadDriver(version string) (string, error) {
	if len(p.zipName) == 0 {
		return "", errors.New("zipname not set")
	}

	// Return if zip file already exists.
	zipPath := path.Join(os.TempDir(), p.zipName)
	if _, err := os.Stat(zipPath); err == nil {
		return zipPath, nil
	}

	url := fmt.Sprintf("%s/%s/chromedriver_%s.zip", repoURL, version, p.platform)

	b, err := p.makeRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("download driver (version: '%s'): %w", version, err)
	}

	if err := os.WriteFile(zipPath, b, 0644); err != nil { //nolint:gosec
		return "", fmt.Errorf("write zip to disk: %w", err)
	}

	return zipPath, nil
}

// unzip will unzip the chromedriver archive and directly return the driver
// contents.
func (p *Patcher) unzip(file string) ([]byte, error) {
	archive, err := zip.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("unzip '%s': %w", file, err)
	}
	defer archive.Close() //nolint:errcheck

	driver, err := archive.Open("chromedriver")
	if err != nil {
		return nil, fmt.Errorf("extract chromedriver from zip: %w", err)
	}
	defer driver.Close() //nolint:errcheck

	driverData, err := io.ReadAll(driver)
	if err != nil {
		return nil, fmt.Errorf("read chromedriver file: %w", err)
	}

	return driverData, nil
}

func (p *Patcher) makeRequest(method, url string, body io.Reader) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func patchDriver(driver []byte) []byte {
	if !re.Match(driver) {
		return nil
	}

	return re.ReplaceAll(driver, randomCDC())
}

func randomCDC() []byte {
	cdc := make([]byte, 26)

	if _, err := rand.Read(cdc); err != nil {
		// Shouldn't happen, but just in case.
		return []byte("xvx_plxklvnobnowmrmiIMvqlb")
	}

	for i, val := range cdc {
		cdc[i] = letters[int(val)%len(letters)]
	}

	cdc[2] = cdc[0]
	cdc[3] = '_'
	cdc[20] = strings.ToUpper(string(cdc[20]))[0]
	cdc[21] = strings.ToUpper(string(cdc[21]))[0]

	return cdc
}
