package app

import (
	"fmt"
	"html"
	"io"
	"reflect"
)

// Text returns a text node.
func Text(v string) ValueNode {
	return &text{textValue: v}
}

type text struct {
	parentNode nodeWithChildren
	jsValue    Value
	textValue  string
}

func (t *text) nodeType() reflect.Type {
	return reflect.TypeOf(t)
}

func (t *text) JSValue() Value {
	return t.jsValue
}

func (t *text) parent() nodeWithChildren {
	return t.parentNode
}

func (t *text) setParent(p nodeWithChildren) {
	t.parentNode = p
}

func (t *text) dismount() {
	t.jsValue = nil
}

func (t *text) text() string {
	return t.textValue
}

func (t *text) mount() error {
	if t.jsValue != nil {
		return fmt.Errorf("node already mounted: %+v", t)
	}

	t.jsValue = Window().
		Get("document").
		Call("createTextNode", t.textValue)

	return nil
}

func (t *text) update(n textNode) {
	if text := n.text(); text != t.textValue {
		t.textValue = text
		t.jsValue.Set("nodeValue", text)
	}
}

func (t *text) html(w io.Writer) {
	t.htmlWithIndent(w, 0)
}

func (t *text) htmlWithIndent(w io.Writer, indent int) {
	writeIndent(w, indent)
	w.Write(stob(html.EscapeString(t.textValue)))
}