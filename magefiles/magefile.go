// Mage port of taskfile.yml / mise.toml. `mage -l` lists the targets.
//
// Staleness is mage's own: mg.Deps for the dependency graph and mage/target for
// the modification-time comparisons. That is coarser than what go-task and mise
// do — they hash file contents, so rewriting a file with the same bytes leaves
// them fresh, while here it does not — but it is the model mage ships with, and
// it needs no cache of its own.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"
)

// Default runs when mage is invoked without a target.
var Default = Build

// generateStamp records the last successful Generate. Its outputs are scattered
// across the same trees as its inputs — docs/, *.enum.gen.go, */sqlgen/ — so
// there is no single generated file to compare the sources against.
const generateStamp = ".mage/generate.stamp"

// sourceDirs are the trees the generators read.
var sourceDirs = []string{"cmd", "internal", "pkg"}

// binaries maps each built binary to its main package.
var binaries = map[string]string{
	"bin/backend":       "./cmd/backend",
	"bin/shoplannerctl": "./cmd/shoplannerctl",
}

// Generate runs swag init and go generate.
func Generate() error {
	stale, err := target.Dir(generateStamp, sourceDirs...)
	if err != nil {
		return fmt.Errorf("stat sources: %w", err)
	}
	if !stale {
		return nil
	}

	env, err := shimEnv()
	if err != nil {
		return err
	}
	swag := []string{"tool", "github.com/swaggo/swag/cmd/swag", "init", "-g", "cmd/backend/main.go"}
	if err = sh.RunWithV(env, "go", swag...); err != nil {
		return err
	}
	if err = sh.RunWithV(env, "go", "generate", "./..."); err != nil {
		return err
	}
	return touch(generateStamp)
}

// Build runs generate, then builds bin/backend and bin/shoplannerctl.
func Build() error {
	mg.Deps(Generate)

	for out, pkg := range binaries {
		stale, err := target.Dir(out, append(sourceDirs, "docs")...)
		if err != nil {
			return fmt.Errorf("stat sources: %w", err)
		}
		if !stale {
			continue
		}
		if err := sh.RunV("go", "build", "-ldflags=-w -s", "-o", out, pkg); err != nil {
			return err
		}
	}
	return nil
}

// Run builds and runs bin/backend --config config/backend.yml.
func Run() error {
	mg.Deps(Build)
	return sh.RunV("bin/backend", "--config", "config/backend.yml")
}

// Test runs go test ./... -race.
func Test() error {
	return sh.RunV("go", "test", "./...", "-race")
}

// TestUpdate regenerates internal/backend/functest/testdata.
func TestUpdate() error {
	return sh.RunV("go", "test", "./internal/backend/functest/", "-update")
}

// Lint runs golangci-lint.
func Lint() error {
	return sh.RunV("golangci-lint", "run")
}

// Fmt runs swag fmt and go fmt ./...
func Fmt() error {
	if err := sh.RunV("swag", "fmt"); err != nil {
		return err
	}
	return sh.RunV("go", "fmt", "./...")
}

// Export writes the docker image to shoplanner.tar.
func Export() error {
	out, err := os.Create("shoplanner.tar")
	if err != nil {
		return fmt.Errorf("create tar: %w", err)
	}
	defer out.Close()

	_, err = sh.Exec(nil, out, os.Stderr,
		"docker", "buildx", "build", "-o", "type=docker,dest=-", "-t", "shoplanner-backend:latest", ".")
	return err
}

// PackageDeb builds the .deb into dist/.
func PackageDeb() error {
	return sh.RunV("packaging/deb/build.sh")
}

// Clean removes the built binaries and the generate stamp, so the next run
// rebuilds and regenerates everything.
func Clean() error {
	for out := range binaries {
		if err := sh.Rm(out); err != nil {
			return err
		}
	}
	return sh.Rm(filepath.Dir(generateStamp))
}

// shimEnv reproduces the taskfile's env block: the tools/ shims read these.
func shimEnv() (map[string]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	return map[string]string{
		"PROJECT_ROOT": root,
		"SQLC_HELPER":  filepath.Join(root, "tools", "sqlc_helper.py"),
		"GOENUM":       filepath.Join(root, "tools", "goenum.py"),
	}, nil
}

// touch creates or refreshes a stamp file.
func touch(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return f.Close()
}
