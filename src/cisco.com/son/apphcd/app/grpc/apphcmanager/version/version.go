// Author  <dorzheho@cisco.com>

package version

const ApiVersion = "v1"

var (
	// Release is the current release of APPHC.
	// Update this whenever making a new release.
	// The release is of the format Major.Minor.Patch[-Prerelease][+BuildMetadata]
	//
	// If it is a development build, the release name is called "canary".
	//
	// Increment major number for new feature additions and behavioral changes.
	// Increment minor number for bug fixes and performance enhancements.
	// Increment patch number for critical fixes to existing releases.
	Release = "canary"
	// BuildMetadata is extra build time data
	BuildMetadata = ""
	// GitCommit is the git sha1
	GitCommit = ""
	// GitTreeState is the state of the git tree
	GitTreeState = ""
)

// Version contains the semver release, git commit, and git tree state.
type Version struct {
	Version      string `json:"version"`
	ApiVersion   string `json:"api-version"`
	GitCommit    string `json:"git-commit"`
	GitTreeState string `json:"git-tree-state"`
}

func (v *Version) String() string {
	return v.Version
}

// getVersion returns the semver string of the version
func getVersion() string {
	if BuildMetadata == "" {
		return Release
	}
	return Release + "+" + BuildMetadata
}

// New returns the semver interpretation of the version.
func New() *Version {
	return &Version{
		Version:      getVersion(),
		ApiVersion:   ApiVersion,
		GitCommit:    GitCommit,
		GitTreeState: GitTreeState,
	}
}
