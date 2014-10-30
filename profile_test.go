package profile

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type checkFn func(t *testing.T, stdout, stderr []byte, err error)

var profileTests = []struct {
	name   string
	code   string
	checks []checkFn
}{{
	name: "default profile",
	code: `
package main

import "github.com/pkg/profile"

func main() {
	defer profile.Start().Stop()
}	
`,
	checks: []checkFn{NoStdout, NoErr},
}, {
	name: "profile quiet",
	code: `
package main

import "github.com/pkg/profile"

func main() {
        defer profile.Start(profile.Quiet).Stop()
}       
`,
	checks: []checkFn{NoStdout, NoStderr, NoErr},
}}

func TestProfile(t *testing.T) {
	for _, tt := range profileTests {
		t.Log(tt.name)
		stdout, stderr, err := runTest(t, tt.code)
		for _, f := range tt.checks {
			f(t, stdout, stderr, err)
		}
	}
}

// NoStdout checks that stdout was blank.
func NoStdout(t *testing.T, stdout, _ []byte, _ error) {
	if len := len(stdout); len > 0 {
		t.Errorf("stdout: wanted 0 bytes, got %d", len)
	}
}

// NoStderr checks that stderr was blank.
func NoStderr(t *testing.T, _, stderr []byte, _ error) {
	if len := len(stderr); len > 0 {
		t.Errorf("stderr: wanted 0 bytes, got %d", len)
	}
}

// NoErr checks that err was nil
func NoErr(t *testing.T, _, _ []byte, err error) {
	if err != nil {
		t.Errorf("error: expected nil, got %v", err)
	}
}

// runTest executes the go program supplied and returns the contents of stdout,
// stderr, and an error which may contain status information about the result
// of the program.
func runTest(t *testing.T, code string) ([]byte, []byte, error) {
	chk := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	gopath, err := ioutil.TempDir("", "profile-gopath")
	chk(err)
	defer os.RemoveAll(gopath)

	srcdir := filepath.Join(gopath, "src")
	err = os.Mkdir(srcdir, 0755)
	chk(err)
	src := filepath.Join(srcdir, "main.go")
	err = ioutil.WriteFile(src, []byte(code), 0644)
	chk(err)

	cmd := exec.Command("go", "run", src)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
