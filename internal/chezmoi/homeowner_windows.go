package chezmoi

import (
	"os"
	"os/exec"
)

// A homeOwner is never set on Windows, which has no root.
type homeOwner struct{}

// NewHomeOwnerSystem returns realSystem, because Windows has no root.
func NewHomeOwnerSystem(
	realSystem *RealSystem,
	destDirAbsPath, homeDirAbsPath AbsPath,
	hasHomeDirFunc func() (bool, error),
) (System, error) {
	return realSystem, nil
}

func (o *homeOwner) chownFile(f *os.File) error { return nil }

func (o *homeOwner) setCredential(cmd *exec.Cmd) {}
