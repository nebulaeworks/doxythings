/**
 * Command doxythings links test, coverage and benchmark results into Doxygen
 * documentation. See package cli for the commands.
 */
package main

import (
	"os"
	"runtime/debug"

	"github.com/nebulaeworks/doxythings/internal/cli"
)

/**
 * version is set at release time with -ldflags "-X main.version=v1.2.3".
 */
var version = ""

/**
 * main runs the command line and exits with its status.
 */
func main() {
	os.Exit(cli.NewApp(os.Stdout, os.Stderr, resolvedVersion()).Run(os.Args[1:]))
}

/**
 * resolvedVersion returns the release version, else the module version
 * recorded by "go install module@version", else "dev".
 *
 * @return The version to report.
 */
func resolvedVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
