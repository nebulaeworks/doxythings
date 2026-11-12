package doxygen

import (
	"encoding/xml"
	"io"
	"strings"
)

/**
 * element is one XML element, kept with its mixed content so its text can be
 * read back in document order.
 */
type element struct {
	/** name is the element's local name. */
	name string
	/** attrs maps attribute names to values. */
	attrs map[string]string
	/**
	 * content holds, in document order, text (string) and child elements
	 * (*element).
	 */
	content []any
}

/**
 * parseElement reads a whole XML document and returns its root element.
 *
 * @param r The XML document.
 * @return The root element, or an error if the document isn't valid XML.
 */
func parseElement(r io.Reader) (*element, error) {
	decoder := xml.NewDecoder(r)
	var stack []*element
	var root *element
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			e := &element{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				e.attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.content = append(parent.content, e)
			} else {
				root = e
			}
			stack = append(stack, e)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.content = append(parent.content, string(t))
			}
		}
	}
	if root == nil {
		return nil, io.ErrUnexpectedEOF
	}
	return root, nil
}

/**
 * attr returns the attribute called name, or "" if there is none.
 *
 * @param name The attribute's name.
 * @return The attribute's value.
 */
func (e *element) attr(name string) string {
	return e.attrs[name]
}

/**
 * child returns the first direct child called name, or nil.
 *
 * @param name The child element's name.
 * @return The child, or nil.
 */
func (e *element) child(name string) *element {
	for _, c := range e.content {
		if child, ok := c.(*element); ok && child.name == name {
			return child
		}
	}
	return nil
}

/**
 * descendants returns e and every element below it called name, in document
 * order.
 *
 * @param name The elements' name.
 * @return The matching elements.
 */
func (e *element) descendants(name string) []*element {
	var found []*element
	e.walk(func(d *element) {
		if d.name == name {
			found = append(found, d)
		}
	})
	return found
}

/**
 * walk calls visit on e and then on every element below it, in document
 * order.
 *
 * @param visit Called on each element.
 */
func (e *element) walk(visit func(*element)) {
	visit(e)
	for _, c := range e.content {
		if child, ok := c.(*element); ok {
			child.walk(visit)
		}
	}
}

/**
 * text returns all the text inside e, trimmed of surrounding whitespace; ""
 * for a nil element.
 *
 * @return The trimmed text.
 */
func (e *element) text() string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	e.appendText(&b)
	return strings.TrimSpace(b.String())
}

/**
 * appendText writes all the text inside e to b, untrimmed.
 *
 * @param b Where to write the text.
 */
func (e *element) appendText(b *strings.Builder) {
	for _, c := range e.content {
		switch v := c.(type) {
		case string:
			b.WriteString(v)
		case *element:
			v.appendText(b)
		}
	}
}
