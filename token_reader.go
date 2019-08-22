package xmlutils

import(
	"encoding/xml"
)

type tokenReader struct {
	start *xml.StartElement
	r xml.TokenReader
}

func (t *tokenReader) Token() (xml.Token, error) {
	if t.start != nil {
		token := t.start
		t.start = nil
		return *token, nil
	}
	return t.r.Token()
}

// NewTokenReader добавляем xml.StartElement в набор xml.TokenReader
func NewTokenReader(
	r xml.TokenReader,
	start xml.StartElement,
) (
	xml.TokenReader,
) {
	return &tokenReader{
		start: &start,
		r:     r,
	}
}

// Используется для вызова UnmarshalXML(*xml.Decoder, xml.StartElement) c текущий Decoder
type unmarshalerWrapper struct{
	data xml.Unmarshaler
}

func (w *unmarshalerWrapper) UnmarshalXML(d *Decoder, start xml.StartElement) error {
	r := &tokenReader{
		start: &start,
		r:     d,
	}
	decoder := xml.NewTokenDecoder(r)
	_, err := decoder.Token()
	if err != nil {
		return err
	}
	return w.data.UnmarshalXML(decoder, start)
}
