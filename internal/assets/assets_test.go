package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nebulaeworks/doxythings/internal/tags"
)

/**
 * TestInstall checks both files are written and the fragment points at the
 * stylesheet by absolute path.
 */
func TestInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "conf")
	written, err := Installer{Tags: tags.All}.Install(dir)
	if err != nil || len(written) != 2 {
		t.Fatalf("written %v, err %v", written, err)
	}
	doxyfile, err := os.ReadFile(filepath.Join(dir, DoxyfileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doxyfile), tags.Benchmarks.Alias()) {
		t.Error("fragment lacks the tag aliases")
	}
	if !strings.Contains(string(doxyfile), `HTML_EXTRA_STYLESHEET += "`+filepath.Join(dir, StylesheetName)+`"`) {
		t.Errorf("fragment lacks the stylesheet:\n%s", doxyfile)
	}
	if css, _ := os.ReadFile(filepath.Join(dir, StylesheetName)); !strings.Contains(string(css), "details.doxythings") {
		t.Error("stylesheet missing or wrong")
	}
}
