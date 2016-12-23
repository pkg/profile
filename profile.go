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
	"sync/atomic"
)

const (
	cpuMode = iota
	memMode
	blockMode
	traceMode
)

// Profile represents an active profiling session.
type Profile struct {
	// quiet suppresses informational messages during profiling.
	quiet bool

	// noShutdownHook controls whether the profiling package should
	// hook SIGINT to write profiles cleanly.
	noShutdownHook bool

	// mode holds the type of profiling that will be made
	mode int

	// path holds the base path where various profiling files are  written.
	// If blank, the base path will be generated by ioutil.TempDir.
	path string

	// memProfileRate holds the rate for the memory profile.
	memProfileRate int

	// closer holds a cleanup function that run after each profile
	closer func()

	// stopped records if a call to profile.Stop has been made
	stopped uint32
}

// NoShutdownHook controls whether the profiling package should
// hook SIGINT to write profiles cleanly.
// Programs with more sophisticated signal handling should set
// this to true and ensure the Stop() function returned from Start()
// is called during shutdown.
func NoShutdownHook(p *Profile) { p.noShutdownHook = true }

// Quiet suppresses informational messages during profiling.
func Quiet(p *Profile) { p.quiet = true }

// CPUProfile enables cpu profiling.
// It disables any previous profiling settings.
func CPUProfile(p *Profile) { p.mode = cpuMode }

// DefaultMemProfileRate is the default memory profiling rate.
// See also http://golang.org/pkg/runtime/#pkg-variables
const DefaultMemProfileRate = 4096

// MemProfile enables memory profiling.
// It disables any previous profiling settings.
func MemProfile(p *Profile) {
	p.memProfileRate = DefaultMemProfileRate
	p.mode = memMode
}

// MemProfileRate enables memory profiling at the preferred rate.
// It disables any previous profiling settings.
func MemProfileRate(rate int) func(*Profile) {
	return func(p *Profile) {
		p.memProfileRate = rate
		p.mode = memMode
	}
}

// BlockProfile enables block (contention) profiling.
// It disables any previous profiling settings.
func BlockProfile(p *Profile) { p.mode = blockMode }

// Trace profile controls if execution tracing will be enabled. It disables any previous profiling settings.
func TraceProfile(p *Profile) { p.mode = traceMode }

// ProfilePath controls the base path where various profiling
// files are written. If blank, the base path will be generated
// by ioutil.TempDir.
func ProfilePath(path string) func(*Profile) {
	return func(p *Profile) {
		p.path = path
	}
}

// Stop stops the profile and flushes any unwritten data.
func (p *Profile) Stop() {
	if !atomic.CompareAndSwapUint32(&p.stopped, 0, 1) {
		// someone has already called close
		return
	}
	p.closer()
	atomic.StoreUint32(&started, 0)
}

// started is non zero if a profile is running.
var started uint32

// Start starts a new profiling session.
// The caller should call the Stop method on the value returned
// to cleanly stop profiling.
func Start(options ...func(*Profile)) interface {
	Stop()
} {
	if !atomic.CompareAndSwapUint32(&started, 0, 1) {
		log.Fatal("profile: Start() already called")
	}

	var prof Profile
	for _, option := range options {
		option(&prof)
	}

	path, err := func() (string, error) {
		if p := prof.path; p != "" {
			return p, os.MkdirAll(p, 0777)
		}
		return ioutil.TempDir("", "profile")
	}()

	if err != nil {
		log.Fatalf("profile: could not create initial output directory: %v", err)
	}

	logf := func(format string, args ...interface{}) {
		if !prof.quiet {
			log.Printf(format, args...)
		}
	}

	switch prof.mode {
	case cpuMode:
		fn := filepath.Join(path, "cpu.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("profile: could not create cpu profile %q: %v", fn, err)
		}
		logf("profile: cpu profiling enabled, %s", fn)
		pprof.StartCPUProfile(f)
		prof.closer = func() {
			pprof.StopCPUProfile()
			f.Close()
			logf("profile: cpu profiling disabled, %s", fn)
		}

	case memMode:
		fn := filepath.Join(path, "mem.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("profile: could not create memory profile %q: %v", fn, err)
		}
		old := runtime.MemProfileRate
		runtime.MemProfileRate = prof.memProfileRate
		logf("profile: memory profiling enabled (rate %d), %s", runtime.MemProfileRate, fn)
		prof.closer = func() {
			pprof.Lookup("heap").WriteTo(f, 0)
			f.Close()
			runtime.MemProfileRate = old
			logf("profile: memory profiling disabled, %s", fn)
		}

	case blockMode:
		fn := filepath.Join(path, "block.pprof")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("profile: could not create block profile %q: %v", fn, err)
		}
		runtime.SetBlockProfileRate(1)
		logf("profile: block profiling enabled, %s", fn)
		prof.closer = func() {
			pprof.Lookup("block").WriteTo(f, 0)
			f.Close()
			runtime.SetBlockProfileRate(0)
			logf("profile: block profiling disabled, %s", fn)
		}

	case traceMode:
		fn := filepath.Join(path, "trace.out")
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("profile: could not create trace output file %q: %v", fn, err)
		}
		if err := startTrace(f); err != nil {
			log.Fatalf("profile: could not start trace: %v", err)
		}
		logf("profile: trace enabled, %s", fn)
		prof.closer = func() {
			stopTrace()
			logf("profile: trace disabled, %s", fn)
		}
	}

	prof.closers = append(prof.closers, func() {
		atomic.SwapUint32(&started, 0)
	})

	if !prof.noShutdownHook {
		go func() {
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			<-c

			log.Println("profile: caught interrupt, stopping profiles")
			// Stop receiving any more interrupts, while exiting.
			signal.Stop(c)
			// Stop profiling calling all closers.
			prof.Stop()

			// Exit peacefully.
			os.Exit(0)
		}()
	}

	return &prof
}
