/**
 * Package cli is doxythings' command-line interface: it parses arguments
 * and wires the reading, linking and rendering packages together.
 */
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

/**
 * Command is one subcommand, such as "doxygen" or "init".
 */
type Command interface {
	/**
	 * Name is what the user types to run it.
	 *
	 * @return The command's name.
	 */
	Name() string
	/**
	 * Summary is a one-line description for the usage message.
	 *
	 * @return The command's description.
	 */
	Summary() string
	/**
	 * Flags declares the command's options on fs, storing them in the
	 * command so Run can read them.
	 *
	 * @param fs The flag set to declare the options on.
	 */
	Flags(fs *flag.FlagSet)
	/**
	 * Run executes the command with its parsed options and its positional
	 * arguments, returning the process exit code.
	 *
	 * @param app The application, for its output streams and error reporting.
	 * @param args The positional arguments.
	 * @return The process exit code: ExitOK, ExitProblems or ExitError.
	 */
	Run(app *App, args []string) int
}

/**
 * Exit codes shared by every command.
 */
const (
	/** ExitOK means success. */
	ExitOK = 0
	/**
	 * ExitProblems means the command ran but reported problems it was
	 * asked to treat as failure (e.g. --strict).
	 */
	ExitProblems = 1
	/** ExitError means the command couldn't do its job, or was misused. */
	ExitError = 2
)

/**
 * App runs commands, writing to its own output streams so it can be tested
 * in-process.
 */
type App struct {
	/** Stdout receives normal output. */
	Stdout io.Writer
	/** Stderr receives problems and errors. */
	Stderr io.Writer
	/** Version is shown by the version command. */
	Version string
	/** commands are the available subcommands, in usage order. */
	commands []Command
}

/**
 * NewApp returns an App with every doxythings command.
 *
 * @param stdout Where normal output goes.
 * @param stderr Where problems are reported.
 * @param version The version the version command prints.
 * @return The application.
 */
func NewApp(stdout, stderr io.Writer, version string) *App {
	return &App{
		Stdout:  stdout,
		Stderr:  stderr,
		Version: version,
		commands: []Command{
			&doxygenCommand{}, &htmlCommand{}, &initCommand{}, &versionCommand{},
		},
	}
}

/**
 * Run runs the command named by args[0] with the remaining arguments and
 * returns the process exit code.
 *
 * @param args The command-line arguments, without the program name.
 * @return The process exit code.
 */
func (a *App) Run(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		a.usage(a.Stdout)
		if len(args) == 0 {
			return ExitError
		}
		return ExitOK
	}
	for _, c := range a.commands {
		if c.Name() == args[0] {
			return a.runCommand(c, args[1:])
		}
	}
	a.Errorf("unknown command %q", args[0])
	a.usage(a.Stderr)
	return ExitError
}

/**
 * Errorf reports a problem on Stderr, prefixed with the program name.
 *
 * @param format A fmt format string.
 * @param args The values format refers to.
 */
func (a *App) Errorf(format string, args ...any) {
	fmt.Fprintf(a.Stderr, "doxythings: "+format+"\n", args...)
}

/**
 * runCommand parses c's options and runs it.
 *
 * @param c The command to run.
 * @param args The arguments after the command's name.
 * @return The process exit code.
 */
func (a *App) runCommand(c Command, args []string) int {
	fs := flag.NewFlagSet("doxythings "+c.Name(), flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	c.Flags(fs)
	positional, err := parseInterspersed(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return ExitOK
	}
	if err != nil {
		return ExitError
	}
	return c.Run(a, positional)
}

/**
 * usage writes the list of commands to w.
 *
 * @param w Where to write the list.
 */
func (a *App) usage(w io.Writer) {
	var b strings.Builder
	b.WriteString("doxythings: test, coverage and benchmark results, linked into Doxygen documentation.\n\n")
	b.WriteString("Usage: doxythings <command> [options] [files]\n\nCommands:\n")
	for _, c := range a.commands {
		fmt.Fprintf(&b, "  %-8s %s\n", c.Name(), c.Summary())
	}
	b.WriteString("\nRun \"doxythings <command> -h\" for a command's options.\n")
	io.WriteString(w, b.String())
}

/**
 * parseInterspersed parses fs from args, allowing options after positional
 * arguments, and returns the positional arguments in order.
 *
 * @param fs The flag set to parse into.
 * @param args The arguments to parse.
 * @return The positional arguments, or an error for an unknown or malformed
 * option.
 */
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		// Parse drops a "--" that ends the options; everything after it is
		// positional, even if it looks like an option.
		if consumed := len(args) - len(rest); consumed > 0 && args[consumed-1] == "--" {
			return append(positional, rest...), nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}
