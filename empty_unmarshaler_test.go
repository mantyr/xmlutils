package xml_test

import (
	"fmt"
	xml "github.com/mantyr/xmldecoder3"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type table2 struct {
	XMLName xml.Name
	XMLNS   string `xml:"xmlns:st,attr"`
	FIO     fio    `xml:"fio"`
}

type fio struct {
	value string
}

func (f *fio) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var data string
	err := d.DecodeElement(&data, &start)
	if err != nil {
		return fmt.Errorf("err: %v", err)
	}
	f.value = data
	return nil
}

func TestEmptyUnmarshaler(t *testing.T) {
	Convey("Проверяем Unmarshal", t, func() {
		Convey("С пустым значением", func() {
			data := `<st:table xmlns:st="http://localhost"><header:fio></header:fio></st:table>`
			v := &table2{}
			v.FIO.value = "test"
			err := xml.Unmarshal([]byte(data), v)
			So(err, ShouldBeNil)
			So(
				v,
				ShouldResemble,
				&table2{
					XMLName: xml.Name{
						Space: "http://localhost",
						Local: "table",
					},
					XMLNS: "http://localhost",
					FIO: fio{
						value: "",
					},
				},
			)
		})
		Convey("Со значением", func() {
			data := `<st:table xmlns:st="http://localhost"><header:fio>test</header:fio></st:table>`
			v := &table2{}
			err := xml.Unmarshal([]byte(data), v)
			So(err, ShouldBeNil)
			So(
				v,
				ShouldResemble,
				&table2{
					XMLName: xml.Name{
						Space: "http://localhost",
						Local: "table",
					},
					XMLNS: "http://localhost",
					FIO: fio{
						value: "test",
					},
				},
			)
		})
	})
}
