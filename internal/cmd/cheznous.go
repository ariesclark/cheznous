package cmd

import (
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"

	"chezmoi.io/chezmoi/v2/internal/chezmoi"
)

const (
	releaseRepositoryOwner = "ariesclark"
	releaseRepositoryName  = "cheznous"
)

// releaseCommandNames are the names of the executables in each release, which
// are the same program.
var releaseCommandNames = []string{"cheznous", "chezmoi"}

// commandNameFromExecutable returns the name the user typed to run executable,
// so that help and completions match it whether it is cheznous or chezmoi.
func commandNameFromExecutable(executable string) string {
	name := filepath.Base(executable)
	if extension := filepath.Ext(name); strings.EqualFold(extension, ".exe") {
		name = strings.TrimSuffix(name, extension)
	}
	return name
}

// replaceExecutables writes executableData to executableAbsPath, and to the
// executables under the other release command names in the same directory, so
// that cheznous and chezmoi stay at the same version.
func (c *Config) replaceExecutables(executableAbsPath chezmoi.AbsPath, executableData []byte) error {
	commandName := commandNameFromExecutable(executableAbsPath.Base())
	extension := strings.TrimPrefix(executableAbsPath.Base(), commandName)
	executableAbsPaths := []chezmoi.AbsPath{executableAbsPath}
	for _, otherCommandName := range releaseCommandNames {
		if strings.EqualFold(otherCommandName, commandName) {
			continue
		}
		otherExecutableAbsPath := executableAbsPath.Dir().JoinString(otherCommandName + extension)
		switch _, err := c.baseSystem.Lstat(otherExecutableAbsPath); {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return err
		default:
			executableAbsPaths = append(executableAbsPaths, otherExecutableAbsPath)
		}
	}

	for _, absPath := range executableAbsPaths {
		if runtime.GOOS == "windows" {
			if err := c.baseSystem.Rename(absPath, absPath.Append(".old")); err != nil {
				return err
			}
		}
		if err := c.baseSystem.WriteFile(absPath, executableData, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func withCommandName(commandName string) configOption {
	return func(c *Config) error {
		c.commandName = commandName
		return nil
	}
}
