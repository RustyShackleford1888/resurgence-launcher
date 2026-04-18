// +build windows

package d2

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/windows/registry"
)

const (
	// ModMaphackIdentifier is the identifier we use to look for installs of maphack.
	ModMaphackIdentifier = "BH.dll"

	// ModHDIdentifier is the identifier we use to look for installs of hd mod.
	ModHDIdentifier = "D2HD.dll"

	// RegistryLayers is where all data about execution resides, like DEP.
	RegistryLayers = `Software\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers`

	// RegistryPermissions is the required permissions needed for operations on our keys.
	RegistryPermissions = registry.QUERY_VALUE | registry.SET_VALUE
)

// validate113cVersion will check the given installations Diablo II version.
func validate113cVersion(path string) (bool, error) {
	// Open local Game.exe.
	content, err := ioutil.ReadFile(localizePath(path) + "\\Game.exe")
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	// Hash the content of the Game.exe.
	hashed := fmt.Sprintf("%x", sha1.Sum(content))

	// Check the game version.
	version, ok := hashList[hashed]

	// Unknown game version.
	if !ok {
		return false, nil
	}

	return version == "1.13c", nil
}

// launch will execute the Diablo II.exe in the given directory.
func launch(path string, flags []string, done chan execState) (*int, error) {
	// Localize the path.
	localized := localizePath(path)

	// Exec the Diablo II.exe with the given command line args.
	cmd := exec.Command(localized+"\\Diablo II.exe", flags...)
	cmd.Dir = localized

	// Collect the output from the command.
	var stderr bytes.Buffer

	// Pipe errors to our buffer.
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Wait on separate thread.
	go func() {
		if err := cmd.Wait(); err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				// The program has exited unsuccessfully most probably they just 'X'ed the window, no need to log it.
				done <- execState{pid: &cmd.Process.Pid, err: nil}
			} else {
				// Was some other wait error such as permissions, return the err.
				done <- execState{pid: &cmd.Process.Pid, err: fmt.Errorf("cmd.Wait: %d : %s", err, stderr.String())}
			}
		}

		done <- execState{pid: &cmd.Process.Pid, err: nil}
	}()

	return &cmd.Process.Pid, nil
}

// configureForOS will set specific configurations, such as compatibility mode.
func configureForOS(path string) error {
	// The key name is the localized path for the Diablo II directory.
	keyName := fmt.Sprintf("%s\\%s", localizePath(path), "Game.exe")

	// Open the compatibility key directory.
	compatibilityKey, err := registry.OpenKey(registry.CURRENT_USER,
		RegistryLayers,
		RegistryPermissions,
	)
	if err != nil {
		return err
	}

	// Set Windows XP Service Pack 2 compatibility mode.
	if err := compatibilityKey.SetStringValue(keyName, "~ WINXPSP2"); err != nil {
		return err
	}

	// Close the registry when we're done.
	if err := compatibilityKey.Close(); err != nil {
		return err
	}

	return nil
}

// applyDEP will run a fix to disable DEP.
func applyDEP(path string) error {
	// The key name is the localized path for the Diablo II directory.
	keyName := fmt.Sprintf("%s\\%s", localizePath(path), "Diablo II.exe")

	// Open the dep key directory.
	depKey, err := registry.OpenKey(registry.CURRENT_USER,
		RegistryLayers,
		RegistryPermissions,
	)
	if err != nil {
		return err
	}

	// Set the value to disable DEP.
	if err := depKey.SetStringValue(keyName, "DisableNXShowUI"); err != nil {
		return err
	}

	// Close the registry when we're done.
	if err := depKey.Close(); err != nil {
		return err
	}

	return nil
}

func isModInstalled(path string, identifier string, manifest *Manifest) (bool, error) {
	filePath := localizePath(fmt.Sprintf("%s/%s", path, identifier))

	// Get the checksum from the file on disk.
	hashed, err := hashCRC32(filePath, polynomial)
	if err != nil {
		// The file doesn't exist on disk, so it's not installed.
		if err == ErrCRCFileNotFound {
			return false, nil
		}

		return false, err
	}

	var crc string
	// File exists on disk, find the CRC.
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

// localizePath will localize the path for the OS.
func localizePath(path string) string {
	// Windows uses backslashes for paths, so we'll reverse them.
	reversed := strings.Replace(path, "/", "\\", -1)

	// Remove the heading backslash.
	_, i := utf8.DecodeRuneInString(reversed)

	return reversed[i:]
}
