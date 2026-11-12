package host

import (
	"testing"

	"github.com/nebulaeworks/doxythings/internal/model"
)

/**
 * fakeRunner returns canned output per command name.
 */
type fakeRunner map[string]string

/**
 * Output implements CommandRunner; unknown commands fail.
 */
func (f fakeRunner) Output(name string, _ ...string) (string, bool) {
	out, ok := f[name]
	return out, ok
}

/**
 * prober returns a Prober on machine "here" with canned command output.
 */
func prober() *Prober {
	return &Prober{
		Runner: fakeRunner{
			"lscpu":     "Model name:            Test CPU\nSocket(s):             1\n",
			"uname":     "6.1.0\n",
			"tuned-adm": "Current active profile: latency-performance\n",
		},
		Hostname: func() (string, error) { return "here", nil },
	}
}

/**
 * TestDetails checks facts are read and missing ones left out.
 */
func TestDetails(t *testing.T) {
	got := prober().Details([]*model.Suite{{HostName: "here"}})
	want := model.Facts{
		{Label: "CPU model", Value: "Test CPU"},
		{Label: "Sockets", Value: "1"},
		{Label: "Kernel", Value: "6.1.0"},
		{Label: "tuned profile", Value: "latency-performance"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("fact %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

/**
 * TestDetailsOtherMachine checks nothing is reported for results from
 * another machine, since it would describe the wrong hardware.
 */
func TestDetailsOtherMachine(t *testing.T) {
	if got := prober().Details([]*model.Suite{{HostName: "elsewhere"}}); got != nil {
		t.Errorf("got %v", got)
	}
}
