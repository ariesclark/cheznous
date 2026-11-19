package chezmoi

import (
	"fmt"
	"io/fs"
	"strings"
)

// groupDirOrder visits group directories after every other entry in their
// directory, so that their conditions see the data and ignore patterns there.
const groupDirOrder = 1

// isGroupDirName returns whether name is the name of a group directory, a Go
// template pipeline in parentheses, whose entries apply to the parent target
// directory when the pipeline is true.
func isGroupDirName(name string) bool {
	return len(name) > len("()") && strings.HasPrefix(name, "(") && strings.HasSuffix(name, ")")
}

// sourceDirEntryOrderOf returns the order in which dirEntry is visited among
// its siblings.
func sourceDirEntryOrderOf(dirEntry fs.DirEntry) int {
	if isGroupDirName(dirEntry.Name()) {
		return groupDirOrder
	}
	return sourceDirEntryOrder[dirEntry.Name()]
}

// isInGroupDir returns whether sourceRelPath is inside a group directory.
func isInGroupDir(sourceRelPath SourceRelPath) bool {
	for _, sourceName := range strings.Split(sourceRelPath.String(), "/") {
		if isGroupDirName(sourceName) {
			return true
		}
	}
	return false
}

// readGroupDir returns nil if the condition of the group directory at
// sourceRelPath is true, so that its entries are read, and fs.SkipDir if it is
// false.
func (s *SourceState) readGroupDir(sourceRelPath SourceRelPath, name string) error {
	result, err := s.ExecuteTemplateData(ExecuteTemplateDataOptions{
		NameRelPath: sourceRelPath.RelPath(),
		Data:        []byte("{{ " + name + " }}"),
	})
	if err != nil {
		return fmt.Errorf("%s: %w", sourceRelPath, err)
	}
	switch string(result) {
	case "true":
		return nil
	case "false":
		return fs.SkipDir
	default:
		return fmt.Errorf("%s: group condition must be true or false, got %q", sourceRelPath, result)
	}
}
