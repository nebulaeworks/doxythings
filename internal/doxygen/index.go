/**
 * Package doxygen reads Doxygen's XML output (GENERATE_XML = YES) into an
 * Index of documented entities and the doxythings annotations on them.
 *
 * It knows nothing about what an annotation means: a Vocabulary says which
 * XML markers and cross-reference lists to collect, and other packages
 * interpret them. Doxygen's ids are also what links are built from, so
 * nothing here depends on how a project lays out its HTML.
 */
package doxygen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

/**
 * Vocabulary says which annotations Read collects.
 */
type Vocabulary interface {
	/**
	 * IsMarker reports whether an XML element called name, found in a
	 * detailed description, is an annotation to collect.
	 *
	 * @param name The XML element's name.
	 * @return Whether to collect it.
	 */
	IsMarker(name string) bool
	/**
	 * IsXRefKey reports whether a cross-reference list (`\xrefitem`) with
	 * this key is an annotation to collect.
	 *
	 * @param key The list's key.
	 * @return Whether to collect it.
	 */
	IsXRefKey(key string) bool
}

/**
 * XRefItem is one entry of a cross-reference list on an entity.
 */
type XRefItem struct {
	/**
	 * RefID is the id of the entity the entry links to, or "" when Doxygen
	 * couldn't resolve it.
	 */
	RefID string
	/** Text is the entry's text, e.g. the symbol name as written. */
	Text string
}

/**
 * Annotations are the doxythings annotations on one entity.
 */
type Annotations struct {
	/** Markers maps a marker element's name to the text of each occurrence. */
	Markers map[string][]string
	/** XRefs maps a cross-reference key to its entries. */
	XRefs map[string][]XRefItem
}

/**
 * IsEmpty reports whether there are no annotations.
 *
 * @return Whether there are none.
 */
func (a Annotations) IsEmpty() bool {
	return len(a.Markers) == 0 && len(a.XRefs) == 0
}

/**
 * Entity is a documented member (function, variable...) or compound (class,
 * namespace, file...).
 */
type Entity struct {
	/** RefID is Doxygen's id for the entity, used in links and anchors. */
	RefID string
	/** Kind is Doxygen's kind, e.g. "function", "class" or "namespace". */
	Kind string
	/** Name is the fully qualified name, e.g. "lib::detail::to_core_json". */
	Name string
	/**
	 * ArgsString is a function's parameter list and qualifiers as Doxygen
	 * writes them, e.g. "(const V &value)"; "" for anything else.
	 */
	ArgsString string
	/** IsCompound is true for classes, namespaces, files and so on. */
	IsCompound bool
	/** Annotations holds the doxythings annotations in its description. */
	Annotations Annotations
}

/**
 * DocCommand returns the Doxygen command that attaches a separate
 * documentation block to this entity, e.g. "@fn lib::f(int x)", and false
 * for a kind that can't take one.
 *
 * @return The command, and whether the entity can take one.
 */
func (e *Entity) DocCommand() (string, bool) {
	if e.IsCompound {
		switch e.Kind {
		case "class", "struct", "union", "namespace", "concept":
			return "@" + e.Kind + " " + e.Name, true
		}
		return "", false
	}
	switch e.Kind {
	case "function", "signal", "slot", "friend":
		return "@fn " + e.Name + e.ArgsString, true
	case "variable":
		return "@var " + e.Name, true
	}
	return "", false
}

/**
 * Index is every entity found in one Doxygen XML directory.
 */
type Index struct {
	/** entities maps each RefID to its entity. */
	entities map[string]*Entity
	/**
	 * annotated lists the entities carrying annotations, in the order
	 * they were found.
	 */
	annotated []*Entity
}

/**
 * Read parses every compound file in xmlDir, collecting the annotations
 * vocabulary describes. It fails if a file isn't valid XML (e.g. a marker
 * containing '<' or '&', which Doxygen copies into the XML verbatim).
 *
 * @param xmlDir The directory of Doxygen's XML output.
 * @param vocabulary Which annotations to collect.
 * @return The index, or an error if a file can't be read or parsed.
 */
func Read(xmlDir string, vocabulary Vocabulary) (*Index, error) {
	paths, err := filepath.Glob(filepath.Join(xmlDir, "*.xml"))
	if err != nil {
		return nil, err
	}
	index := &Index{entities: map[string]*Entity{}}
	for _, path := range paths {
		if name := filepath.Base(path); name == "index.xml" || name == "Doxyfile.xml" {
			continue
		}
		if err := index.addFile(path, vocabulary); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	return index, nil
}

/**
 * Lookup returns the entity with this RefID, and whether there is one.
 *
 * @param refID The entity's Doxygen id.
 * @return The entity, and whether there is one.
 */
func (x *Index) Lookup(refID string) (*Entity, bool) {
	e, ok := x.entities[refID]
	return e, ok
}

/**
 * Annotated returns the entities carrying annotations, in the order found.
 *
 * @return The annotated entities.
 */
func (x *Index) Annotated() []*Entity {
	return x.annotated
}

/**
 * addFile adds the compounds and members of one XML file.
 *
 * @param path The XML file.
 * @param vocabulary Which annotations to collect.
 * @return An error if the file can't be read or parsed.
 */
func (x *Index) addFile(path string, vocabulary Vocabulary) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	root, err := parseElement(file)
	if err != nil {
		return err
	}
	for _, compound := range root.descendants("compounddef") {
		entity := &Entity{
			RefID:      compound.attr("id"),
			Kind:       compound.attr("kind"),
			Name:       compound.child("compoundname").text(),
			IsCompound: true,
		}
		x.entities[entity.RefID] = entity
		x.annotate(entity, compound, vocabulary)
		// A member is repeated in every compound listing it (its namespace,
		// file, group...), always under the same id; the first one wins.
		for _, member := range compound.descendants("memberdef") {
			id := member.attr("id")
			if _, seen := x.entities[id]; seen {
				continue
			}
			name := member.child("qualifiedname").text()
			if name == "" {
				name = entity.Name + "::" + member.child("name").text()
			}
			m := &Entity{
				RefID:      id,
				Kind:       member.attr("kind"),
				Name:       name,
				ArgsString: member.child("argsstring").text(),
			}
			x.entities[id] = m
			x.annotate(m, member, vocabulary)
		}
	}
	return nil
}

/**
 * annotate collects the annotations in definition's detailed description
 * onto entity, and records it as annotated if there are any.
 *
 * @param entity The entity to annotate.
 * @param definition The entity's compounddef or memberdef element.
 * @param vocabulary Which annotations to collect.
 */
func (x *Index) annotate(entity *Entity, definition *element, vocabulary Vocabulary) {
	description := definition.child("detaileddescription")
	if description == nil {
		return
	}
	a := Annotations{Markers: map[string][]string{}, XRefs: map[string][]XRefItem{}}
	description.walk(func(e *element) {
		if vocabulary.IsMarker(e.name) {
			a.Markers[e.name] = append(a.Markers[e.name], e.text())
		}
		if e.name != "xrefsect" {
			return
		}
		// Ids look like "<key>_1_<key>000001".
		key := e.attr("id")
		if i := strings.LastIndex(key, "_1_"); i >= 0 {
			key = key[:i]
		}
		if !vocabulary.IsXRefKey(key) {
			return
		}
		for _, para := range e.descendants("para") {
			item := XRefItem{Text: para.text()}
			if ref := para.child("ref"); ref != nil {
				item.RefID = ref.attr("refid")
			}
			if item.RefID != "" || item.Text != "" {
				a.XRefs[key] = append(a.XRefs[key], item)
			}
		}
	})
	if a.IsEmpty() {
		return
	}
	entity.Annotations = a
	x.annotated = append(x.annotated, entity)
}
