package xmldecoder

import (
	"encoding/xml"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type table2 struct {
	XMLName xml.Name
	XMLNS string `xml:"xmlns:st,attr"`
	FIO fio `xml:"header:fio"`
}

type fio struct {
	value string
}

func (f *fio) UnmarshalXML(d *Decoder, start xml.StartElement) error {
	var data string
	err := d.DecodeElement(&data, &start)
	if err != nil {
		return err
	}
	f.value = data
	return nil
}

func TestEmptyUnmarshaler(t *testing.T) {
	Convey("Проверяем Unmarshal с пустым значением", t, func() {
		data := `<st:table xmlns:st="http://localhost"><header:fio>test</header:fio></st:table>`
		v := &table2{}
		err := Unmarshal([]byte(data), v)
		So(err, ShouldBeNil)
		So(
			v,
			ShouldResemble,
			&table2{
				XMLName: xml.Name{
					Space:"http://localhost",
					Local:"table",
				},
				XMLNS: "http://localhost",
				FIO: fio{
					value: "test",
				},
			},
		)
	})
}
