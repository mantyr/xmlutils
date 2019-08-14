package xmldecoder

import (
	"encoding/xml"

	"bytes"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Decoder struct {
	*xml.Decoder
	buf            bytes.Buffer
	saved          *bytes.Buffer
	stk            *stack
	free           *stack
	needClose      bool
	toClose        xml.Name
	nextToken      xml.Token
	nextByte       int
	ns             map[string]string
	err            error
	line           int
	offset         int64
	unmarshalDepth int
	
	unmarshaler bool
	depth       int64
	next        xml.Token
}

// NewDecoder creates a new XML parser reading from r.
// If r does not implement io.ByteReader, NewDecoder will
// do its own buffering.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		Decoder:  xml.NewDecoder(r),
		ns:       make(map[string]string),
		nextByte: -1,
		line:     1,
	}
}

const (
	xmlURL      = "http://www.w3.org/XML/1998/namespace"
	xmlnsPrefix = "xmlns"
	xmlPrefix   = "xml"
)

// Parsing state - stack holds old name space translations
// and the current set of open elements. The translations to pop when
// ending a given tag are *below* it on the stack, which is
// more work but forced on us by XML.
type stack struct {
	next *stack
	kind int
	name xml.Name
	ok   bool
}

const (
	stkStart = iota
	stkNs
	stkEOF
)

func (d *Decoder) push(kind int) *stack {
	s := d.free
	if s != nil {
		d.free = s.next
	} else {
		s = new(stack)
	}
	s.next = d.stk
	s.kind = kind
	d.stk = s
	return s
}

func (d *Decoder) pop() *stack {
	s := d.stk
	if s != nil {
		d.stk = s.next
		s.next = d.free
		d.free = s
	}
	return s
}

// Record that after the current element is finished
// (that element is already pushed on the stack)
// Token should return EOF until popEOF is called.
func (d *Decoder) pushEOF() {
//	d.eof = true
//	d.stk = 0
//	d.stop = false
}

// Undo a pushEOF.
// The element must have been finished, so the EOF should be at the top of the stack.
func (d *Decoder) popEOF() bool {
//	if d.stk == nil || d.stk.kind != stkEOF {
//		return false
//	}
//	d.pop()
	return true
}

// Record that we are starting an element with the given name.
func (d *Decoder) pushElement(name xml.Name) {
	s := d.push(stkStart)
	s.name = name
}

// Record that we are changing the value of ns[local].
// The old value is url, ok.
func (d *Decoder) pushNs(local string, url string, ok bool) {
	s := d.push(stkNs)
	s.name.Local = local
	s.name.Space = url
	s.ok = ok
}

// Creates a SyntaxError with the current line number.
func (d *Decoder) syntaxError(msg string) error {
	return &xml.SyntaxError{Msg: msg, Line: d.line}
}

// Record that we are ending an element with the given name.
// The name must match the record at the top of the stack,
// which must be a pushElement record.
// After popping the element, apply any undo records from
// the stack to restore the name translations that existed
// before we saw this element.
func (d *Decoder) popElement(t *xml.EndElement) bool {
	s := d.pop()
	name := t.Name
	switch {
	case s == nil || s.kind != stkStart:
		d.err = d.syntaxError("unexpected end element </" + name.Local + ">")
		return false
	case s.name.Local != name.Local:
		if !d.Strict {
			d.needClose = true
			d.toClose = t.Name
			t.Name = s.name
			return true
		}
		d.err = d.syntaxError("element <" + s.name.Local + "> closed by </" + name.Local + ">")
		return false
	case s.name.Space != name.Space:
		d.err = d.syntaxError("element <" + s.name.Local + "> in space " + s.name.Space +
			"closed by </" + name.Local + "> in space " + name.Space)
		return false
	}

	// Pop stack until a Start or EOF is on the top, undoing the
	// translations that were associated with the element we just closed.
	for d.stk != nil && d.stk.kind != stkStart && d.stk.kind != stkEOF {
		s := d.pop()
		if s.ok {
			d.ns[s.name.Local] = s.name.Space
		} else {
			delete(d.ns, s.name.Local)
		}
	}

	return true
}


// InputOffset returns the input stream byte offset of the current decoder position.
// The offset gives the location of the end of the most recently returned token
// and the beginning of the next token.
func (d *Decoder) InputOffset() int64 {
	return d.offset
}

// Return saved offset.
// If we did ungetc (nextByte >= 0), have to back up one.
func (d *Decoder) savedOffset() int {
	n := d.saved.Len()
	if d.nextByte >= 0 {
		n--
	}
	return n
}

var entity = map[string]int{
	"lt":   '<',
	"gt":   '>',
	"amp":  '&',
	"apos": '\'',
	"quot": '"',
}

// Decide whether the given rune is in the XML Character Range, per
// the Char production of https://www.xml.com/axml/testaxml.htm,
// Section 2.2 Characters.
func isInCharacterRange(r rune) (inrange bool) {
	return r == 0x09 ||
		r == 0x0A ||
		r == 0x0D ||
		r >= 0x20 && r <= 0xD7FF ||
		r >= 0xE000 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0x10FFFF
}

func isNameByte(c byte) bool {
	return 'A' <= c && c <= 'Z' ||
		'a' <= c && c <= 'z' ||
		'0' <= c && c <= '9' ||
		c == '_' || c == ':' || c == '.' || c == '-'
}

func isName(s []byte) bool {
	if len(s) == 0 {
		return false
	}
	c, n := utf8.DecodeRune(s)
	if c == utf8.RuneError && n == 1 {
		return false
	}
	if !unicode.Is(first, c) {
		return false
	}
	for n < len(s) {
		s = s[n:]
		c, n = utf8.DecodeRune(s)
		if c == utf8.RuneError && n == 1 {
			return false
		}
		if !unicode.Is(first, c) && !unicode.Is(second, c) {
			return false
		}
	}
	return true
}


// These tables were generated by cut and paste from Appendix B of
// the XML spec at https://www.xml.com/axml/testaxml.htm
// and then reformatting. First corresponds to (Letter | '_' | ':')
// and second corresponds to NameChar.

var first = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x003A, 0x003A, 1},
		{0x0041, 0x005A, 1},
		{0x005F, 0x005F, 1},
		{0x0061, 0x007A, 1},
		{0x00C0, 0x00D6, 1},
		{0x00D8, 0x00F6, 1},
		{0x00F8, 0x00FF, 1},
		{0x0100, 0x0131, 1},
		{0x0134, 0x013E, 1},
		{0x0141, 0x0148, 1},
		{0x014A, 0x017E, 1},
		{0x0180, 0x01C3, 1},
		{0x01CD, 0x01F0, 1},
		{0x01F4, 0x01F5, 1},
		{0x01FA, 0x0217, 1},
		{0x0250, 0x02A8, 1},
		{0x02BB, 0x02C1, 1},
		{0x0386, 0x0386, 1},
		{0x0388, 0x038A, 1},
		{0x038C, 0x038C, 1},
		{0x038E, 0x03A1, 1},
		{0x03A3, 0x03CE, 1},
		{0x03D0, 0x03D6, 1},
		{0x03DA, 0x03E0, 2},
		{0x03E2, 0x03F3, 1},
		{0x0401, 0x040C, 1},
		{0x040E, 0x044F, 1},
		{0x0451, 0x045C, 1},
		{0x045E, 0x0481, 1},
		{0x0490, 0x04C4, 1},
		{0x04C7, 0x04C8, 1},
		{0x04CB, 0x04CC, 1},
		{0x04D0, 0x04EB, 1},
		{0x04EE, 0x04F5, 1},
		{0x04F8, 0x04F9, 1},
		{0x0531, 0x0556, 1},
		{0x0559, 0x0559, 1},
		{0x0561, 0x0586, 1},
		{0x05D0, 0x05EA, 1},
		{0x05F0, 0x05F2, 1},
		{0x0621, 0x063A, 1},
		{0x0641, 0x064A, 1},
		{0x0671, 0x06B7, 1},
		{0x06BA, 0x06BE, 1},
		{0x06C0, 0x06CE, 1},
		{0x06D0, 0x06D3, 1},
		{0x06D5, 0x06D5, 1},
		{0x06E5, 0x06E6, 1},
		{0x0905, 0x0939, 1},
		{0x093D, 0x093D, 1},
		{0x0958, 0x0961, 1},
		{0x0985, 0x098C, 1},
		{0x098F, 0x0990, 1},
		{0x0993, 0x09A8, 1},
		{0x09AA, 0x09B0, 1},
		{0x09B2, 0x09B2, 1},
		{0x09B6, 0x09B9, 1},
		{0x09DC, 0x09DD, 1},
		{0x09DF, 0x09E1, 1},
		{0x09F0, 0x09F1, 1},
		{0x0A05, 0x0A0A, 1},
		{0x0A0F, 0x0A10, 1},
		{0x0A13, 0x0A28, 1},
		{0x0A2A, 0x0A30, 1},
		{0x0A32, 0x0A33, 1},
		{0x0A35, 0x0A36, 1},
		{0x0A38, 0x0A39, 1},
		{0x0A59, 0x0A5C, 1},
		{0x0A5E, 0x0A5E, 1},
		{0x0A72, 0x0A74, 1},
		{0x0A85, 0x0A8B, 1},
		{0x0A8D, 0x0A8D, 1},
		{0x0A8F, 0x0A91, 1},
		{0x0A93, 0x0AA8, 1},
		{0x0AAA, 0x0AB0, 1},
		{0x0AB2, 0x0AB3, 1},
		{0x0AB5, 0x0AB9, 1},
		{0x0ABD, 0x0AE0, 0x23},
		{0x0B05, 0x0B0C, 1},
		{0x0B0F, 0x0B10, 1},
		{0x0B13, 0x0B28, 1},
		{0x0B2A, 0x0B30, 1},
		{0x0B32, 0x0B33, 1},
		{0x0B36, 0x0B39, 1},
		{0x0B3D, 0x0B3D, 1},
		{0x0B5C, 0x0B5D, 1},
		{0x0B5F, 0x0B61, 1},
		{0x0B85, 0x0B8A, 1},
		{0x0B8E, 0x0B90, 1},
		{0x0B92, 0x0B95, 1},
		{0x0B99, 0x0B9A, 1},
		{0x0B9C, 0x0B9C, 1},
		{0x0B9E, 0x0B9F, 1},
		{0x0BA3, 0x0BA4, 1},
		{0x0BA8, 0x0BAA, 1},
		{0x0BAE, 0x0BB5, 1},
		{0x0BB7, 0x0BB9, 1},
		{0x0C05, 0x0C0C, 1},
		{0x0C0E, 0x0C10, 1},
		{0x0C12, 0x0C28, 1},
		{0x0C2A, 0x0C33, 1},
		{0x0C35, 0x0C39, 1},
		{0x0C60, 0x0C61, 1},
		{0x0C85, 0x0C8C, 1},
		{0x0C8E, 0x0C90, 1},
		{0x0C92, 0x0CA8, 1},
		{0x0CAA, 0x0CB3, 1},
		{0x0CB5, 0x0CB9, 1},
		{0x0CDE, 0x0CDE, 1},
		{0x0CE0, 0x0CE1, 1},
		{0x0D05, 0x0D0C, 1},
		{0x0D0E, 0x0D10, 1},
		{0x0D12, 0x0D28, 1},
		{0x0D2A, 0x0D39, 1},
		{0x0D60, 0x0D61, 1},
		{0x0E01, 0x0E2E, 1},
		{0x0E30, 0x0E30, 1},
		{0x0E32, 0x0E33, 1},
		{0x0E40, 0x0E45, 1},
		{0x0E81, 0x0E82, 1},
		{0x0E84, 0x0E84, 1},
		{0x0E87, 0x0E88, 1},
		{0x0E8A, 0x0E8D, 3},
		{0x0E94, 0x0E97, 1},
		{0x0E99, 0x0E9F, 1},
		{0x0EA1, 0x0EA3, 1},
		{0x0EA5, 0x0EA7, 2},
		{0x0EAA, 0x0EAB, 1},
		{0x0EAD, 0x0EAE, 1},
		{0x0EB0, 0x0EB0, 1},
		{0x0EB2, 0x0EB3, 1},
		{0x0EBD, 0x0EBD, 1},
		{0x0EC0, 0x0EC4, 1},
		{0x0F40, 0x0F47, 1},
		{0x0F49, 0x0F69, 1},
		{0x10A0, 0x10C5, 1},
		{0x10D0, 0x10F6, 1},
		{0x1100, 0x1100, 1},
		{0x1102, 0x1103, 1},
		{0x1105, 0x1107, 1},
		{0x1109, 0x1109, 1},
		{0x110B, 0x110C, 1},
		{0x110E, 0x1112, 1},
		{0x113C, 0x1140, 2},
		{0x114C, 0x1150, 2},
		{0x1154, 0x1155, 1},
		{0x1159, 0x1159, 1},
		{0x115F, 0x1161, 1},
		{0x1163, 0x1169, 2},
		{0x116D, 0x116E, 1},
		{0x1172, 0x1173, 1},
		{0x1175, 0x119E, 0x119E - 0x1175},
		{0x11A8, 0x11AB, 0x11AB - 0x11A8},
		{0x11AE, 0x11AF, 1},
		{0x11B7, 0x11B8, 1},
		{0x11BA, 0x11BA, 1},
		{0x11BC, 0x11C2, 1},
		{0x11EB, 0x11F0, 0x11F0 - 0x11EB},
		{0x11F9, 0x11F9, 1},
		{0x1E00, 0x1E9B, 1},
		{0x1EA0, 0x1EF9, 1},
		{0x1F00, 0x1F15, 1},
		{0x1F18, 0x1F1D, 1},
		{0x1F20, 0x1F45, 1},
		{0x1F48, 0x1F4D, 1},
		{0x1F50, 0x1F57, 1},
		{0x1F59, 0x1F5B, 0x1F5B - 0x1F59},
		{0x1F5D, 0x1F5D, 1},
		{0x1F5F, 0x1F7D, 1},
		{0x1F80, 0x1FB4, 1},
		{0x1FB6, 0x1FBC, 1},
		{0x1FBE, 0x1FBE, 1},
		{0x1FC2, 0x1FC4, 1},
		{0x1FC6, 0x1FCC, 1},
		{0x1FD0, 0x1FD3, 1},
		{0x1FD6, 0x1FDB, 1},
		{0x1FE0, 0x1FEC, 1},
		{0x1FF2, 0x1FF4, 1},
		{0x1FF6, 0x1FFC, 1},
		{0x2126, 0x2126, 1},
		{0x212A, 0x212B, 1},
		{0x212E, 0x212E, 1},
		{0x2180, 0x2182, 1},
		{0x3007, 0x3007, 1},
		{0x3021, 0x3029, 1},
		{0x3041, 0x3094, 1},
		{0x30A1, 0x30FA, 1},
		{0x3105, 0x312C, 1},
		{0x4E00, 0x9FA5, 1},
		{0xAC00, 0xD7A3, 1},
	},
}

var second = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x002D, 0x002E, 1},
		{0x0030, 0x0039, 1},
		{0x00B7, 0x00B7, 1},
		{0x02D0, 0x02D1, 1},
		{0x0300, 0x0345, 1},
		{0x0360, 0x0361, 1},
		{0x0387, 0x0387, 1},
		{0x0483, 0x0486, 1},
		{0x0591, 0x05A1, 1},
		{0x05A3, 0x05B9, 1},
		{0x05BB, 0x05BD, 1},
		{0x05BF, 0x05BF, 1},
		{0x05C1, 0x05C2, 1},
		{0x05C4, 0x0640, 0x0640 - 0x05C4},
		{0x064B, 0x0652, 1},
		{0x0660, 0x0669, 1},
		{0x0670, 0x0670, 1},
		{0x06D6, 0x06DC, 1},
		{0x06DD, 0x06DF, 1},
		{0x06E0, 0x06E4, 1},
		{0x06E7, 0x06E8, 1},
		{0x06EA, 0x06ED, 1},
		{0x06F0, 0x06F9, 1},
		{0x0901, 0x0903, 1},
		{0x093C, 0x093C, 1},
		{0x093E, 0x094C, 1},
		{0x094D, 0x094D, 1},
		{0x0951, 0x0954, 1},
		{0x0962, 0x0963, 1},
		{0x0966, 0x096F, 1},
		{0x0981, 0x0983, 1},
		{0x09BC, 0x09BC, 1},
		{0x09BE, 0x09BF, 1},
		{0x09C0, 0x09C4, 1},
		{0x09C7, 0x09C8, 1},
		{0x09CB, 0x09CD, 1},
		{0x09D7, 0x09D7, 1},
		{0x09E2, 0x09E3, 1},
		{0x09E6, 0x09EF, 1},
		{0x0A02, 0x0A3C, 0x3A},
		{0x0A3E, 0x0A3F, 1},
		{0x0A40, 0x0A42, 1},
		{0x0A47, 0x0A48, 1},
		{0x0A4B, 0x0A4D, 1},
		{0x0A66, 0x0A6F, 1},
		{0x0A70, 0x0A71, 1},
		{0x0A81, 0x0A83, 1},
		{0x0ABC, 0x0ABC, 1},
		{0x0ABE, 0x0AC5, 1},
		{0x0AC7, 0x0AC9, 1},
		{0x0ACB, 0x0ACD, 1},
		{0x0AE6, 0x0AEF, 1},
		{0x0B01, 0x0B03, 1},
		{0x0B3C, 0x0B3C, 1},
		{0x0B3E, 0x0B43, 1},
		{0x0B47, 0x0B48, 1},
		{0x0B4B, 0x0B4D, 1},
		{0x0B56, 0x0B57, 1},
		{0x0B66, 0x0B6F, 1},
		{0x0B82, 0x0B83, 1},
		{0x0BBE, 0x0BC2, 1},
		{0x0BC6, 0x0BC8, 1},
		{0x0BCA, 0x0BCD, 1},
		{0x0BD7, 0x0BD7, 1},
		{0x0BE7, 0x0BEF, 1},
		{0x0C01, 0x0C03, 1},
		{0x0C3E, 0x0C44, 1},
		{0x0C46, 0x0C48, 1},
		{0x0C4A, 0x0C4D, 1},
		{0x0C55, 0x0C56, 1},
		{0x0C66, 0x0C6F, 1},
		{0x0C82, 0x0C83, 1},
		{0x0CBE, 0x0CC4, 1},
		{0x0CC6, 0x0CC8, 1},
		{0x0CCA, 0x0CCD, 1},
		{0x0CD5, 0x0CD6, 1},
		{0x0CE6, 0x0CEF, 1},
		{0x0D02, 0x0D03, 1},
		{0x0D3E, 0x0D43, 1},
		{0x0D46, 0x0D48, 1},
		{0x0D4A, 0x0D4D, 1},
		{0x0D57, 0x0D57, 1},
		{0x0D66, 0x0D6F, 1},
		{0x0E31, 0x0E31, 1},
		{0x0E34, 0x0E3A, 1},
		{0x0E46, 0x0E46, 1},
		{0x0E47, 0x0E4E, 1},
		{0x0E50, 0x0E59, 1},
		{0x0EB1, 0x0EB1, 1},
		{0x0EB4, 0x0EB9, 1},
		{0x0EBB, 0x0EBC, 1},
		{0x0EC6, 0x0EC6, 1},
		{0x0EC8, 0x0ECD, 1},
		{0x0ED0, 0x0ED9, 1},
		{0x0F18, 0x0F19, 1},
		{0x0F20, 0x0F29, 1},
		{0x0F35, 0x0F39, 2},
		{0x0F3E, 0x0F3F, 1},
		{0x0F71, 0x0F84, 1},
		{0x0F86, 0x0F8B, 1},
		{0x0F90, 0x0F95, 1},
		{0x0F97, 0x0F97, 1},
		{0x0F99, 0x0FAD, 1},
		{0x0FB1, 0x0FB7, 1},
		{0x0FB9, 0x0FB9, 1},
		{0x20D0, 0x20DC, 1},
		{0x20E1, 0x3005, 0x3005 - 0x20E1},
		{0x302A, 0x302F, 1},
		{0x3031, 0x3035, 1},
		{0x3099, 0x309A, 1},
		{0x309D, 0x309E, 1},
		{0x30FC, 0x30FE, 1},
	},
}

// HTMLEntity is an entity map containing translations for the
// standard HTML entity characters.
//
// See the Decoder.Strict and Decoder.Entity fields' documentation.
var HTMLEntity map[string]string = htmlEntity

var htmlEntity = map[string]string{
	/*
		hget http://www.w3.org/TR/html4/sgml/entities.html |
		ssam '
			,y /\&gt;/ x/\&lt;(.|\n)+/ s/\n/ /g
			,x v/^\&lt;!ENTITY/d
			,s/\&lt;!ENTITY ([^ ]+) .*U\+([0-9A-F][0-9A-F][0-9A-F][0-9A-F]) .+/	"\1": "\\u\2",/g
		'
	*/
	"nbsp":     "\u00A0",
	"iexcl":    "\u00A1",
	"cent":     "\u00A2",
	"pound":    "\u00A3",
	"curren":   "\u00A4",
	"yen":      "\u00A5",
	"brvbar":   "\u00A6",
	"sect":     "\u00A7",
	"uml":      "\u00A8",
	"copy":     "\u00A9",
	"ordf":     "\u00AA",
	"laquo":    "\u00AB",
	"not":      "\u00AC",
	"shy":      "\u00AD",
	"reg":      "\u00AE",
	"macr":     "\u00AF",
	"deg":      "\u00B0",
	"plusmn":   "\u00B1",
	"sup2":     "\u00B2",
	"sup3":     "\u00B3",
	"acute":    "\u00B4",
	"micro":    "\u00B5",
	"para":     "\u00B6",
	"middot":   "\u00B7",
	"cedil":    "\u00B8",
	"sup1":     "\u00B9",
	"ordm":     "\u00BA",
	"raquo":    "\u00BB",
	"frac14":   "\u00BC",
	"frac12":   "\u00BD",
	"frac34":   "\u00BE",
	"iquest":   "\u00BF",
	"Agrave":   "\u00C0",
	"Aacute":   "\u00C1",
	"Acirc":    "\u00C2",
	"Atilde":   "\u00C3",
	"Auml":     "\u00C4",
	"Aring":    "\u00C5",
	"AElig":    "\u00C6",
	"Ccedil":   "\u00C7",
	"Egrave":   "\u00C8",
	"Eacute":   "\u00C9",
	"Ecirc":    "\u00CA",
	"Euml":     "\u00CB",
	"Igrave":   "\u00CC",
	"Iacute":   "\u00CD",
	"Icirc":    "\u00CE",
	"Iuml":     "\u00CF",
	"ETH":      "\u00D0",
	"Ntilde":   "\u00D1",
	"Ograve":   "\u00D2",
	"Oacute":   "\u00D3",
	"Ocirc":    "\u00D4",
	"Otilde":   "\u00D5",
	"Ouml":     "\u00D6",
	"times":    "\u00D7",
	"Oslash":   "\u00D8",
	"Ugrave":   "\u00D9",
	"Uacute":   "\u00DA",
	"Ucirc":    "\u00DB",
	"Uuml":     "\u00DC",
	"Yacute":   "\u00DD",
	"THORN":    "\u00DE",
	"szlig":    "\u00DF",
	"agrave":   "\u00E0",
	"aacute":   "\u00E1",
	"acirc":    "\u00E2",
	"atilde":   "\u00E3",
	"auml":     "\u00E4",
	"aring":    "\u00E5",
	"aelig":    "\u00E6",
	"ccedil":   "\u00E7",
	"egrave":   "\u00E8",
	"eacute":   "\u00E9",
	"ecirc":    "\u00EA",
	"euml":     "\u00EB",
	"igrave":   "\u00EC",
	"iacute":   "\u00ED",
	"icirc":    "\u00EE",
	"iuml":     "\u00EF",
	"eth":      "\u00F0",
	"ntilde":   "\u00F1",
	"ograve":   "\u00F2",
	"oacute":   "\u00F3",
	"ocirc":    "\u00F4",
	"otilde":   "\u00F5",
	"ouml":     "\u00F6",
	"divide":   "\u00F7",
	"oslash":   "\u00F8",
	"ugrave":   "\u00F9",
	"uacute":   "\u00FA",
	"ucirc":    "\u00FB",
	"uuml":     "\u00FC",
	"yacute":   "\u00FD",
	"thorn":    "\u00FE",
	"yuml":     "\u00FF",
	"fnof":     "\u0192",
	"Alpha":    "\u0391",
	"Beta":     "\u0392",
	"Gamma":    "\u0393",
	"Delta":    "\u0394",
	"Epsilon":  "\u0395",
	"Zeta":     "\u0396",
	"Eta":      "\u0397",
	"Theta":    "\u0398",
	"Iota":     "\u0399",
	"Kappa":    "\u039A",
	"Lambda":   "\u039B",
	"Mu":       "\u039C",
	"Nu":       "\u039D",
	"Xi":       "\u039E",
	"Omicron":  "\u039F",
	"Pi":       "\u03A0",
	"Rho":      "\u03A1",
	"Sigma":    "\u03A3",
	"Tau":      "\u03A4",
	"Upsilon":  "\u03A5",
	"Phi":      "\u03A6",
	"Chi":      "\u03A7",
	"Psi":      "\u03A8",
	"Omega":    "\u03A9",
	"alpha":    "\u03B1",
	"beta":     "\u03B2",
	"gamma":    "\u03B3",
	"delta":    "\u03B4",
	"epsilon":  "\u03B5",
	"zeta":     "\u03B6",
	"eta":      "\u03B7",
	"theta":    "\u03B8",
	"iota":     "\u03B9",
	"kappa":    "\u03BA",
	"lambda":   "\u03BB",
	"mu":       "\u03BC",
	"nu":       "\u03BD",
	"xi":       "\u03BE",
	"omicron":  "\u03BF",
	"pi":       "\u03C0",
	"rho":      "\u03C1",
	"sigmaf":   "\u03C2",
	"sigma":    "\u03C3",
	"tau":      "\u03C4",
	"upsilon":  "\u03C5",
	"phi":      "\u03C6",
	"chi":      "\u03C7",
	"psi":      "\u03C8",
	"omega":    "\u03C9",
	"thetasym": "\u03D1",
	"upsih":    "\u03D2",
	"piv":      "\u03D6",
	"bull":     "\u2022",
	"hellip":   "\u2026",
	"prime":    "\u2032",
	"Prime":    "\u2033",
	"oline":    "\u203E",
	"frasl":    "\u2044",
	"weierp":   "\u2118",
	"image":    "\u2111",
	"real":     "\u211C",
	"trade":    "\u2122",
	"alefsym":  "\u2135",
	"larr":     "\u2190",
	"uarr":     "\u2191",
	"rarr":     "\u2192",
	"darr":     "\u2193",
	"harr":     "\u2194",
	"crarr":    "\u21B5",
	"lArr":     "\u21D0",
	"uArr":     "\u21D1",
	"rArr":     "\u21D2",
	"dArr":     "\u21D3",
	"hArr":     "\u21D4",
	"forall":   "\u2200",
	"part":     "\u2202",
	"exist":    "\u2203",
	"empty":    "\u2205",
	"nabla":    "\u2207",
	"isin":     "\u2208",
	"notin":    "\u2209",
	"ni":       "\u220B",
	"prod":     "\u220F",
	"sum":      "\u2211",
	"minus":    "\u2212",
	"lowast":   "\u2217",
	"radic":    "\u221A",
	"prop":     "\u221D",
	"infin":    "\u221E",
	"ang":      "\u2220",
	"and":      "\u2227",
	"or":       "\u2228",
	"cap":      "\u2229",
	"cup":      "\u222A",
	"int":      "\u222B",
	"there4":   "\u2234",
	"sim":      "\u223C",
	"cong":     "\u2245",
	"asymp":    "\u2248",
	"ne":       "\u2260",
	"equiv":    "\u2261",
	"le":       "\u2264",
	"ge":       "\u2265",
	"sub":      "\u2282",
	"sup":      "\u2283",
	"nsub":     "\u2284",
	"sube":     "\u2286",
	"supe":     "\u2287",
	"oplus":    "\u2295",
	"otimes":   "\u2297",
	"perp":     "\u22A5",
	"sdot":     "\u22C5",
	"lceil":    "\u2308",
	"rceil":    "\u2309",
	"lfloor":   "\u230A",
	"rfloor":   "\u230B",
	"lang":     "\u2329",
	"rang":     "\u232A",
	"loz":      "\u25CA",
	"spades":   "\u2660",
	"clubs":    "\u2663",
	"hearts":   "\u2665",
	"diams":    "\u2666",
	"quot":     "\u0022",
	"amp":      "\u0026",
	"lt":       "\u003C",
	"gt":       "\u003E",
	"OElig":    "\u0152",
	"oelig":    "\u0153",
	"Scaron":   "\u0160",
	"scaron":   "\u0161",
	"Yuml":     "\u0178",
	"circ":     "\u02C6",
	"tilde":    "\u02DC",
	"ensp":     "\u2002",
	"emsp":     "\u2003",
	"thinsp":   "\u2009",
	"zwnj":     "\u200C",
	"zwj":      "\u200D",
	"lrm":      "\u200E",
	"rlm":      "\u200F",
	"ndash":    "\u2013",
	"mdash":    "\u2014",
	"lsquo":    "\u2018",
	"rsquo":    "\u2019",
	"sbquo":    "\u201A",
	"ldquo":    "\u201C",
	"rdquo":    "\u201D",
	"bdquo":    "\u201E",
	"dagger":   "\u2020",
	"Dagger":   "\u2021",
	"permil":   "\u2030",
	"lsaquo":   "\u2039",
	"rsaquo":   "\u203A",
	"euro":     "\u20AC",
}

// HTMLAutoClose is the set of HTML elements that
// should be considered to close automatically.
//
// See the Decoder.Strict and Decoder.Entity fields' documentation.
var HTMLAutoClose []string = htmlAutoClose

var htmlAutoClose = []string{
	/*
		hget http://www.w3.org/TR/html4/loose.dtd |
		9 sed -n 's/<!ELEMENT ([^ ]*) +- O EMPTY.+/	"\1",/p' | tr A-Z a-z
	*/
	"basefont",
	"br",
	"area",
	"link",
	"img",
	"param",
	"hr",
	"input",
	"col",
	"frame",
	"isindex",
	"base",
	"meta",
}

var (
	escQuot = []byte("&#34;") // shorter than "&quot;"
	escApos = []byte("&#39;") // shorter than "&apos;"
	escAmp  = []byte("&amp;")
	escLT   = []byte("&lt;")
	escGT   = []byte("&gt;")
	escTab  = []byte("&#x9;")
	escNL   = []byte("&#xA;")
	escCR   = []byte("&#xD;")
	escFFFD = []byte("\uFFFD") // Unicode replacement character
)

var (
	cdataStart  = []byte("<![CDATA[")
	cdataEnd    = []byte("]]>")
	cdataEscape = []byte("]]]]><![CDATA[>")
)

// procInst parses the `param="..."` or `param='...'`
// value out of the provided string, returning "" if not found.
func procInst(param, s string) string {
	// TODO: this parsing is somewhat lame and not exact.
	// It works for all actual cases, though.
	param = param + "="
	idx := strings.Index(s, param)
	if idx == -1 {
		return ""
	}
	v := s[idx+len(param):]
	if v == "" {
		return ""
	}
	if v[0] != '\'' && v[0] != '"' {
		return ""
	}
	idx = strings.IndexRune(v[1:], rune(v[0]))
	if idx == -1 {
		return ""
	}
	return v[1 : idx+1]
}
