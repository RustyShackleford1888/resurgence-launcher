// +build linux

package d2

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// ModMaphackIdentifier is the identifier we use to look for installs of maphack.
	ModMaphackIdentifier = "BH.dll"

	// ModHDIdentifier is the identifier we use to look for installs of hd mod.
	ModHDIdentifier = "D2HD.dll"
)

// validate113cVersion will check the given installations Diablo II version.
func validate113cVersion(dir string) (bool, error) {
	content, err := ioutil.ReadFile(filepath.Join(localizePath(dir), "Game.exe"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	hashed := fmt.Sprintf("%x", sha1.Sum(content))
	version, ok := hashList[hashed]
	if !ok {
		return false, nil
	}

	return version == "1.13c", nil
}

// launch will execute the Diablo II.exe in the given directory.
func launch(path string, flags []string, done chan execState) (*int, error) {
	localized := localizePath(path)
	runner, runnerArgs := getLinuxRunner()
	if _, err := exec.LookPath(runner); err != nil {
		return nil, fmt.Errorf("unable to find %q in PATH; install Wine or set RESURGENCE_D2_RUNNER", runner)
	}

	args := append(runnerArgs, "Diablo II.exe")
	args = append(args, flags...)

	cmd := exec.Command(runner, args...)
	cmd.Dir = localized

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				done <- execState{pid: &cmd.Process.Pid, err: nil}
			} else {
				done <- execState{pid: &cmd.Process.Pid, err: fmt.Errorf("cmd.Wait: %v : %s", err, stderr.String())}
			}
			return
		}

		done <- execState{pid: &cmd.Process.Pid, err: nil}
	}()

	return &cmd.Process.Pid, nil
}

// localizePath will localize the path for the OS.
func localizePath(path string) string {
	return path
}

// configureForOS will set specific configurations, such as compatibility mode.
func configureForOS(path string) error {
	return nil
}

// applyDEP will run a fix to disable DEP.
func applyDEP(path string) error {
	return nil
}

func setDiabloRegistryKeys() error {
	return nil
}

func isModInstalled(path string, identifier string, manifest *Manifest) (bool, error) {
	filePath := filepath.Join(localizePath(path), identifier)

	hashed, err := hashCRC32(filePath, polynomial)
	if err != nil {
		if err == ErrCRCFileNotFound {
			return false, nil
		}

		return false, err
	}

	var crc string
	for _, f := range manifest.Files {
		if f.Name == identifier {
			crc = f.CRC
			break
		}
	}

	if crc == hashed {
		return true, nil
	}

	return false, nil
}

func getLinuxRunner() (string, []string) {
	runner := strings.TrimSpace(os.Getenv("RESURGENCE_D2_RUNNER"))
	if runner == "" {
		return "wine", nil
	}

	parts := strings.Fields(runner)
	if len(parts) == 0 {
		return "wine", nil
	}

	return parts[0], parts[1:]
}
