//go:build !windows

package chezmoi

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"sync"
	"syscall"

	"chezmoi.io/chezmoi/v2/internal/chezmoierrors"
)

// A homeOwner is the user who owns the home directory. When root applies a
// destination directory that contains the home directory, a HomeOwnerSystem
// gives them what it writes there and runs the scripts there as them.
type homeOwner struct {
	uid    int
	gid    int
	groups []uint32
	env    []string
}

// A HomeOwnerSystem is the RealSystem of root applying a destination
// directory that contains the home directory of another user, for a source
// directory with a ~ directory. It gives that user what it writes in their home
// directory and runs the scripts there as them, so that root only writes
// outside it.
type HomeOwnerSystem struct {
	*RealSystem

	homeDirAbsPath AbsPath
	owner          homeOwner
	rootEnv        []string
	hasHomeDirFunc func() (bool, error)
	hasHomeDirOnce sync.Once
	hasHomeDir     bool
	hasHomeDirErr  error
}

// NewHomeOwnerSystem returns a HomeOwnerSystem when root applies
// destDirAbsPath and it contains homeDirAbsPath, which another user owns, and
// realSystem otherwise. hasHomeDirFunc returns whether the source directory
// has a ~ directory, and is called on the first write.
func NewHomeOwnerSystem(
	realSystem *RealSystem,
	destDirAbsPath, homeDirAbsPath AbsPath,
	hasHomeDirFunc func() (bool, error),
) (System, error) {
	if os.Geteuid() != 0 || destDirAbsPath == homeDirAbsPath || !homeDirAbsPath.HasDirPrefix(destDirAbsPath) {
		return realSystem, nil
	}
	fileInfo, err := os.Stat(homeDirAbsPath.String())
	if err != nil {
		return nil, err
	}
	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid == 0 {
		return realSystem, nil
	}
	owner, err := lookupHomeOwner(stat.Uid, homeDirAbsPath)
	if err != nil {
		return nil, err
	}
	root, err := user.Current()
	if err != nil {
		return nil, err
	}
	return &HomeOwnerSystem{
		RealSystem:     realSystem,
		homeDirAbsPath: homeDirAbsPath,
		owner:          owner,
		rootEnv:        []string{"HOME=" + root.HomeDir, "USER=" + root.Username, "LOGNAME=" + root.Username},
		hasHomeDirFunc: hasHomeDirFunc,
	}, nil
}

// lookupHomeOwner returns the user with uid, whose home directory is
// homeDirAbsPath.
func lookupHomeOwner(uid uint32, homeDirAbsPath AbsPath) (homeOwner, error) {
	ownerUser, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return homeOwner{}, err
	}
	gid, err := strconv.Atoi(ownerUser.Gid)
	if err != nil {
		return homeOwner{}, err
	}
	groupIDs, err := ownerUser.GroupIds()
	if err != nil {
		return homeOwner{}, err
	}
	groups := make([]uint32, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		group, err := strconv.ParseUint(groupID, 10, 32)
		if err != nil {
			return homeOwner{}, fmt.Errorf("%s: %w", groupID, err)
		}
		groups = append(groups, uint32(group))
	}
	return homeOwner{
		uid:    int(uid),
		gid:    gid,
		groups: groups,
		env:    []string{"HOME=" + homeDirAbsPath.String(), "USER=" + ownerUser.Username, "LOGNAME=" + ownerUser.Username},
	}, nil
}

func (o *homeOwner) chownFile(f *os.File) error {
	return f.Chown(o.uid, o.gid)
}

func (o *homeOwner) setCredential(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:    uint32(o.uid), //nolint:gosec
			Gid:    uint32(o.gid), //nolint:gosec
			Groups: o.groups,
		},
	}
}

// Mkdir implements System.Mkdir.
func (s *HomeOwnerSystem) Mkdir(name AbsPath, perm fs.FileMode) error {
	if err := s.RealSystem.Mkdir(name, perm); err != nil {
		return err
	}
	return s.chown(name)
}

// RunScript implements System.RunScript.
func (s *HomeOwnerSystem) RunScript(scriptName RelPath, dir AbsPath, data []byte, options RunScriptOptions) (err error) {
	args := runScriptArgs{
		scriptName:    scriptName.Base(),
		workingDir:    dir,
		setWorkingDir: true,
		data:          data,
		interpreter:   options.Interpreter,
		sourceRelPath: options.SourceRelPath,
		env:           s.rootEnv,
	}
	state := runScriptState{
		createScriptTempDirOnce: &s.createScriptTempDirOnce,
		scriptTempDir:           s.scriptTempDir,
		system:                  s,
	}
	switch inHomeDir, err := s.isInHomeDir(dir); {
	case err != nil:
		return err
	case inHomeDir:
		args.owner = &s.owner
		args.env = s.owner.env
		state = runScriptState{
			createScriptTempDirOnce: &sync.Once{},
			system:                  s,
		}
	}

	preparedScript, err := prepareScriptCmd(args, state)
	if err != nil {
		return err
	}
	defer chezmoierrors.CombineFunc(&err, preparedScript.cleanup)

	preparedScript.cmd.Stdin = os.Stdin
	preparedScript.cmd.Stdout = os.Stdout
	preparedScript.cmd.Stderr = os.Stderr

	return s.RunCmd(preparedScript.cmd)
}

// WriteFile implements System.WriteFile.
func (s *HomeOwnerSystem) WriteFile(filename AbsPath, data []byte, perm fs.FileMode) error {
	if err := s.RealSystem.WriteFile(filename, data, perm); err != nil {
		return err
	}
	return s.chown(filename)
}

// WriteSymlink implements System.WriteSymlink.
func (s *HomeOwnerSystem) WriteSymlink(oldName string, newName AbsPath) error {
	if err := s.RealSystem.WriteSymlink(oldName, newName); err != nil {
		return err
	}
	return s.chown(newName)
}

// chown gives the home directory's owner name, if it is in the home directory.
func (s *HomeOwnerSystem) chown(name AbsPath) error {
	switch inHomeDir, err := s.isInHomeDir(name); {
	case err != nil:
		return err
	case !inHomeDir || name == s.homeDirAbsPath:
		return nil
	default:
		return os.Lchown(name.String(), s.owner.uid, s.owner.gid)
	}
}

// isInHomeDir returns whether absPath is the home directory or in it, for a
// source directory with a ~ directory.
func (s *HomeOwnerSystem) isInHomeDir(absPath AbsPath) (bool, error) {
	s.hasHomeDirOnce.Do(func() {
		s.hasHomeDir, s.hasHomeDirErr = s.hasHomeDirFunc()
	})
	if s.hasHomeDirErr != nil || !s.hasHomeDir {
		return false, s.hasHomeDirErr
	}
	return absPath.HasDirPrefix(s.homeDirAbsPath), nil
}
