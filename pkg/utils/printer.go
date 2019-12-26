package utils

import (
"bytes"
"fmt"
"io"
"reflect"
"text/tabwriter"
)

type Printer struct {
	out io.Writer
}

func New(out io.Writer) *Printer {
	return &Printer{
		out: out,
	}
}

func (p *Printer) Write(level int, format string, a ...interface{}) {
	levelSpace := "  "
	prefix := ""
	for i := 0; i < level; i++ {
		prefix += levelSpace
	}
	var aa []interface{}
	for _, v := range a {
		var ss string
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
			if rv.IsNil() {
				ss = "<none>"
			} else {
				rv = rv.Elem()
				v = rv.Interface()
				ss = fmt.Sprint(v)
			}
		} else {
			ss = fmt.Sprint(v)
		}
		if ss == "" {
			ss = "<none>"
		}
		aa = append(aa, ss)
	}
	fmt.Fprintf(p.out, prefix+format, aa...)
}

func TabbedString(f func(io.Writer) error) (string, error) {
	out := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	out.Init(buf, 0, 8, 2, ' ', 0)

	err := f(out)
	if err != nil {
		return "", err
	}

	out.Flush()
	str := string(buf.String())
	return str, nil
}

