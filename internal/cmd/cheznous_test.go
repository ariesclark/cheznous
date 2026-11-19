package cmd

import (
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/coreos/go-semver/semver"
	vfs "github.com/twpayne/go-vfs/v5"

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
