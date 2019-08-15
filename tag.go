package xmlutils

import (
	"strings"
	"errors"
)

type XMLTag struct {
	// Namespace это xml.Name.Space
	Namespace string

	// Tags это header:from>header:data>header:id
	Tags []Tag

	// Flags это:
	// ,omitempty
	// ,innerxml
	// ,chardata
	// ,attr
	Flags []string
}

type Tag struct {
	Prefix string
	Value string
}

func (t Tag) String() string {
	if t.Prefix == "" {
		return t.Value
	}
	return t.Prefix+":"+t.Value
}

func (t *XMLTag) DeleteNSPrefix() {
	for n, tag := range t.Tags {
		tag := tag
		tag.Prefix = ""
		t.Tags[n] = tag
	}
}

func (t *XMLTag) String() string {
	var s string
	if t.Namespace != "" {
		s = t.Namespace+" "
	}
	tags := make([]string, len(t.Tags))
	for n, tag := range t.Tags {
		tags[n] = tag.String()
	}
	s = s+strings.Join(tags, ">")
	if len(t.Flags) > 0 {
		s = s+","+strings.Join(t.Flags, ",")
	}
	return s
}

func ParseXMLTag(s string) (*XMLTag, error) {
	var err error
	t := &XMLTag{}

	if i := strings.Index(s, " "); i >= 0 {
		t.Namespace, s = s[:i], s[i+1:]
	}
	tokens := strings.Split(s, ",")
	switch len(tokens) {
	case 0:
		return nil, errors.New("empty tag")
	case 1:
		t.Tags, err = ParseTag(s)
		if err != nil {
			return nil, err
		}
	default:
		t.Tags, err = ParseTag(tokens[0])
		if err != nil {
			return nil, err
		}
		t.Flags = tokens[1:]
	}
	return t, nil
}

func ParseTag(s string) ([]Tag, error) {
	tokens := strings.Split(s, ">")
	t := make([]Tag, len(tokens))
	for n, token := range tokens {
		tag := strings.Split(token, ":")
		switch len(tag) {
		case 0:
			return t, errors.New("empty tag section")
		case 1:
			t[n] = Tag{
				Value: token,
			}
		case 2:
			t[n] = Tag{
				Prefix: tag[0],
				Value:  tag[1],
			}
		default:
			return t, errors.New("many tag prefix")
		}
	}
	return t, nil
}

// DeleteNSPrefix удаляет префикс xml тега, оставляя все остальные элементы тега
func DeleteNSPrefix(tag string) (string, error) {
	t, err := ParseXMLTag(tag)
	if err != nil {
		return "", err
	}
	t.DeleteNSPrefix()
	return t.String(), nil
}

