package chezmoi

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// homeDirName is the name of the directory at the top of the source directory
// whose entries apply to the home directory. Every other entry applies to the
// destination directory, which must then contain the home directory.
const homeDirName = "~"

// A homeDirAction is what reading the source state does with an entry.
type homeDirAction int

const (
	homeDirActionRead homeDirAction = iota
	homeDirActionEnter
	homeDirActionSkip
)

// WithHomeDir sets the home directory.
func WithHomeDir(homeDirAbsPath AbsPath) SourceStateOption {
	return func(s *SourceState) {
		s.homeDirAbsPath = homeDirAbsPath
	}
}

// readHomeDir records whether the source directory has a ~ directory and, if
// it does, where the home directory is in the destination directory.
func (s *SourceState) readHomeDir() error {
	hasHomeDir, err := HasHomeDir(s.system, s.sourceDirAbsPath)
	if err != nil || !hasHomeDir {
		s.hasHomeDir = false
		return err
	}
	s.hasHomeDir = true
	homeRelPath, err := HomeRelPath(s.destDirAbsPath, s.homeDirAbsPath)
	if err != nil {
		return err
	}
	s.homeRelPath = homeRelPath
	return nil
}

// HasHomeDir returns whether the source directory sourceDirAbsPath has a ~
// directory.
func HasHomeDir(system System, sourceDirAbsPath AbsPath) (bool, error) {
	switch fileInfo, err := system.Stat(sourceDirAbsPath.JoinString(homeDirName)); {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	default:
		return fileInfo.IsDir(), nil
	}
}

// HomeRelPath returns the path of homeDirAbsPath in destDirAbsPath, which is
// empty when they are the same.
func HomeRelPath(destDirAbsPath, homeDirAbsPath AbsPath) (RelPath, error) {
	if destDirAbsPath == homeDirAbsPath {
		return EmptyRelPath, nil
	}
	homeRelPath, err := homeDirAbsPath.TrimDirPrefix(destDirAbsPath)
	if err != nil {
		return EmptyRelPath, fmt.Errorf("%s: the destination directory must be the home directory %s or contain it, because the source directory has a ~ directory", destDirAbsPath, homeDirAbsPath)
	}
	return homeRelPath, nil
}

// appliesOutsideHomeDir returns whether s applies the entries outside the ~
// directory, which it does when the destination directory contains the home
// directory.
func (s *SourceState) appliesOutsideHomeDir() bool {
	return !s.homeRelPath.IsEmpty()
}

// homeDirActionOf returns what reading s does with the entry at sourceRelPath:
// read it, only enter it for the ~ directory itself, or skip an entry outside
// the ~ directory when the destination is the home directory.
func (s *SourceState) homeDirActionOf(sourceRelPath SourceRelPath) (homeDirAction, error) {
	sourceNames := strings.Split(sourceRelPath.String(), "/")
	last := len(sourceNames) - 1
	for i, sourceName := range sourceNames {
		isDir := i < last || sourceRelPath.isDir
		switch {
		case isDir && isGroupDirName(sourceName):
			continue
		case isDir && sourceName == homeDirName:
			switch {
			case i > 0:
				return 0, fmt.Errorf("%s: the ~ directory must be at the top of the source directory", path.Join(sourceNames[:i+1]...))
			case i == last:
				return homeDirActionEnter, nil
			default:
				return homeDirActionRead, nil
			}
		case !s.hasHomeDir, s.appliesOutsideHomeDir(), i == last && isSharedName(sourceName):
			return homeDirActionRead, nil
		default:
			return homeDirActionSkip, nil
		}
	}
	return homeDirActionRead, nil
}

// isSharedName returns whether name is the name of an entry outside the ~
// directory that is read even when the destination is the home directory.
func isSharedName(name string) bool {
	return name == dataName || isPrefixDotFormat(name, dataName) || name == TemplatesDirName || name == VersionName
}

// isTargetRootHomeDir returns whether sourceNames[i] is a ~ directory with only
// group directories above it.
func isTargetRootHomeDir(sourceNames []string, i int) bool {
	if sourceNames[i] != homeDirName {
		return false
	}
	for _, sourceName := range sourceNames[:i] {
		if !isGroupDirName(sourceName) {
			return false
		}
	}
	return true
}

// isInHomeDir returns whether sourceRelPath is in the ~ directory.
func isInHomeDir(sourceRelPath SourceRelPath) bool {
	sourceNames := strings.Split(sourceRelPath.String(), "/")
	for i := range sourceNames {
		if isTargetRootHomeDir(sourceNames, i) {
			return true
		}
	}
	return false
}

// TargetRelPathIn returns the relative path of the target of sourceRelPath in
// a destination directory that has the home directory at homeRelPath.
func TargetRelPathIn(sourceRelPath SourceRelPath, encryptedSuffix string, homeRelPath RelPath) (RelPath, error) {
	targetRelPath, err := sourceRelPath.TargetRelPath(encryptedSuffix)
	if err != nil || homeRelPath.IsEmpty() || !isInHomeDir(sourceRelPath) {
		return targetRelPath, err
	}
	if targetRelPath.IsEmpty() {
		return homeRelPath, nil
	}
	return homeRelPath.Join(targetRelPath), nil
}

// targetRelPath returns the relative path of the target of sourceRelPath.
func (s *SourceState) targetRelPath(sourceRelPath SourceRelPath, encryptedSuffix string) (RelPath, error) {
	return TargetRelPathIn(sourceRelPath, encryptedSuffix, s.homeRelPath)
}

// homeTargetRelPath returns the relative path of the home directory in the
// destination directory, which is DotRelPath when they are the same.
func (s *SourceState) homeTargetRelPath() RelPath {
	if s.homeRelPath.IsEmpty() {
		return DotRelPath
	}
	return s.homeRelPath
}

// mkdirAll is s.root.MkdirAll for the parent directory of an external, except
// that it leaves the home directory and the directories above it unmanaged.
func (s *SourceState) mkdirAll(targetRelPath RelPath, origin SourceStateOrigin) (*SourceStateDir, error) {
	if s.homeRelPath.IsEmpty() || targetRelPath != s.homeRelPath && !targetRelPath.HasDirPrefix(s.homeRelPath) {
		return s.root.MkdirAll(targetRelPath, origin, s.umask)
	}
	node := &s.root
	for _, componentRelPath := range s.homeRelPath.SplitAll() {
		if node.Children == nil {
			node.Children = make(map[RelPath]*SourceStateEntryTreeNode)
		}
		child, ok := node.Children[componentRelPath]
		if !ok {
			child = NewSourceStateEntryTreeNode()
			node.Children[componentRelPath] = child
		}
		node = child
	}
	if targetRelPath == s.homeRelPath {
		return nil, nil
	}
	relPath, err := targetRelPath.TrimDirPrefix(s.homeRelPath)
	if err != nil {
		return nil, err
	}
	return node.MkdirAll(relPath, origin, s.umask)
}

// targetDirSourceDir returns the source directory of the target directory
// targetDirRelPath when it is the home directory or the top of the destination
// directory, which have no entries of their own.
func (s *SourceState) targetDirSourceDir(targetDirRelPath RelPath) (SourceRelPath, bool) {
	switch {
	case s.hasHomeDir && targetDirRelPath == s.homeTargetRelPath():
		return NewSourceRelDirPath(homeDirName), true
	case targetDirRelPath == DotRelPath:
		return SourceRelPath{}, true
	default:
		return SourceRelPath{}, false
	}
}
