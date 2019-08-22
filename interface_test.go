package xmlutils

import (
	"encoding/xml"
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type MyCharData2 struct {
	body string
}

/*
    Expected tokens:
    0 - token: xml.CharData{0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x20} <nil>
    1 - token: xml.Comment{0x20, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x20} <nil>
    2 - token: xml.CharData{0x77, 0x6f, 0x72, 0x6c, 0x64} <nil>
    3 - token: xml.EndElement{Name:xml.Name{Space:"", Local:"Data"}} <nil>
    4 - token: <nil> EOF
*/
func (m *MyCharData2) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		t, err := d.Token()
		if err == io.EOF { // found end of element
			break
		}
		if err != nil {
			return err
		}
		if char, ok := t.(xml.CharData); ok {
			m.body += string(char)
		}
	}
	return nil
}

type MyAttr2 struct {
	attr string
}

func (m *MyAttr2) UnmarshalXMLAttr(attr xml.Attr) error {
	m.attr = attr.Value
	return nil
}

type MyStruct2 struct {
	Data *MyCharData2
	Attr *MyAttr2 `xml:",attr"`

	Data2 MyCharData2
	Attr2 MyAttr2 `xml:",attr"`
}

func TestDecoderInterface(t *testing.T) {
	Convey("Проверяем UnmarshalXML(d *xml.Decoder, start xml.StartElement)", t, func() {
		xml := `<?xml version="1.0" encoding="utf-8"?>
			<MyStruct Attr="attr1" Attr2="attr2">
			<Data>hello <!-- comment -->world</Data>
			<Data2>howdy <!-- comment -->world</Data2>
			</MyStruct>
		`
		var m MyStruct2
		err := Unmarshal([]byte(xml), &m)
		So(err, ShouldBeNil)
		So(m.Data, ShouldNotBeNil)
		So(m.Attr, ShouldNotBeNil)
		So(m.Data.body, ShouldEqual, "hello world")
		So(m.Attr.attr, ShouldEqual, "attr1")
		So(m.Data2.body, ShouldEqual, "howdy world")
		So(m.Attr2.attr, ShouldEqual, "attr2")
	})
}
