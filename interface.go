package xmlutils

import(
	"encoding/xml"
)

type XMLEncoder interface{
	Indent(prefix, indent string)

	Encode(v interface{}) error

	EncodeElement(
		v interface{},
		start xml.StartElement,
	) error

	EncodeToken(t xml.Token) error

	Flush() error
}

type XMLDecoder interface{
	Decode(v interface{}) error

	DecodeElement(
		v interface{},
		start *xml.StartElement,
	) error

	Token() (xml.Token, error)

	RawToken() (xml.Token, error)

	Skip() error
}
