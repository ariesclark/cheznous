package chezmoi

import "os/exec"

// NewHomeOwnerSystem returns realSystem, because Windows has no root.
func NewHomeOwnerSystem(realSystem *RealSystem, destDirAbsPath, homeDirAbsPath AbsPath, hasHomeDirFunc func() (bool, error)) (System, error) {
	return realSystem, nil
}

func (o *homeOwner) setCredential(cmd *exec.Cmd) {}
