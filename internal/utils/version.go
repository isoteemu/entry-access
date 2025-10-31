package utils

import "runtime/debug"

// TODO: set by build system
var BuildVersion = ""

func GetVersion() string {
	if BuildVersion != "" {
		return BuildVersion
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		panic("Failed to read build info: binary may not have been built with Go modules")
	}

	// Check if dirty
	for _, setting := range info.Settings {
		if setting.Key == "vcs.modified" && setting.Value == "true" {
			return info.Main.Version + "-dirty"
		}
	}

	return info.Main.Version
}
