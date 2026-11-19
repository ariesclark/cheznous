package cmd

import (
	"path/filepath"
	"strings"
)

// commandNameFromExecutable returns the name the user typed to run executable,
// so that help and completions match it whether it is cheznous or chezmoi.
func commandNameFromExecutable(executable string) string {
	name := filepath.Base(executable)
	if extension := filepath.Ext(name); strings.EqualFold(extension, ".exe") {
		name = strings.TrimSuffix(name, extension)
	}
	return name
}

func withCommandName(commandName string) configOption {
	return func(c *Config) error {
		c.commandName = commandName
		return nil
	}
}
