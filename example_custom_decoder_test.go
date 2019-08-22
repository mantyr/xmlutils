package xmlutils_test

import (
	"fmt"

	"encoding/xml"
	"github.com/mantyr/xmlutils"
)

type A struct {
	B struct {
		Data string `xml:"prefix:data"`
	}
}

func (a *A) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return xmlutils.NewTokenDecoder(d, start).Decode(&a.B)
}

func Example_customDecoderAndPrefixTag() {
	data := `<a><prefix:data>test</prefix:data></a>`
	a := &A{}
	err := xml.Unmarshal([]byte(data), &a)
	fmt.Println(err)
	fmt.Println(a.B.Data)
	// Output:
	// <nil>
	// test
}
