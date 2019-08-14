package xmldecoder

import (
	"encoding/xml"
	"bytes"
	"encoding"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"
)

func Unmarshal(data []byte, v interface{}) error {
	return NewDecoder(bytes.NewReader(data)).Decode(v)
}

func (d *Decoder) Decode(v interface{}) error {
	return d.DecodeElement(v, nil)
}

func (d *Decoder) DecodeElement(v interface{}, start *xml.StartElement) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return errors.New("non-pointer passed to Unmarshal")
	}
	return d.unmarshal(val.Elem(), start)
}

func (d *Decoder) Token() (xml.Token, error) {
	if d.unmarshaler {
		token, err := d.Decoder.Token()
		if err != nil {
			return nil, err
		}
		switch token.(type) {
		case xml.CharData:
		case xml.StartElement:
			d.depth++
		case xml.EndElement:
			d.depth--
		}
		if d.depth < 1 {
			d.next = token
			return nil, io.EOF
		}
		return token, err
	} else {
		if d.next != nil {
			d.next = nil
			return d.next, nil
		}
		return d.Decoder.Token()
	}
}


type Unmarshaler interface {
	UnmarshalXML(d *Decoder, start xml.StartElement) error
}

// receiverType returns the receiver type to use in an expression like "%s.MethodName".
func receiverType(val interface{}) string {
	t := reflect.TypeOf(val)
	if t.Name() != "" {
		return t.String()
	}
	return "(" + t.String() + ")"
}

// unmarshalInterface unmarshals a single XML element into val.
// start is the opening tag of the element.
func (d *Decoder) unmarshalInterface(val Unmarshaler, start *xml.StartElement) error {
	d.depth = 1
	d.unmarshaler = true

	err := val.UnmarshalXML(d, *start)
	if err != nil {
		return err
	}
	d.unmarshaler = false
	return nil
}

/*
	1. как вариант прочитать все токены в текущем пространстве
	2. последний токен если он вышел за пределы содержания - сохраняем
	3. в Token() читаем если есть сохранённый токен - отдаём, если нет читаем новый
	
	
*/

// unmarshalTextInterface unmarshals a single XML element into val.
// The chardata contained in the element (but not its children)
// is passed to the text unmarshaler.
func (d *Decoder) unmarshalTextInterface(val encoding.TextUnmarshaler) error {
	var buf []byte
	depth := 1
	for depth > 0 {
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t := t.(type) {
		case xml.CharData:
			if depth == 1 {
				buf = append(buf, t...)
			}
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return val.UnmarshalText(buf)
}

// unmarshalAttr unmarshals a single XML attribute into val.
func (d *Decoder) unmarshalAttr(val reflect.Value, attr xml.Attr) error {
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		val = val.Elem()
	}
	if val.CanInterface() && val.Type().Implements(unmarshalerAttrType) {
		// This is an unmarshaler with a non-pointer receiver,
		// so it's likely to be incorrect, but we do what we're told.
		return val.Interface().(xml.UnmarshalerAttr).UnmarshalXMLAttr(attr)
	}
	if val.CanAddr() {
		pv := val.Addr()
		if pv.CanInterface() && pv.Type().Implements(unmarshalerAttrType) {
			return pv.Interface().(xml.UnmarshalerAttr).UnmarshalXMLAttr(attr)
		}
	}

	// Not an UnmarshalerAttr; try encoding.TextUnmarshaler.
	if val.CanInterface() && val.Type().Implements(textUnmarshalerType) {
		// This is an unmarshaler with a non-pointer receiver,
		// so it's likely to be incorrect, but we do what we're told.
		return val.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(attr.Value))
	}
	if val.CanAddr() {
		pv := val.Addr()
		if pv.CanInterface() && pv.Type().Implements(textUnmarshalerType) {
			return pv.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(attr.Value))
		}
	}

	if val.Type().Kind() == reflect.Slice && val.Type().Elem().Kind() != reflect.Uint8 {
		// Slice of element values.
		// Grow slice.
		n := val.Len()
		val.Set(reflect.Append(val, reflect.Zero(val.Type().Elem())))

		// Recur to read element into slice.
		if err := d.unmarshalAttr(val.Index(n), attr); err != nil {
			val.SetLen(n)
			return err
		}
		return nil
	}

	if val.Type() == attrType {
		val.Set(reflect.ValueOf(attr))
		return nil
	}

	return copyValue(val, []byte(attr.Value))
}

var (
	attrType            = reflect.TypeOf(xml.Attr{})
	unmarshalerType     = reflect.TypeOf((*Unmarshaler)(nil)).Elem()
	unmarshalerAttrType = reflect.TypeOf((*xml.UnmarshalerAttr)(nil)).Elem()
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

// Unmarshal a single XML element into val.
func (d *Decoder) unmarshal(val reflect.Value, start *xml.StartElement) error {
	// Find start element if we need it.
	if start == nil {
		for {
			tok, err := d.Token()
			if err != nil {
				return err
			}
			if t, ok := tok.(xml.StartElement); ok {
				start = &t
				break
			}
		}
	}

	// Load value from interface, but only if the result will be
	// usefully addressable.
	if val.Kind() == reflect.Interface && !val.IsNil() {
		e := val.Elem()
		if e.Kind() == reflect.Ptr && !e.IsNil() {
			val = e
		}
	}

	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		val = val.Elem()
	}

	if val.CanInterface() && val.Type().Implements(unmarshalerType) {
		// This is an unmarshaler with a non-pointer receiver,
		// so it's likely to be incorrect, but we do what we're told.
		return d.unmarshalInterface(val.Interface().(Unmarshaler), start)
	}

	if val.CanAddr() {
		pv := val.Addr()
		if pv.CanInterface() && pv.Type().Implements(unmarshalerType) {
			return d.unmarshalInterface(pv.Interface().(Unmarshaler), start)
		}
	}

	if val.CanInterface() && val.Type().Implements(textUnmarshalerType) {
		return d.unmarshalTextInterface(val.Interface().(encoding.TextUnmarshaler))
	}

	if val.CanAddr() {
		pv := val.Addr()
		if pv.CanInterface() && pv.Type().Implements(textUnmarshalerType) {
			return d.unmarshalTextInterface(pv.Interface().(encoding.TextUnmarshaler))
		}
	}

	var (
		data         []byte
		saveData     reflect.Value
		comment      []byte
		saveComment  reflect.Value
		saveXML      reflect.Value
		saveXMLIndex int
		saveXMLData  []byte
		saveAny      reflect.Value
		sv           reflect.Value
		tinfo        *typeInfo
		err          error
	)

	switch v := val; v.Kind() {
	default:
		return errors.New("unknown type " + v.Type().String())

	case reflect.Interface:
		// TODO: For now, simply ignore the field. In the near
		//       future we may choose to unmarshal the start
		//       element on it, if not nil.
		return d.Skip()

	case reflect.Slice:
		typ := v.Type()
		if typ.Elem().Kind() == reflect.Uint8 {
			// []byte
			saveData = v
			break
		}

		// Slice of element values.
		// Grow slice.
		n := v.Len()
		v.Set(reflect.Append(val, reflect.Zero(v.Type().Elem())))

		// Recur to read element into slice.
		if err := d.unmarshal(v.Index(n), start); err != nil {
			v.SetLen(n)
			return err
		}
		return nil

	case reflect.Bool, reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.String:
		saveData = v

	case reflect.Struct:
		typ := v.Type()
		if typ == nameType {
			v.Set(reflect.ValueOf(start.Name))
			break
		}

		sv = v
		tinfo, err = getTypeInfo(typ)
		if err != nil {
			return err
		}

		// Validate and assign element name.
		if tinfo.xmlname != nil {
			finfo := tinfo.xmlname
			if finfo.name != "" && finfo.name != start.Name.Local {
				return xml.UnmarshalError("expected element type <" + finfo.name + "> but have <" + start.Name.Local + ">")
			}
			if finfo.xmlns != "" && finfo.xmlns != start.Name.Space {
				e := "expected element <" + finfo.name + "> in name space " + finfo.xmlns + " but have "
				if start.Name.Space == "" {
					e += "no name space"
				} else {
					e += start.Name.Space
				}
				return xml.UnmarshalError(e)
			}
			fv := finfo.value(sv)
			if _, ok := fv.Interface().(xml.Name); ok {
				fv.Set(reflect.ValueOf(start.Name))
			}
		}

		// Assign attributes.
		for _, a := range start.Attr {
			handled := false
			any := -1
			for i := range tinfo.fields {
				finfo := &tinfo.fields[i]
				switch finfo.flags & fMode {
				case fAttr:
					strv := finfo.value(sv)
					if a.Name.Local == finfo.name && (finfo.xmlns == "" || finfo.xmlns == a.Name.Space) {
						if err := d.unmarshalAttr(strv, a); err != nil {
							return err
						}
						handled = true
					}

				case fAny | fAttr:
					if any == -1 {
						any = i
					}
				}
			}
			if !handled && any >= 0 {
				finfo := &tinfo.fields[any]
				strv := finfo.value(sv)
				if err := d.unmarshalAttr(strv, a); err != nil {
					return err
				}
			}
		}

		// Determine whether we need to save character data or comments.
		for i := range tinfo.fields {
			finfo := &tinfo.fields[i]
			switch finfo.flags & fMode {
			case fCDATA, fCharData:
				if !saveData.IsValid() {
					saveData = finfo.value(sv)
				}

			case fComment:
				if !saveComment.IsValid() {
					saveComment = finfo.value(sv)
				}

			case fAny, fAny | fElement:
				if !saveAny.IsValid() {
					saveAny = finfo.value(sv)
				}

			case fInnerXml:
				if !saveXML.IsValid() {
					saveXML = finfo.value(sv)
					if d.saved == nil {
						saveXMLIndex = 0
						d.saved = new(bytes.Buffer)
					} else {
						saveXMLIndex = d.savedOffset()
					}
				}
			}
		}
	}

	// Find end element.
	// Process sub-elements along the way.
Loop:
	for {
		var savedOffset int
		if saveXML.IsValid() {
			savedOffset = d.savedOffset()
		}
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			consumed := false
			if sv.IsValid() {
				consumed, err = d.unmarshalPath(tinfo, sv, nil, &t)
				if err != nil {
					return err
				}
				if !consumed && saveAny.IsValid() {
					consumed = true
					if err := d.unmarshal(saveAny, &t); err != nil {
						return err
					}
				}
			}
			if !consumed {
				if err := d.Skip(); err != nil {
					return err
				}
			}

		case xml.EndElement:
			if saveXML.IsValid() {
				saveXMLData = d.saved.Bytes()[saveXMLIndex:savedOffset]
				if saveXMLIndex == 0 {
					d.saved = nil
				}
			}
			break Loop

		case xml.CharData:
			if saveData.IsValid() {
				data = append(data, t...)
			}

		case xml.Comment:
			if saveComment.IsValid() {
				comment = append(comment, t...)
			}
		}
	}

	if saveData.IsValid() && saveData.CanInterface() && saveData.Type().Implements(textUnmarshalerType) {
		if err := saveData.Interface().(encoding.TextUnmarshaler).UnmarshalText(data); err != nil {
			return err
		}
		saveData = reflect.Value{}
	}

	if saveData.IsValid() && saveData.CanAddr() {
		pv := saveData.Addr()
		if pv.CanInterface() && pv.Type().Implements(textUnmarshalerType) {
			if err := pv.Interface().(encoding.TextUnmarshaler).UnmarshalText(data); err != nil {
				return err
			}
			saveData = reflect.Value{}
		}
	}

	if err := copyValue(saveData, data); err != nil {
		return err
	}

	switch t := saveComment; t.Kind() {
	case reflect.String:
		t.SetString(string(comment))
	case reflect.Slice:
		t.Set(reflect.ValueOf(comment))
	}

	switch t := saveXML; t.Kind() {
	case reflect.String:
		t.SetString(string(saveXMLData))
	case reflect.Slice:
		if t.Type().Elem().Kind() == reflect.Uint8 {
			t.Set(reflect.ValueOf(saveXMLData))
		}
	}

	return nil
}

func copyValue(dst reflect.Value, src []byte) (err error) {
	dst0 := dst

	if dst.Kind() == reflect.Ptr {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		dst = dst.Elem()
	}

	// Save accumulated data.
	switch dst.Kind() {
	case reflect.Invalid:
		// Probably a comment.
	default:
		return errors.New("cannot unmarshal into " + dst0.Type().String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if len(src) == 0 {
			dst.SetInt(0)
			return nil
		}
		itmp, err := strconv.ParseInt(strings.TrimSpace(string(src)), 10, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetInt(itmp)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if len(src) == 0 {
			dst.SetUint(0)
			return nil
		}
		utmp, err := strconv.ParseUint(strings.TrimSpace(string(src)), 10, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetUint(utmp)
	case reflect.Float32, reflect.Float64:
		if len(src) == 0 {
			dst.SetFloat(0)
			return nil
		}
		ftmp, err := strconv.ParseFloat(strings.TrimSpace(string(src)), dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetFloat(ftmp)
	case reflect.Bool:
		if len(src) == 0 {
			dst.SetBool(false)
			return nil
		}
		value, err := strconv.ParseBool(strings.TrimSpace(string(src)))
		if err != nil {
			return err
		}
		dst.SetBool(value)
	case reflect.String:
		dst.SetString(string(src))
	case reflect.Slice:
		if len(src) == 0 {
			// non-nil to flag presence
			src = []byte{}
		}
		dst.SetBytes(src)
	}
	return nil
}

// unmarshalPath walks down an XML structure looking for wanted
// paths, and calls unmarshal on them.
// The consumed result tells whether XML elements have been consumed
// from the Decoder until start's matching end element, or if it's
// still untouched because start is uninteresting for sv's fields.
func (d *Decoder) unmarshalPath(tinfo *typeInfo, sv reflect.Value, parents []string, start *xml.StartElement) (consumed bool, err error) {
	recurse := false
Loop:
	for i := range tinfo.fields {
		finfo := &tinfo.fields[i]
		if finfo.flags&fElement == 0 || len(finfo.parents) < len(parents) || finfo.xmlns != "" && finfo.xmlns != start.Name.Space {
			continue
		}
		for j := range parents {
			if parents[j] != finfo.parents[j] {
				continue Loop
			}
		}
		if len(finfo.parents) == len(parents) && finfo.name == start.Name.Local {
			// It's a perfect match, unmarshal the field.
			return true, d.unmarshal(finfo.value(sv), start)
		}
		if len(finfo.parents) > len(parents) && finfo.parents[len(parents)] == start.Name.Local {
			// It's a prefix for the field. Break and recurse
			// since it's not ok for one field path to be itself
			// the prefix for another field path.
			recurse = true

			// We can reuse the same slice as long as we
			// don't try to append to it.
			parents = finfo.parents[:len(parents)+1]
			break
		}
	}
	if !recurse {
		// We have no business with this element.
		return false, nil
	}
	// The element is not a perfect match for any field, but one
	// or more fields have the path to this element as a parent
	// prefix. Recurse and attempt to match these.
	for {
		var tok xml.Token
		tok, err = d.Token()
		if err != nil {
			return true, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			consumed2, err := d.unmarshalPath(tinfo, sv, parents, &t)
			if err != nil {
				return true, err
			}
			if !consumed2 {
				if err := d.Skip(); err != nil {
					return true, err
				}
			}
		case xml.EndElement:
			return true, nil
		}
	}
}

// Skip reads tokens until it has consumed the end element
// matching the most recent start element already consumed.
// It recurs if it encounters a start element, so it can be used to
// skip nested structures.
// It returns nil if it finds an end element matching the start
// element; otherwise it returns an error describing the problem.
func (d *Decoder) Skip() error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}
