package version

// These are overridden at build time via:
//   -ldflags "-X desktop/internal/version.Version=v1.2.3 -X desktop/internal/version.Commit=abc1234"
var (
	// Version is the running app version (e.g. "v1.2.3"). "dev" means an
	// un-stamped local build, which never auto-updates.
	Version = "dev"
	// Commit is the short git SHA of the build.
	Commit = "none"
	// RepoOwner / RepoName identify the GitHub repo hosting releases.
	RepoOwner = "DivXPro"
	RepoName  = "Anya"
)
