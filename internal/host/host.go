/**
 * Package host reads hardware details that benchmark harnesses don't
 * report, from the current machine.
 *
 * They only describe the results when the report is generated on the
 * machine the benchmarks ran on, so Prober.Details checks that first.
 */
package host

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/nebulaeworks/doxythings/internal/model"
)

/**
 * CommandRunner runs a command and returns its standard output.
 */
type CommandRunner interface {
	/**
	 * Output returns the command's output, and false if it couldn't be run
	 * or failed.
	 *
	 * @param name The program to run.
	 * @param args Its arguments.
	 * @return Its trimmed standard output, and whether it ran successfully.
	 */
	Output(name string, args ...string) (string, bool)
}

/**
 * ExecRunner runs real commands, giving each up to Timeout to finish.
 */
type ExecRunner struct {
	/** Timeout bounds how long one command may take. */
	Timeout time.Duration
}

/**
 * Output runs the command, failing if it errors or exceeds the timeout.
 *
 * @param name The program to run.
 * @param args Its arguments.
 * @return Its trimmed standard output, and whether it ran successfully.
 */
func (r ExecRunner) Output(name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return string(out), err == nil
}

/**
 * Prober gathers details about the current machine.
 */
type Prober struct {
	/** Runner runs the commands details come from. */
	Runner CommandRunner
	/** Hostname returns the current machine's network name. */
	Hostname func() (string, error)
}

/**
 * NewProber returns a Prober for the real machine.
 *
 * @return The prober.
 */
func NewProber() *Prober {
	return &Prober{Runner: ExecRunner{Timeout: 5 * time.Second}, Hostname: os.Hostname}
}

/**
 * Details returns extra facts about the current machine, or none if any
 * suite reports having run on a different one.
 *
 * @param suites The suites being reported, whose host names are checked against
 * this machine's.
 * @return The facts, possibly none.
 */
func (p *Prober) Details(suites []*model.Suite) model.Facts {
	current, _ := p.Hostname()
	for _, s := range suites {
		if s.HostName != "" && s.HostName != current {
			return nil
		}
	}
	return p.gather()
}

/**
 * gather reads what lscpu, uname and tuned-adm report, leaving out anything
 * unavailable.
 *
 * @return The facts found.
 */
func (p *Prober) gather() model.Facts {
	lscpu, _ := p.Runner.Output("lscpu")
	kernel, _ := p.Runner.Output("uname", "-r")
	all := model.Facts{
		{Label: "CPU model", Value: field(lscpu, "Model name")},
		{Label: "Sockets", Value: field(lscpu, "Socket(s)")},
		{Label: "Cores per socket", Value: field(lscpu, "Core(s) per socket")},
		{Label: "Threads per core", Value: field(lscpu, "Thread(s) per core")},
		{Label: "Max CPU MHz", Value: field(lscpu, "CPU max MHz")},
		{Label: "Kernel", Value: strings.TrimSpace(kernel)},
	}
	if tuned, ok := p.Runner.Output("tuned-adm", "active"); ok && tuned != "" {
		all = append(all, model.Fact{Label: "tuned profile", Value: tunedProfile(tuned)})
	}
	var found model.Facts
	for _, fact := range all {
		if fact.Value != "" {
			found = append(found, fact)
		}
	}
	return found
}

/**
 * field returns the value of a "Label: value" line in output, or "".
 *
 * @param output A command's output.
 * @param label The label to look for.
 * @return The trimmed value.
 */
func field(output, label string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(label) + `:[ \t]*(.+)$`)
	if m := re.FindStringSubmatch(output); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

/**
 * tunedProfilePattern finds the profile name in "tuned-adm active" output.
 */
var tunedProfilePattern = regexp.MustCompile(`profile:\s*(.+)`)

/**
 * tunedProfile extracts the active profile from "tuned-adm active" output,
 * or returns the whole output if it isn't in the expected form.
 *
 * @param output The output of tuned-adm active.
 * @return The profile name.
 */
func tunedProfile(output string) string {
	if m := tunedProfilePattern.FindStringSubmatch(output); m != nil {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(output)
}
