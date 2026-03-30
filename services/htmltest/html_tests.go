// Package htmltest provides utilities for testing Web/HTML contexts with models.
package htmltest

import (
	"bytes"
	"html/template"
	"io"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HTMLDoc struct
type HTMLDoc struct {
	doc *goquery.Document
}

// NewHTMLParser parse html file
func NewHTMLParserFromBuffer(t testing.TB, body *bytes.Buffer) *HTMLDoc {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(body)
	require.NoError(t, err)
	return &HTMLDoc{doc: doc}
}

func NewHTMLParserFromReader(t testing.TB, reader io.Reader) *HTMLDoc {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(reader)
	require.NoError(t, err)
	return &HTMLDoc{doc: doc}
}

func NewHTMLParserFromString(t testing.TB, str string) *HTMLDoc {
	return NewHTMLParserFromReader(t, strings.NewReader(str))
}

func NewHTMLParserFromTemplateHTML(t testing.TB, template template.HTML) *HTMLDoc {
	return NewHTMLParserFromString(t, string(template))
}

// GetInputValueByID for get input value by id
func (doc *HTMLDoc) GetInputValueByID(id string) string {
	text, _ := doc.doc.Find("#" + id).Attr("value")
	return text
}

// GetInputValueByName for get input value by name
func (doc *HTMLDoc) GetInputValueByName(name string) string {
	text, _ := doc.doc.Find("input[name=\"" + name + "\"]").Attr("value")
	return text
}

// Find gets the descendants of each element in the current set of
// matched elements, filtered by a selector. It returns a new Selection
// object containing these matched elements.
func (doc *HTMLDoc) Find(selector string) *goquery.Selection {
	return doc.doc.Find(selector)
}

// GetCSRF for getting CSRF token value from input
func (doc *HTMLDoc) GetCSRF() string {
	return doc.GetInputValueByName("_csrf")
}

// AssertElement check if element by selector exists or does not exist depending on checkExists
func (doc *HTMLDoc) AssertElement(t testing.TB, selector string, checkExists bool) {
	AssertElement(t, doc.doc.Selection, selector, checkExists)
}

func AssertElement(t testing.TB, parentSelector *goquery.Selection, selector string, checkExists bool) {
	sel := parentSelector.Find(selector)
	if checkExists {
		assert.Equal(t, 1, sel.Length(), "should exist exactly one element for selector '%s'", selector)
	} else {
		assert.Equal(t, 0, sel.Length(), "should not exist any element for selector '%s'", selector)
	}
}

// AssertElementExists asserts that at least one element exists for the given selector.
func (doc *HTMLDoc) AssertElementExists(t testing.TB, selector string) {
	t.Helper()
	AssertElementExists(t, doc.doc.Selection, selector)
}

func AssertElementExists(t testing.TB, parentSelector *goquery.Selection, selector string) {
	t.Helper()
	sel := parentSelector.Find(selector)
	assert.Greater(t, sel.Length(), 0, "should exist at least one element for selector '%s'", selector)
}

// AssertElementNotExists asserts that no element exists for the given selector.
func (doc *HTMLDoc) AssertElementNotExists(t testing.TB, selector string) {
	t.Helper()
	AssertElementNotExists(t, doc.doc.Selection, selector)
}

func AssertElementNotExists(t testing.TB, parentSelector *goquery.Selection, selector string) {
	t.Helper()
	sel := parentSelector.Find(selector)
	assert.Equal(t, 0, sel.Length(), "should not exist any element for selector '%s'", selector)
}

// AssertElementCount asserts that an exact number of elements exist for the given selector.
func (doc *HTMLDoc) AssertElementCount(t testing.TB, selector string, count int) {
	t.Helper()
	AssertElementCount(t, doc.doc.Selection, selector, count)
}

func AssertElementCount(t testing.TB, parentSelector *goquery.Selection, selector string, count int) {
	t.Helper()
	sel := parentSelector.Find(selector)
	assert.Equal(t, count, sel.Length(), "should exist %d elements for selector '%s'", count, selector)
}

// AssertElementCount asserts that an exact number of elements exist for the given selector.
func (doc *HTMLDoc) AssertElementChildCount(t testing.TB, selector string, count int) {
	t.Helper()
	AssertElementChildCount(t, doc.doc.Selection, selector, count)
}

func AssertElementChildCount(t testing.TB, parentSelector *goquery.Selection, selector string, count int) {
	t.Helper()
	sel := parentSelector.Find(selector)
	size := sel.Children().Length()
	assert.Equal(t, count, size, "should exist %d elements for child selector '%s'", count, selector)
}

// AssertElementContains asserts that the first element for the selector contains the given text.
func (doc *HTMLDoc) AssertElementContains(t testing.TB, selector, text string) {
	t.Helper()
	AssertElementContains(t, doc.doc.Selection, selector, text)
}

func AssertElementContains(t testing.TB, parentSelector *goquery.Selection, selector, text string) {
	t.Helper()
	sel := parentSelector.Find(selector)
	require.Greater(t, sel.Length(), 0, "should exist at least one element for selector '%s' to check for text", selector)
	textFromSelector := sel.Text()
	assert.Contains(t, textFromSelector, text, "element '%s' should contain text '%s'", selector, text)
}

// AssertElementContains asserts that the first element for the selector contains the given text.
func (doc *HTMLDoc) AssertElementNotContains(t testing.TB, selector, text string) {
	t.Helper()
	AssertElementNotContains(t, doc.doc.Selection, selector, text)
}

func AssertElementNotContains(t testing.TB, parentSelector *goquery.Selection, selector, text string) {
	t.Helper()
	sel := parentSelector.Find(selector)
	length := sel.Length()
	if length > 0 {
		textFromSelector := sel.Text()
		assert.NotContains(t, textFromSelector, text, "element '%s' should contain text '%s'", selector, text)
	}
}

// AssertElementEmpty asserts that the first element for the selector is empty (has no text).
func (doc *HTMLDoc) AssertElementEmpty(t testing.TB, selector string) {
	t.Helper()
	AssertElementEmpty(t, doc.doc.Selection, selector)
}

func AssertElementEmpty(t testing.TB, parentSelector *goquery.Selection, selector string) {
	t.Helper()
	sel := parentSelector.Find(selector)
	require.Greater(t, sel.Length(), 0, "should exist at least one element for selector '%s' to check for emptiness", selector)
	assert.Empty(t, sel.First().Text(), "element '%s' should be empty", selector)
}
