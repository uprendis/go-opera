package version

import (
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	params.VersionMajor = 0     // Major version component of the current release
	params.VersionMinor = 5     // Minor version component of the current release
	params.VersionPatch = 1     // Patch version component of the current release
	params.VersionMeta = "stable" // Version metadata to append to the version string
}
