# XML utils

[![Build Status](https://travis-ci.org/mantyr/xmlutils.svg?branch=master)](https://travis-ci.org/mantyr/xmlutils)
[![GoDoc](https://godoc.org/github.com/mantyr/xmlutils?status.png)](http://godoc.org/github.com/mantyr/xmlutils)
[![Go Report Card](https://goreportcard.com/badge/github.com/mantyr/xmlutils?v=4)][goreport]
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE.md)

This don't stable version

## Особенности реализации

- [x] Игнорировать prefix в xml tag `xml:"prefix:name"` воспринимается в Unmarshal как `xml:"name"`

   Это нужно для одинакового поведения Unmarshal/Marshal с префиксными тегами

- [x] Добавить поддержку xml.Unmarshaler

   Теперь нет необходимости править все реализации xml.Unmarshaler на xmlutils.Unmarshaler

- [x] Добавить проброс xmlutils.Decoder в xml.Unmarshaler

   Теперь в не зависимости от того как был запущен Unmarshal (от xml или от xmlutils) у вас есть возможность использовать xmlutils там где это нужно

- [ ] Вынести все имеющиеся структуры из форка обратно в encoding/xml

   Сейчас это реализовано для большинства структур (такие как xml.StartElement, xml.Name, xml.Attr, xml.Token и прочие), но не исключаю что ещё есть что можно перенести

### Проброс xmlutils.Decoder через интерфейс xml.UnmarshalXML
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
