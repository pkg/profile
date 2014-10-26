// Package profile provides a simple way to manage runtime/pprof
// profiling of your Go application.
package profile

import (
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
)

type profile struct {
	// Quiet suppresses informational messages during profiling.
	Quiet bool

	// CPUProfile controls if cpu profiling will be enabled.
	CPUProfile bool

	// MemProfile controls if memory profiling will be enabled.
	MemProfile bool

	// BlockProfile controls if block (contention) profiling will
	// be enabled.
	// It defaults to false.
	BlockProfile bool

	// ProfilePath controls the base path where various profiling
	// files are written. If blank, the base path will be generated
	// by ioutil.TempDir.
	ProfilePath string

	// NoShutdownHook controls whether the profiling package should
	// hook SIGINT to write profiles cleanly.
	NoShutdownHook bool

	// MemProfileRate sent the rate for the memory profile
	MemProfileRate int

	closers []func()
}

// NoShutdownHook controls whether the profiling package should
// hook SIGINT to write profiles cleanly.
// Programs with more sophisticated signal handling should set
// this to true and ensure the Stop() function returned from Start()
// is called during shutdown.
func NoShutdownHook(p *profile) { p.NoShutdownHook = true }

// Quiet suppresses informational messages during profiling.
func Quiet(p *profile) { p.Quiet = true }

func (p *profile) NoProfiles() {
	p.CPUProfile = false
	p.MemProfile = false
	p.BlockProfile = false
}

// CPUProfile controls if cpu profiling will be enabled. It disables any previous profiling settings.
func CPUProfile(p *profile) {
	p.NoProfiles()
	p.CPUProfile = true
}

// MemProfile controls if memory profiling will be enabled. It disables any previous profiling settings.
func MemProfile(p *profile) {
	p.NoProfiles()
	p.MemProfile = true
}

// BlockProfile controls if block (contention) profiling will be enabled. It disables any previous profiling settings.
func BlockProfile(p *profile) {
	p.NoProfiles()
	p.BlockProfile = true
}

// resolvePath resolves the profile's path or outputs to a temporarry directory
func resolvePath(path string) (resolvedPath string, err error) {
	if path != "" {
		return path, os.MkdirAll(path, 0777)
	}

	return ioutil.TempDir("", "profile")
}

func (p *profile) Stop() {
	for _, c := range p.closers {
		c()
	}
}

func newProfile() *profile {
	prof := &profile{MemProfileRate: 4096}
	CPUProfile(prof)
	return prof
}

// Start starts a new profiling session.
// The caller should call the Stop method on the value returned
// to cleanly stop profiling.
func Start(options ...func(*profile)) interface {
	Stop()
} {
	prof := newProfile()
	for _, option := range options {
		option(prof)
	}

	path, err := resolvePath(prof.ProfilePath)
	if err != nil {
		log.Fatalf("profile: could not create initial output directory: %v", err)
	}

	prof.ProfilePath = path
	if prof.Quiet {
		log.SetOutput(ioutil.Discard)
	}

	switch {
	case prof.CPUProfile:
		fn := filepath.Join(prof.ProfilePath, "cpu.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("profile: could not create cpu profile %q: %v", fn, err)
		}

		log.Printf("profile: cpu profiling enabled, %s", fn)
		pprof.StartCPUProfile(f)
		prof.closers = append(prof.closers, func() {
			pprof.StopCPUProfile()
			f.Close()
		})

	case prof.MemProfile:
		fn := filepath.Join(prof.ProfilePath, "mem.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("profile: could not create memory profile %q: %v", fn, err)
		}
		old := runtime.MemProfileRate
		runtime.MemProfileRate = prof.MemProfileRate

		log.Printf("profile: memory profiling enabled, %s", fn)
		prof.closers = append(prof.closers, func() {
			pprof.Lookup("heap").WriteTo(f, 0)
			f.Close()
			runtime.MemProfileRate = old
		})

	case prof.BlockProfile:
		fn := filepath.Join(prof.ProfilePath, "block.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("profile: could not create block profile %q: %v", fn, err)
		}
		runtime.SetBlockProfileRate(1)

		log.Printf("profile: block profiling enabled, %s", fn)
		prof.closers = append(prof.closers, func() {
			pprof.Lookup("block").WriteTo(f, 0)
			f.Close()
			runtime.SetBlockProfileRate(0)
		})
	}

	if !prof.NoShutdownHook {
		go func() {
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			<-c

			log.Println("profile: caught interrupt, stopping profiles")
			prof.Stop()

			os.Exit(0)
		}()
	}

	return prof
}
