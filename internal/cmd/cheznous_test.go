package cmd

import (
	"bytes"
	"errors"
	"runtime"
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/coreos/go-semver/semver"
	vfs "github.com/twpayne/go-vfs/v5"
	"github.com/twpayne/go-vfs/v5/vfst"

	"chezmoi.io/chezmoi/v2/internal/chezmoi"
	"chezmoi.io/chezmoi/v2/internal/chezmoitest"
)

func TestForkReleaseSatisfiesItsUpstreamBaseVersion(t *testing.T) {
	for _, command := range []string{"init", "apply"} {
		t.Run(command, func(t *testing.T) {
			chezmoitest.WithTestFS(t, map[string]any{
				"/home/user/.local/share/chezmoi": map[string]any{
					".chezmoiversion": "2.74.1",
					".git": map[string]any{
						".keep": nil,
					},
				},
			}, func(fileSystem vfs.FS) {
				config := newTestConfig(t, fileSystem, withVersionInfo(VersionInfo{
					Version: "v2.74.1-cheznous.1",
				}))
				assert.NoError(t, config.execute([]string{command}))
			})
		})
	}
}

func TestForkReleaseIsTooOldForANewerUpstreamVersion(t *testing.T) {
	for _, command := range []string{"init", "apply"} {
		t.Run(command, func(t *testing.T) {
			chezmoitest.WithTestFS(t, map[string]any{
				"/home/user/.local/share/chezmoi": map[string]any{
					".chezmoiversion": "2.75.0",
					".git": map[string]any{
						".keep": nil,
					},
				},
			}, func(fileSystem vfs.FS) {
				config := newTestConfig(t, fileSystem, withVersionInfo(VersionInfo{
					Version: "v2.74.1-cheznous.1",
				}))
				err := config.execute([]string{command})
				tooOldError, ok := errors.AsType[*chezmoi.TooOldError](err)
				assert.True(t, ok, "%v", err)
				assert.Equal(t, chezmoi.TooOldError{
					Have: *semver.New("2.74.1-cheznous.1"),
					Need: *semver.New("2.75.0"),
				}, *tooOldError)
			})
		})
	}
}

func TestUpstreamPreReleaseIsTooOldForItsReleaseVersion(t *testing.T) {
	chezmoitest.WithTestFS(t, map[string]any{
		"/home/user/.local/share/chezmoi": map[string]any{
			".chezmoiversion": "2.74.1",
			".git": map[string]any{
				".keep": nil,
			},
		},
	}, func(fileSystem vfs.FS) {
		config := newTestConfig(t, fileSystem, withVersionInfo(VersionInfo{
			Version: "v2.74.1-rc1",
		}))
		err := config.execute([]string{"init"})
		_, ok := errors.AsType[*chezmoi.TooOldError](err)
		assert.True(t, ok, "%v", err)
	})
}

func TestForkReleasesSortByUpstreamBaseThenForkNumber(t *testing.T) {
	ordered := []string{
		"2.74.1-cheznous.2",
		"2.74.1-cheznous.10",
		"2.75.0-cheznous.1",
	}
	for i := 1; i < len(ordered); i++ {
		assert.True(t, semver.New(ordered[i-1]).LessThan(*semver.New(ordered[i])), "%s < %s", ordered[i-1], ordered[i])
	}
}

func TestCommandNameFromExecutable(t *testing.T) {
	type testCase struct {
		executable          string
		expectedCommandName string
	}
	testCases := []testCase{
		{executable: "cheznous", expectedCommandName: "cheznous"},
		{executable: "chezmoi", expectedCommandName: "chezmoi"},
	}
	if runtime.GOOS == "windows" {
		testCases = append(testCases,
			testCase{executable: `C:\Users\user\.local\bin\cheznous.exe`, expectedCommandName: "cheznous"},
			testCase{executable: `C:\Users\user\.local\bin\CHEZMOI.EXE`, expectedCommandName: "CHEZMOI"},
		)
	} else {
		testCases = append(testCases,
			testCase{executable: "/usr/local/bin/cheznous", expectedCommandName: "cheznous"},
			testCase{executable: "/usr/local/bin/chezmoi", expectedCommandName: "chezmoi"},
		)
	}
	for _, tc := range testCases {
		t.Run(tc.executable, func(t *testing.T) {
			assert.Equal(t, tc.expectedCommandName, commandNameFromExecutable(tc.executable))
		})
	}
}

func TestCompletionRegistersTheCommandName(t *testing.T) {
	chezmoitest.WithTestFS(t, map[string]any{
		"/home/user": &vfst.Dir{Perm: 0o755},
	}, func(fileSystem vfs.FS) {
		var stdout bytes.Buffer
		config := newTestConfig(t, fileSystem, withCommandName("cheznous"), withStdout(&stdout))
		assert.NoError(t, config.execute([]string{"completion", "bash"}))
		assert.Contains(t, stdout.String(), "complete -o default -F __start_cheznous cheznous\n")
	})
}
