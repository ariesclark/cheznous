//go:build !windows

package chezmoi

import (
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/alecthomas/assert/v2"
	vfs "github.com/twpayne/go-vfs/v5"
)

func TestHomeOwnerSystem(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	nobody := 65534

	destDir := t.TempDir()
	t.Setenv("HOME", destDir+"/home/nobody")
	assert.NoError(t, os.Chmod(filepath.Dir(destDir), 0o755))
	assert.NoError(t, os.Chmod(destDir, 0o755))
	homeDir := destDir + "/home/nobody"
	assert.NoError(t, os.MkdirAll(homeDir, 0o755))
	assert.NoError(t, os.Chown(homeDir, nobody, nobody))
	assert.NoError(t, os.MkdirAll(destDir+"/etc", 0o755))

	system, err := NewHomeOwnerSystem(
		NewRealSystem(vfs.OSFS),
		NewAbsPath(destDir),
		NewAbsPath(homeDir),
		func() (bool, error) { return true, nil },
	)
	assert.NoError(t, err)
	_, ok := system.(*HomeOwnerSystem)
	assert.True(t, ok)

	owner := func(name string) int {
		t.Helper()
		info, err := os.Lstat(name)
		assert.NoError(t, err)
		return int(info.Sys().(*syscall.Stat_t).Uid) //nolint:forcetypeassert
	}

	assert.NoError(t, system.Mkdir(NewAbsPath(homeDir+"/.config"), fs.ModePerm))
	assert.NoError(t, system.WriteFile(NewAbsPath(homeDir+"/.config/file"), []byte("home\n"), 0o644))
	assert.NoError(t, system.WriteSymlink("file", NewAbsPath(homeDir+"/.config/link")))
	assert.NoError(t, system.WriteFile(NewAbsPath(destDir+"/etc/file"), []byte("etc\n"), 0o644))
	assert.Equal(t, nobody, owner(homeDir+"/.config"))
	assert.Equal(t, nobody, owner(homeDir+"/.config/file"))
	assert.Equal(t, nobody, owner(homeDir+"/.config/link"))
	assert.Equal(t, 0, owner(destDir+"/etc/file"))
	assert.Equal(t, nobody, owner(homeDir))

	script := []byte("#!/bin/sh\nid -u > id\necho \"$HOME\" >> id\n")
	assert.NoError(t, system.RunScript(NewRelPath("home.sh"), NewAbsPath(homeDir), script, RunScriptOptions{}))
	assert.NoError(t, system.RunScript(NewRelPath("root.sh"), NewAbsPath(destDir+"/etc"), script, RunScriptOptions{}))
	homeID, err := os.ReadFile(homeDir + "/id")
	assert.NoError(t, err)
	assert.Equal(t, strconv.Itoa(nobody)+"\n"+homeDir+"\n", string(homeID))
	rootID, err := os.ReadFile(destDir + "/etc/id")
	assert.NoError(t, err)
	root, err := user.LookupId("0")
	assert.NoError(t, err)
	assert.Equal(t, "0\n"+root.HomeDir+"\n", string(rootID))
}

func TestHomeOwnerSystemWithoutHomeDir(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	destDir := t.TempDir()
	homeDir := destDir + "/home/nobody"
	assert.NoError(t, os.MkdirAll(homeDir, 0o755))
	assert.NoError(t, os.Chown(homeDir, 65534, 65534))

	system, err := NewHomeOwnerSystem(
		NewRealSystem(vfs.OSFS),
		NewAbsPath(destDir),
		NewAbsPath(homeDir),
		func() (bool, error) { return false, nil },
	)
	assert.NoError(t, err)
	assert.NoError(t, system.WriteFile(NewAbsPath(homeDir+"/file"), []byte("home\n"), 0o644))
	info, err := os.Lstat(homeDir + "/file")
	assert.NoError(t, err)
	assert.Equal(t, uint32(0), info.Sys().(*syscall.Stat_t).Uid) //nolint:forcetypeassert
}
