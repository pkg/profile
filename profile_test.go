package profile_test

import (
	"github.com/davecheney/profile"
)

func ExampleStart() {
	// start a simple CPU profile
	prof := profile.Start(profile.CPUProfile)
	// always call Stop to cleanly flush
	// profiling data to disk.
	defer prof.Stop()
}
