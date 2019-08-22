package xmlutils

import (
	"encoding/xml"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type prefixA struct {
	XMLName xml.Name
	B prefixB `xml:"prefix:b"`
}

type prefixB struct {
	is bool
	d prefixD
}

type prefixD struct {
	XMLName xml.Name
	Data string `xml:"data1"`
	PrefixData string `xml:"prefix:data"`
}

func (b *prefixB) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	if b.is {
		return d.DecodeElement(&b.d, &start)
	}
	return NewTokenDecoder(d, start).Decode(&b.d)
}

func TestCustomDecoder(t *testing.T) {
	Convey("Проверяем проброс Decoder через xml.Decoder", t, func() {
		data := `<?xml version="1.0" encoding="utf-8"?>
			<MyStruct Attr="attr1" Attr2="attr2">
				<prefix:b>
					<data1>test1</data1>
					<prefix:data>test2</prefix:data>
				</prefix:b>
			</MyStruct>
		`
		Convey("Пробрасываем Decoder - теги доступны", func() {
			a := &prefixA{}
			err := Unmarshal([]byte(data), a)
			So(err, ShouldBeNil)
			So(a.B.d.XMLName, ShouldResemble, xml.Name{Space: "prefix", Local: "b"})
			So(a.B.d.Data, ShouldEqual, "test1")
			So(a.B.d.PrefixData, ShouldEqual, "test2")
		})
		Convey("Не пробрасываем Decoder - теги не доступны", func() {
			a := &prefixA{}
			a.B.is = true
			err := Unmarshal([]byte(data), a)
			So(err, ShouldBeNil)
			So(a.B.d.XMLName, ShouldResemble, xml.Name{Space: "prefix", Local: "b"})
			So(a.B.d.Data, ShouldEqual, "test1")
			So(a.B.d.PrefixData, ShouldEqual, "")
		})
	})
}
