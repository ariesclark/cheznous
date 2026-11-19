# cheznous

cheznous is a fork of [chezmoi](https://github.com/twpayne/chezmoi). It is chezmoi with the changes below, rebased onto each chezmoi release. Everything else, including the source directory, the config file and the `.chezmoi*` files, works as the [chezmoi documentation](https://www.chezmoi.io/) describes.

## Install

On Linux and macOS:

```sh
sh -c "$(curl -fsLS https://raw.githubusercontent.com/ariesclark/cheznous/master/assets/scripts/install.sh)"
```

On Windows:

```powershell
iex "&{$(irm 'https://raw.githubusercontent.com/ariesclark/cheznous/master/assets/scripts/install.ps1')}"
```

Each release installs the same program under two names, `cheznous` and `chezmoi`. Help and completions use the name you run. `cheznous upgrade` installs the latest [release of this fork](https://github.com/ariesclark/cheznous/releases) and updates both names.

## Versions

A release is tagged `v<chezmoi version>-cheznous.<n>`, such as `v2.74.1-cheznous.1`: the `n`th cheznous release on top of chezmoi 2.74.1. A source directory whose `.chezmoiversion` asks for chezmoi 2.74.1 accepts it.

## Groups

A directory whose name is a Go template pipeline in parentheses is a group. Its contents apply to its parent directory when the pipeline is true, and are skipped when it is false. The group's name is not part of the target path:

```text
~/.local/share/chezmoi/
├── dot_gitconfig.tmpl                     → ~/.gitconfig
├── (eq .chezmoi.os `windows`)/
│   ├── dot_wslconfig                      → ~/.wslconfig, on Windows only
│   └── AppData/…                          → ~/AppData/…, on Windows only
└── (ne .chezmoi.os `windows`)/
    └── dot_bashrc                         → ~/.bashrc, everywhere but Windows
```

* The pipeline uses the same functions and data as any other template. It must print `true` or `false`; anything else is an error. Windows does not allow `"`, `/` or `\` in a file name, so write strings with backticks, as in `` `windows` ``.
* A group can be at any depth, such as `dot_config/(.work)/`. A group inside another group applies only when both are true.
* Two groups that are both true cannot give the same target, just as two entries in one directory cannot. Directories with the same name and attributes merge.
* `.chezmoiignore`, `.chezmoiremove`, `.chezmoiscripts`, `.chezmoitemplates` and `.chezmoiexternal` files in a group apply only when the group is true. `.chezmoidata` is not allowed in a group, so that every condition sees the same data.
* A group's condition sees the config data, the `.chezmoidata` next to it and above it, and chezmoi's own data such as `.chezmoi.os`. For anything a pipeline cannot express, compute a value in the config template and test it, as in `(.work)`.
* `edit`, `re-add`, `chattr` and `forget` keep a file in its group. `add` puts a new file outside any group, unless its parent directory is only in groups, where it joins the parent's group. ``add --group '(eq .chezmoi.os `linux`)'`` puts targets into that group instead, or moves them there, creating the group's directories; the group's condition must be true.
* A file with a name in parentheses is an ordinary file. For a directory whose target name is in parentheses, use the `literal_` prefix, as in `literal_(name)`.
