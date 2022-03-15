# XML utils

[![Build Status](https://travis-ci.org/mantyr/xmlutils.svg?branch=master)](https://travis-ci.org/mantyr/xmlutils)
[![GoDoc](https://godoc.org/github.com/mantyr/xmlutils?status.png)](http://godoc.org/github.com/mantyr/xmlutils)
[![Go Report Card](https://goreportcard.com/badge/github.com/mantyr/xmlutils?v=4)][goreport]
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE.md)

This don't stable version

## Implementation features

- [x] Ignore prefix in xml tag

   Now `xml:"prefix:name"` is interpreted by Unmarshal as `xml:"name"`

   This is necessary for the same behavior of Unmarshal/Marshal with prefix tags

- [x] Add support for xml.Unmarshaler

   Now there is no need to change all implementations of xml.Unmarshaler to xmlutils.Unmarshaler

- [x] Add forwarding xmlutils.Decoder to xml.Unmarshaler

   Now, regardless of how Unmarshal was launched (from xml or from xmlutils), you have the opportunity to use xmlutils where necessary

- [ ] Move all existing structures from the fork back to encoding/xml

   Now this is implemented for most structures (such as xml.StartElement, xml.Name, xml.Attr, xml.Token and others), but I do not exclude that there is something else that can be transferred

### Forwarding xmlutils.Decoder via the xml.UnmarshalXML interface
```GO
package main

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

func main() {
	data := `<a><prefix:data>test</prefix:data></a>`
	a := &A{}
	err := xml.Unmarshal([]byte(data), &a)
	fmt.Println(err)
	fmt.Println(a.B.Data)
	// Output:
	// <nil>
	// test
}
```

## Installation

    $ go get -u github.com/mantyr/xmlutils

## Author

[Oleg Shevelev][mantyr]

[mantyr]: https://github.com/mantyr

[build_status]: https://travis-ci.org/mantyr/xmlutils
[godoc]:        http://godoc.org/github.com/mantyr/xmlutils
[goreport]:     https://goreportcard.com/report/github.com/mantyr/xmlutils
