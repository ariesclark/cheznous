package chezmoi

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
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
	return slices.ContainsFunc(strings.Split(sourceRelPath.String(), "/"), isGroupDirName)
}

// groupSourceDir returns the source directory of the entry in a group that
// already manages targetRelPath, so that rewriting the entry keeps it in its
// group.
func (s *SourceState) groupSourceDir(targetRelPath RelPath) (SourceRelPath, bool) {
	sourceStateEntry := s.root.Get(targetRelPath)
	if sourceStateEntry == nil || sourceStateEntry.Origin().IsExternal() {
		return SourceRelPath{}, false
	}
	sourceRelPath := sourceStateEntry.SourceRelPath()
	if !isInGroupDir(sourceRelPath) {
		return SourceRelPath{}, false
	}
	return sourceRelPath.Dir(), true
}

// checkAddGroup returns an error unless group is the name of a group directory
// whose condition is true.
func (s *SourceState) checkAddGroup(group string) error {
	if !isGroupDirName(group) {
		return fmt.Errorf("%s: not a group, which is a template pipeline in parentheses", group)
	}
	switch err := s.readGroupDir(NewSourceRelDirPath(group), group); {
	case errors.Is(err, fs.SkipDir):
		return fmt.Errorf("%s: group condition is false on this machine", group)
	default:
		return err
	}
}

// groupedSourceDir returns the source directory in group for the target
// directory targetDirRelPath, which is its current source directory with any
// group directories taken out, and creates it in sourceSystem.
func (s *SourceState) groupedSourceDir(sourceSystem System, group string, targetDirRelPath RelPath) (SourceRelPath, error) {
	currentSourceDir, ok := s.targetDirSourceDir(targetDirRelPath)
	if !ok {
		sourceStateEntry := s.root.Get(targetDirRelPath)
		if sourceStateEntry == nil {
			return SourceRelPath{}, fmt.Errorf("%s: parent directory not in source state", targetDirRelPath)
		}
		currentSourceDir = sourceStateEntry.SourceRelPath()
	}
	var ungroupedSourceNames []string
	for sourceName := range strings.SplitSeq(currentSourceDir.String(), "/") {
		if sourceName != "" && !isGroupDirName(sourceName) {
			ungroupedSourceNames = append(ungroupedSourceNames, sourceName)
		}
	}
	sourceNames := make([]string, 0, 1+len(ungroupedSourceNames))
	if len(ungroupedSourceNames) > 0 && ungroupedSourceNames[0] == homeDirName {
		sourceNames = append(sourceNames, homeDirName)
		ungroupedSourceNames = ungroupedSourceNames[1:]
	}
	sourceNames = append(sourceNames, group)
	sourceNames = append(sourceNames, ungroupedSourceNames...)
	groupedSourceDir := NewSourceRelDirPath(path.Join(sourceNames...))
	if err := MkdirAll(sourceSystem, s.sourceDirAbsPath.Join(groupedSourceDir.RelPath()), fs.ModePerm); err != nil {
		return SourceRelPath{}, err
	}
	return groupedSourceDir, nil
}

// readGroupDir returns nil if the condition of the group directory at
// sourceRelPath is true, so that its entries are read, and [fs.SkipDir] if it
// is false.
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
