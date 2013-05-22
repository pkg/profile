// Package profile provides a simple way to manage runtime/pprof
// profiling of your Go application.
package profile

import (
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
)

// Config controls the operation of the profile package.
type Config struct {
	// Verbose controls the output of informational messages
	// during profiling. It defaults to true. If set to false
	// only error messages will be output.
	Verbose bool

	// CPUProfile controls if cpu profiling will be enabled.
	// It defaults to false.
	CPUProfile bool

	// MemProfile controls if memory profiling will be enabled.
	// It defaults to false.
	MemProfile bool

	// ProfilePath controls the base path where various profiling
	// files are written. It defaults to the output of
	// ioutil.TempDir.
	ProfilePath string

	// HandleInterrupt controls whether the profiling package should
	// hook SIGINT to write profiles cleanly.
	// It defaults to true, programs with more sophisticated signal
	// handling should set this to false and ensure the Stop() function
	// returned from Start() is called during shutdown.
	HandleInterrupt bool
}

var CPUProfile = &Config{
	Verbose:         true,
	CPUProfile:      true,
	HandleInterrupt: true,
}

var MemProfile = &Config{
	Verbose:         true,
	MemProfile:      true,
	HandleInterrupt: true,
}

func (c *Config) getVerbose() bool {
	if c == nil {
		return true
	}
	return c.Verbose
}

func (c *Config) getCPUProfile() bool {
	if c == nil {
		return false
	}
	return c.CPUProfile
}

func (c *Config) getMemProfile() bool {
	if c == nil {
		return false
	}
	return c.MemProfile
}

func (c *Config) getProfilePath() string {
	if c == nil {
		return ""
	}
	return c.ProfilePath
}

func (c *Config) getHandleInterrupt() bool {
	if c == nil {
		return true
	}
	return c.HandleInterrupt
}

type profile struct {
	path string
	*Config
	closers []func()
}

func (p *profile) Stop() {
	for _, c := range p.closers {
		c()
	}
}

// Start starts a new profiling session configured using *Config.
// The caller should call the Stop method on the value returned
// to cleanly stop profiling.
// Passing a nil *Config is the same as passing a *Config with
// defaults chosen.
func Start(cfg *Config) interface {
	Stop()
} {
	path := cfg.getProfilePath()
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
		Config: cfg,
	}

	if prof.getCPUProfile() {
		fn := filepath.Join(prof.path, "cpu.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatal("profile: could not create cpu profile file %q: %v", fn, err)
		}
		if prof.getVerbose() {
			log.Printf("profile: cpu profiling enabled, %s", fn)
		}
		pprof.StartCPUProfile(f)
		prof.closers = append(prof.closers, func() {
			pprof.StopCPUProfile()
			f.Close()
		})
	}

	if prof.getMemProfile() {
		fn := filepath.Join(prof.path, "mem.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatal("profile: could not create memory profile file %q: %v", fn, err)
		}
		if prof.getVerbose() {
			log.Printf("profile: memory profiling enabled, %s", fn)
		}
		prof.closers = append(prof.closers, func() {
			pprof.Lookup("heap").WriteTo(f, 0)
			f.Close()
		})
	}

	if prof.getHandleInterrupt() {
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
