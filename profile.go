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

type config struct {
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
}

// NoShutdownHook controls whether the profiling package should
// hook SIGINT to write profiles cleanly.
// Programs with more sophisticated signal handling should set
// this to true and ensure the Stop() function returned from Start()
// is called during shutdown.
func NoShutdownHook(c *config) { c.NoShutdownHook = true }

// Quiet suppresses informational messages during profiling.
func Quiet(c *config) { c.Quiet = true }

const memProfileRate = 4096

func (c *config) NoProfiles() {
	c.CPUProfile = false
	c.MemProfile = false
	c.BlockProfile = false
}

// CPUProfile controls if cpu profiling will be enabled. It disables an previous profiling settings.
func CPUProfile(c *config) {
	c.NoProfiles()
	c.CPUProfile = true
}

// MemProfile controls if memory profiling will be enabled. It disables any previous profiling settings.
func MemProfile(c *config) {
	c.NoProfiles()
	c.MemProfile = true
}

// BlockProfile controls if block (contention) profiling will be enabled. It disables any previous profiling settings.
func BlockProfile(c *config) {
	c.NoProfiles()
	c.BlockProfile = true
}

// ProfilePath controls the base path where various profiling
// files are written. If blank, the base path will be generated
// by ioutil.TempDir.
func ProfilePath(path string) func(*config) {
	return func(c *config) {
		c.ProfilePath = path
	}
}

type profile struct {
	path string
	*config
	closers []func()
}

func (p *profile) Stop() {
	for _, c := range p.closers {
		c()
	}
}

func defaultOptions() []func(*config) {
	return []func(*config){
		CPUProfile,
		ProfilePath(""),
	}
}

// Start starts a new profiling session.
// The caller should call the Stop method on the value returned
// to cleanly stop profiling.
func Start(options ...func(*config)) interface {
	Stop()
} {
	options = append(defaultOptions(), options...)
	var cfg config
	for _, option := range options {
		option(&cfg)
	}

	path := cfg.ProfilePath
	var err error
	if path == "" {
		path, err = ioutil.TempDir("", "profile")
	} else {
		err = os.MkdirAll(path, 0777)
	}
	if err != nil {
		log.Fatalf("profile: could not create initial output directory: %v", err)
	}
	prof := &profile{
		path:   path,
		config: &cfg,
	}

	if prof.CPUProfile {
		fn := filepath.Join(prof.path, "cpu.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("profile: could not create cpu profile %q: %v", fn, err)
		}
		if !prof.Quiet {
			log.Printf("profile: cpu profiling enabled, %s", fn)
		}
		pprof.StartCPUProfile(f)
		prof.closers = append(prof.closers, func() {
			pprof.StopCPUProfile()
			f.Close()
		})
	}

	if prof.MemProfile {
		fn := filepath.Join(prof.path, "mem.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("profile: could not create memory profile %q: %v", fn, err)
		}
		old := runtime.MemProfileRate
		runtime.MemProfileRate = memProfileRate
		if !prof.Quiet {
			log.Printf("profile: memory profiling enabled, %s", fn)
		}
		prof.closers = append(prof.closers, func() {
			pprof.Lookup("heap").WriteTo(f, 0)
			f.Close()
			runtime.MemProfileRate = old
		})
	}

	if prof.BlockProfile {
		fn := filepath.Join(prof.path, "block.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("profile: could not create block profile %q: %v", fn, err)
		}
		runtime.SetBlockProfileRate(1)
		if !prof.Quiet {
			log.Printf("profile: block profiling enabled, %s", fn)
		}
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
