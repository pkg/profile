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

	// CPUProfile controls if CPU profiling will be enabled.
	// It defaults to false
	CPUProfile bool

	// ProfilePath controls the base path where various profiling
	// files are written. It defaults to the output of
	// ioutil.TempDir.
	ProfilePath string
}

var CPUProfile = &Config{
	Verbose:    true,
	CPUProfile: true,
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

func (c *Config) getProfilePath() string {
	if c == nil {
		return ""
	}
	return c.ProfilePath
}

// Stopper represents a mechanism to finish a profiling run.
type Stopper interface {
	Stop()
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
// Passing a nil *Config is the same as passing a *Config with
// defaults chosen.
func Start(cfg *Config) Stopper {
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
			log.Printf("profile: cpu profiling enabled, %q", fn)
		}
		pprof.StartCPUProfile(f)
		prof.closers = append(prof.closers, pprof.StopCPUProfile)
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c

		log.Println("profile: caught interrupt, stopping profiles")
		prof.Stop()

		os.Exit(0)
	}()

	return prof
}
