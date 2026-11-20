package chezmoi

// A homeOwner is the user who owns the home directory. When root applies a
// destination directory that contains the home directory, a HomeOwnerSystem
// gives them what it writes there and runs the scripts there as them.
type homeOwner struct {
	uid    int
	gid    int
	groups []uint32
	env    []string
}
