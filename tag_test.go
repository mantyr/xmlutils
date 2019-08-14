package xmldecoder

import (
	"encoding/xml"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type table struct {
	XMLName xml.Name
	XMLNS string `xml:"xmlns:st,attr"`
	Item string `xml:"st:item"`
	Other string `xml:"header:from>header:data>header:id,omitempty"`
}

func TestTag(t *testing.T) {
	Convey("Проверяем Unmarshal с prefix:tag", t, func() {
		data := `<st:table xmlns:st="http://localhost"><st:item>test</st:item><header:from><header:data><header:id>from-data-id</header:id></header:data></header:from></st:table>`
		v := &table{}
		err := Unmarshal([]byte(data), v)
		So(err, ShouldBeNil)
		So(
			v,
			ShouldResemble,
			&table{
				XMLName: xml.Name{
					Space:"http://localhost",
					Local:"table",
				},
				XMLNS: "http://localhost",
				Item: "test",
				Other: "from-data-id",
			},
		)
		v.XMLName = xml.Name{
			Space: "",
			Local: "st:table",
		}
		v.XMLNS = "http://localhost"
		result, err := xml.Marshal(v)
		So(err, ShouldBeNil)
		So(
			string(result),
			ShouldEqual,
			data,
		)
	})
}
