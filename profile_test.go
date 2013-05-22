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

func ExampleStop() {
	// start a default profile and cleanly
	// flush the data to disk on exit.
	defer profile.Start(nil).Stop()
}
