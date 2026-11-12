package doxygen

import (
	"os"
	"path/filepath"
	"testing"
)

/**
 * vocabulary collects marker "m" and cross-reference key "k".
 */
type vocabulary struct{}

/**
 * IsMarker implements Vocabulary.
 */
func (vocabulary) IsMarker(name string) bool { return name == "m" }

/**
 * IsXRefKey implements Vocabulary.
 */
func (vocabulary) IsXRefKey(key string) bool { return key == "k" }

/**
 * compoundXML is a namespace with one annotated function, listed twice as a
 * member would be in its namespace and its group.
 */
const compoundXML = `<?xml version="1.0"?>
<doxygen>
  <compounddef id="ns" kind="namespace">
    <compoundname>lib</compoundname>
    <sectiondef>
      <memberdef id="f1" kind="function">
        <name>f</name>
        <argsstring>(int x)</argsstring>
        <detaileddescription><para>Text. <m> A/B </m>
          <xrefsect id="k_1_k000001"><xreftitle>K</xreftitle><xrefdescription>
            <para><ref refid="g1">lib::g</ref></para>
            <para>missing::symbol</para>
          </xrefdescription></xrefsect>
          <xrefsect id="todo_1_todo000001"><xrefdescription><para>ignored</para></xrefdescription></xrefsect>
        </para></detaileddescription>
      </memberdef>
      <memberdef id="f1" kind="function"><name>duplicate</name></memberdef>
      <memberdef id="g1" kind="variable"><qualifiedname>lib::g</qualifiedname></memberdef>
    </sectiondef>
  </compounddef>
</doxygen>`

/**
 * TestRead checks entities, names, de-duplication and annotations.
 */
func TestRead(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"namespacelib.xml": compoundXML, "index.xml": "not xml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	index, err := Read(dir, vocabulary{})
	if err != nil {
		t.Fatal(err)
	}
	f, ok := index.Lookup("f1")
	if !ok || f.Name != "lib::f" || f.ArgsString != "(int x)" {
		t.Fatalf("f1: %+v", f)
	}
	if cmd, _ := f.DocCommand(); cmd != "@fn lib::f(int x)" {
		t.Errorf("doc command %q", cmd)
	}
	if got := f.Annotations.Markers["m"]; len(got) != 1 || got[0] != "A/B" {
		t.Errorf("markers %q", got)
	}
	items := f.Annotations.XRefs["k"]
	if len(items) != 2 || items[0].RefID != "g1" || items[1].RefID != "" || items[1].Text != "missing::symbol" {
		t.Errorf("xrefs %+v", items)
	}
	if len(f.Annotations.XRefs) != 1 {
		t.Errorf("collected a key outside the vocabulary: %v", f.Annotations.XRefs)
	}
	if got := index.Annotated(); len(got) != 1 || got[0] != f {
		t.Errorf("annotated %v", got)
	}
}

/**
 * TestReadInvalidXML checks a broken file is reported with its path.
 */
func TestReadInvalidXML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.xml"), []byte("<a><b></a>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir, vocabulary{}); err == nil {
		t.Error("expected an error")
	}
}
