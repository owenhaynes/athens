// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cfg holds configuration shared by multiple parts
// of the go command.
package cfg

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// These are general "build flags" used by build and other commands.
var (
	BuildA                 bool               // -a flag
	BuildBuildmode         string             // -buildmode flag
	BuildMod               string             // -mod flag
	BuildI                 bool               // -i flag
	BuildLinkshared        bool               // -linkshared flag
	BuildMSan              bool               // -msan flag
	BuildN                 bool               // -n flag
	BuildO                 string             // -o flag
	BuildP                 = runtime.NumCPU() // -p flag
	BuildPkgdir            string             // -pkgdir flag
	BuildRace              bool               // -race flag
	BuildToolexec          []string           // -toolexec flag
	BuildToolchainName     string
	BuildToolchainCompiler func() string
	BuildToolchainLinker   func() string
	BuildTrimpath          bool // -trimpath flag
	BuildV                 bool // -v flag
	BuildWork              bool // -work flag
	BuildX                 bool // -x flag

	CmdName string // "build", "install", "list", "mod tidy", etc.

	DebugActiongraph string // -debug-actiongraph flag (undocumented, unstable)
)

func init() {
	BuildToolchainCompiler = func() string { return "missing-compiler" }
	BuildToolchainLinker = func() string { return "missing-linker" }
}

// An EnvVar is an environment variable Name=Value.
type EnvVar struct {
	Name  string
	Value string
}

// OrigEnv is the original environment of the program at startup.
var OrigEnv []string

// CmdEnv is the new environment for running go tool commands.
// User binaries (during go test or go run) are run with OrigEnv,
// not CmdEnv.
var CmdEnv []EnvVar

// Global build parameters (used during package load)
var (

	// ModulesEnabled specifies whether the go command is running
	// in module-aware mode (as opposed to GOPATH mode).
	// It is equal to modload.Enabled, but not all packages can import modload.
	ModulesEnabled bool
)

var envCache struct {
	once sync.Once
	m    map[string]string
}

// EnvFile returns the name of the Go environment configuration file.

// func initEnvCache() {
// 	envCache.m = make(map[string]string)
// 	file, _ := EnvFile()
// 	if file == "" {
// 		return
// 	}
// 	data, err := ioutil.ReadFile(file)
// 	if err != nil {
// 		return
// 	}

// 	for len(data) > 0 {
// 		// Get next line.
// 		line := data
// 		i := bytes.IndexByte(data, '\n')
// 		if i >= 0 {
// 			line, data = line[:i], data[i+1:]
// 		} else {
// 			data = nil
// 		}

// 		i = bytes.IndexByte(line, '=')
// 		if i < 0 || line[0] < 'A' || 'Z' < line[0] {
// 			// Line is missing = (or empty) or a comment or not a valid env name. Ignore.
// 			// (This should not happen, since the file should be maintained almost
// 			// exclusively by "go env -w", but better to silently ignore than to make
// 			// the go command unusable just because somehow the env file has
// 			// gotten corrupted.)
// 			continue
// 		}
// 		key, val := line[:i], line[i+1:]
// 		envCache.m[string(key)] = string(val)
// 	}
// }

// Getenv gets the value for the configuration key.
// It consults the operating system environment
// and then the go/env file.
// If Getenv is called for a key that cannot be set
// in the go/env file (for example GODEBUG), it panics.
// This ensures that CanGetenv is accurate, so that
// 'go env -w' stays in sync with what Getenv can retrieve.
// func Getenv(key string) string {
// 	// if !CanGetenv(key) {
// 	// 	switch key {
// 	// 	case "CGO_TEST_ALLOW", "CGO_TEST_DISALLOW", "CGO_test_ALLOW", "CGO_test_DISALLOW":
// 	// 		// used by internal/work/security_test.go; allow
// 	// 	default:
// 	// 		panic("internal error: invalid Getenv " + key)
// 	// 	}
// 	// }
// 	val := os.Getenv(key)
// 	if val != "" {
// 		return val
// 	}
// 	envCache.once.Do(initEnvCache)
// 	return envCache.m[key]
// }

// // CanGetenv reports whether key is a valid go/env configuration key.
// func CanGetenv(key string) bool {
// 	return strings.Contains(cfg.KnownEnv, "\t"+key+"\n")
// }

// GetArchEnv returns the name and setting of the
// GOARCH-specific architecture environment variable.
// If the current architecture has no GOARCH-specific variable,
// GetArchEnv returns empty key and value.

// There is a copy of findGOROOT, isSameDir, and isGOROOT in
// x/tools/cmd/godoc/goroot.go.
// Try to keep them in sync for now.

// findGOROOT returns the GOROOT value, using either an explicitly
// provided environment variable, a GOROOT that contains the current
// os.Executable value, or else the GOROOT that the binary was built
// with from runtime.GOROOT().
//
// There is a copy of this code in x/tools/cmd/godoc/goroot.go.
func findGOROOT() string {
	def := filepath.Clean(runtime.GOROOT())
	if runtime.Compiler == "gccgo" {
		// gccgo has no real GOROOT, and it certainly doesn't
		// depend on the executable's location.
		return def
	}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.Abs(exe)
		if err == nil {
			if dir := filepath.Join(exe, "../.."); isGOROOT(dir) {
				// If def (runtime.GOROOT()) and dir are the same
				// directory, prefer the spelling used in def.
				if isSameDir(def, dir) {
					return def
				}
				return dir
			}
			exe, err = filepath.EvalSymlinks(exe)
			if err == nil {
				if dir := filepath.Join(exe, "../.."); isGOROOT(dir) {
					if isSameDir(def, dir) {
						return def
					}
					return dir
				}
			}
		}
	}
	return def
}

// isSameDir reports whether dir1 and dir2 are the same directory.
func isSameDir(dir1, dir2 string) bool {
	if dir1 == dir2 {
		return true
	}
	info1, err1 := os.Stat(dir1)
	info2, err2 := os.Stat(dir2)
	return err1 == nil && err2 == nil && os.SameFile(info1, info2)
}

// isGOROOT reports whether path looks like a GOROOT.
//
// It does this by looking for the path/pkg/tool directory,
// which is necessary for useful operation of the cmd/go tool,
// and is not typically present in a GOPATH.
//
// There is a copy of this code in x/tools/cmd/godoc/goroot.go.
func isGOROOT(path string) bool {
	stat, err := os.Stat(filepath.Join(path, "pkg", "tool"))
	if err != nil {
		return false
	}
	return stat.IsDir()
}
