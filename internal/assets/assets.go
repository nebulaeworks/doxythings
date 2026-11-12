/**
 * Package assets holds the files a project's Doxygen needs from doxythings,
 * compiled into the binary so that nothing else has to be installed.
 */
package assets

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nebulaeworks/doxythings/internal/tags"
)

/**
 * DoxyfileName is the Doxyfile fragment projects `@INCLUDE`.
 */
const DoxyfileName = "doxythings.doxyfile"

/**
 * StylesheetName is the stylesheet DoxyfileName adds to Doxygen's output.
 */
const StylesheetName = "doxythings.css"

/**
 * stylesheet styles the HTML doxythings generates.
 */
//go:embed doxythings.css
var stylesheet []byte

/**
 * Installer writes the assets into a directory.
 */
type Installer struct {
	/** Tags are the documentation tags the Doxyfile fragment defines. */
	Tags tags.Set
}

/**
 * Install writes DoxyfileName and StylesheetName into dir, creating it if
 * needed, and returns the paths written. The Doxyfile fragment refers to the
 * stylesheet by absolute path, so it works from any working directory.
 *
 * @param dir The directory to write into.
 * @return The paths of the files written, or an error if dir or a file couldn't
 * be written.
 */
func (i Installer) Install(dir string) ([]string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	stylesheetPath := filepath.Join(dir, StylesheetName)
	doxyfile := i.Tags.Doxyfile() +
		fmt.Sprintf("\n# Styles doxythings' collapsible sections.\nHTML_EXTRA_STYLESHEET += \"%s\"\n", stylesheetPath)
	files := []struct {
		path    string
		content []byte
	}{
		{filepath.Join(dir, DoxyfileName), []byte(doxyfile)},
		{stylesheetPath, stylesheet},
	}
	written := make([]string, 0, len(files))
	for _, f := range files {
		if err := os.WriteFile(f.path, f.content, 0o644); err != nil {
			return written, err
		}
		written = append(written, f.path)
	}
	return written, nil
}
