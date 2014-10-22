package profile_test

import (
	"github.com/davecheney/profile"
)

func ExampleStart() {
	// start a simple CPU profile and register
	// a defer to Stop (flush) the profiling data.
	defer profile.Start().Stop()
}

func ExampleMemProfile() {
	defer profile.Start(profile.MemProfile).Stop()
}

func ExampleNoShutdownHook() {
	// start a CPU profileri with a custom path.
	defer profile.Start(profile.CPUProfile, profile.ProfilePath("/home/dfc"), profile.NoShutdownHook).Stop()
}
