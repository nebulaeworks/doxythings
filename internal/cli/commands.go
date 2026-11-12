package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nebulaeworks/doxythings/internal/assets"
	"github.com/nebulaeworks/doxythings/internal/doxygen"
	"github.com/nebulaeworks/doxythings/internal/gbench"
	"github.com/nebulaeworks/doxythings/internal/host"
	"github.com/nebulaeworks/doxythings/internal/link"
	"github.com/nebulaeworks/doxythings/internal/model"
	"github.com/nebulaeworks/doxythings/internal/render"
	"github.com/nebulaeworks/doxythings/internal/tags"
)

/**
 * SuiteLoader reads one result file into a Suite.
 */
type SuiteLoader interface {
	/**
	 * Load reads the file at path.
	 *
	 * @param path The result file.
	 * @return The results, or an error if the file can't be read or parsed.
	 */
	Load(path string) (*model.Suite, error)
}

/**
 * resultOptions are the options shared by commands that read benchmark
 * results.
 */
type resultOptions struct {
	/** output is the file to write. */
	output string
	/** title is the overview page's title. */
	title string
	/** buildType is the benchmarks' build type; Debug adds a warning. */
	buildType string
	/** loader reads result files. */
	loader SuiteLoader
}

/**
 * declare adds the shared options to fs.
 *
 * @param fs The flag set to declare the options on.
 */
func (o *resultOptions) declare(fs *flag.FlagSet) {
	fs.StringVar(&o.output, "output", "", "the file to write (required)")
	fs.StringVar(&o.title, "title", "Benchmark Results", "the overview page's title")
	fs.StringVar(&o.buildType, "build-type", "", "the benchmarks' build type; Debug adds a warning")
	o.loader = gbench.Loader{}
}

/**
 * suites loads every result file in paths, reporting and skipping any that
 * can't be read, and adds the build-type warning.
 *
 * @param app The application, to report unreadable files on.
 * @param paths The result files.
 * @return The suites that could be read, in the order given.
 */
func (o *resultOptions) suites(app *App, paths []string) []*model.Suite {
	var suites []*model.Suite
	for _, path := range paths {
		suite, err := o.loader.Load(path)
		if err != nil {
			app.Errorf("%s: skipped (%v)", path, err)
			continue
		}
		suites = append(suites, suite)
	}
	if strings.EqualFold(o.buildType, "debug") {
		for _, s := range suites {
			s.Warnings = append(s.Warnings, "The benchmarks were built as Debug: timings may not reflect a Release build.")
		}
	}
	return suites
}

/**
 * checkOutput reports a missing --output, returning false if it's missing.
 *
 * @param app The application, to report the problem on.
 * @return Whether --output was given.
 */
func (o *resultOptions) checkOutput(app *App) bool {
	if o.output == "" {
		app.Errorf("--output is required")
		return false
	}
	return true
}

/**
 * doxygenCommand writes Doxygen input for results.
 */
type doxygenCommand struct {
	/** resultOptions are the shared options. */
	resultOptions
	/** fragmentsDir is where HTML fragments go. */
	fragmentsDir string
	/** xmlDir is Doxygen XML output to link against; "" for none. */
	xmlDir string
	/** pageName is the overview page's `@page` name. */
	pageName string
	/** strict makes unmatched results or benchmarks an error. */
	strict bool
}

/**
 * Name implements Command.
 */
func (*doxygenCommand) Name() string { return "doxygen" }

/**
 * Summary implements Command.
 */
func (*doxygenCommand) Summary() string {
	return "write a .dox file and HTML fragments for Doxygen's next run"
}

/**
 * Flags implements Command.
 */
func (c *doxygenCommand) Flags(fs *flag.FlagSet) {
	c.declare(fs)
	fs.StringVar(&c.fragmentsDir, "fragments-dir", "", "where HTML fragments go; must be on Doxygen's EXAMPLE_PATH (required)")
	fs.StringVar(&c.xmlDir, "xml-dir", "", "Doxygen XML output to link results against")
	fs.StringVar(&c.pageName, "page-name", "benchmarks", "the overview page's @page name")
	fs.BoolVar(&c.strict, "strict", false, "exit non-zero if any result or benchmark is unmatched")
}

/**
 * Run implements Command.
 */
func (c *doxygenCommand) Run(app *App, args []string) int {
	if !c.checkOutput(app) {
		return ExitError
	}
	if c.fragmentsDir == "" {
		app.Errorf("--fragments-dir is required")
		return ExitError
	}
	suites := c.suites(app, args)
	var links *link.Result
	if c.xmlDir != "" {
		index, err := doxygen.Read(c.xmlDir, tags.All)
		if err != nil {
			app.Errorf("%v", err)
			return ExitError
		}
		links = link.Link(suites, link.FindBenchmarks(index))
	}
	renderer := &render.DoxygenRenderer{PageName: c.pageName, Title: c.title}
	out := renderer.Render(suites, host.NewProber().Details(suites), links)

	if err := writeFile(c.output, out.Dox); err != nil {
		app.Errorf("%v", err)
		return ExitError
	}
	if err := c.replaceFragments(out.Fragments); err != nil {
		app.Errorf("%v", err)
		return ExitError
	}
	problems := append(links.Problems(), out.Problems...)
	for _, p := range problems {
		app.Errorf("%s", p)
	}
	if c.strict && len(problems) > 0 {
		return ExitProblems
	}
	return ExitOK
}

/**
 * replaceFragments removes fragments left by earlier runs, then writes
 * fragments.
 *
 * @param fragments The fragments to write, by file name.
 * @return An error if a file couldn't be removed or written.
 */
func (c *doxygenCommand) replaceFragments(fragments map[string]string) error {
	if err := os.MkdirAll(c.fragmentsDir, 0o755); err != nil {
		return err
	}
	stale, err := filepath.Glob(filepath.Join(c.fragmentsDir, render.AnchorPrefix+"*.html"))
	if err != nil {
		return err
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	for name, content := range fragments {
		if err := writeFile(filepath.Join(c.fragmentsDir, name), content); err != nil {
			return err
		}
	}
	return nil
}

/**
 * htmlCommand writes results as a standalone HTML page.
 */
type htmlCommand struct {
	/** resultOptions are the shared options. */
	resultOptions
}

/**
 * Name implements Command.
 */
func (*htmlCommand) Name() string { return "html" }

/**
 * Summary implements Command.
 */
func (*htmlCommand) Summary() string { return "write results as a standalone HTML page" }

/**
 * Flags implements Command.
 */
func (c *htmlCommand) Flags(fs *flag.FlagSet) { c.declare(fs) }

/**
 * Run implements Command.
 */
func (c *htmlCommand) Run(app *App, args []string) int {
	if !c.checkOutput(app) {
		return ExitError
	}
	suites := c.suites(app, args)
	page := (&render.StandaloneRenderer{Title: c.title}).Render(suites, host.NewProber().Details(suites))
	if err := writeFile(c.output, page); err != nil {
		app.Errorf("%v", err)
		return ExitError
	}
	return ExitOK
}

/**
 * initCommand writes the Doxyfile fragment and stylesheet projects need.
 */
type initCommand struct {
	/** dir is where to write them. */
	dir string
}

/**
 * Name implements Command.
 */
func (*initCommand) Name() string { return "init" }

/**
 * Summary implements Command.
 */
func (*initCommand) Summary() string {
	return "write the Doxyfile fragment (tags, stylesheet) to @INCLUDE"
}

/**
 * Flags implements Command.
 */
func (c *initCommand) Flags(fs *flag.FlagSet) {
	fs.StringVar(&c.dir, "dir", ".", "where to write "+assets.DoxyfileName+" and "+assets.StylesheetName)
}

/**
 * Run implements Command.
 */
func (c *initCommand) Run(app *App, args []string) int {
	written, err := assets.Installer{Tags: tags.All}.Install(c.dir)
	if err != nil {
		app.Errorf("%v", err)
		return ExitError
	}
	for _, path := range written {
		fmt.Fprintln(app.Stdout, path)
	}
	return ExitOK
}

/**
 * versionCommand prints the version.
 */
type versionCommand struct{}

/**
 * Name implements Command.
 */
func (*versionCommand) Name() string { return "version" }

/**
 * Summary implements Command.
 */
func (*versionCommand) Summary() string { return "print the version" }

/**
 * Flags implements Command.
 */
func (*versionCommand) Flags(fs *flag.FlagSet) {}

/**
 * Run implements Command.
 */
func (*versionCommand) Run(app *App, args []string) int {
	fmt.Fprintln(app.Stdout, "doxythings "+app.Version)
	return ExitOK
}

/**
 * writeFile writes content to path, creating its directory if needed.
 *
 * @param path The file to write.
 * @param content What to write.
 * @return An error if the directory or file couldn't be written.
 */
func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
