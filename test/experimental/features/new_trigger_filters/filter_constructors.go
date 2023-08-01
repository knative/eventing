package new_trigger_filters

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type Filter interface {
	FilterString(indents int) string
}

type AttributeFilter struct {
	Type      string
	Attribute string
	Value     string
}

func (f *AttributeFilter) FilterString(indents int) string {
	return indent(fmt.Sprintf(`- %s:
	%s: %s`, f.Type, f.Attribute, f.Value), indents)
}

type CESQLFilter struct {
	Value string
}

func (f *CESQLFilter) FilterString(indents int) string {
	return indent(fmt.Sprintf(" - cesql: %s", f.Value), indents)
}

type NotFilter struct {
	Filter Filter
}

func (f *NotFilter) FilterString(indents int) string {
	tmpl, err := template.New("not-filter").Parse("- not:\n{{ .Filter.FilterString 1 }}")
	if err != nil {
		panic("failed to parse filter string")
	}
	var result bytes.Buffer
	err = tmpl.Execute(&result, *f)
	if err != nil {
		panic("failed to execute template to filter")
	}
	return indent(result.String(), indents)
}

type ArrayFilter struct {
	Type    string
	Filters []Filter
}

func (f *ArrayFilter) FilterString(indents int) string {
	var helpers template.FuncMap = map[string]interface{}{
		"last": func(index int, len int) bool {
			return index == len-1
		},
	}
	tmpl, err := template.New("filter").Funcs(helpers).Parse("- {{.Type}}:\n{{$filtersLength := len .Filters}}{{range $idx, $filter := .Filters}}{{$filter.FilterString 1}}{{if not (last $idx $filtersLength)}}\n{{end}}{{end}}")
	if err != nil {
		panic("failed to parse filter string")
	}
	var result bytes.Buffer
	err = tmpl.Execute(&result, *f)
	if err != nil {
		panic("failed to execute template to filter")
	}
	return indent(result.String(), indents)
}

func indent(input string, numIndents int) string {
	tabCharacter := strings.Repeat("\t", numIndents)
	lines := strings.SplitAfter(input, "\n")
	lines[0] = fmt.Sprintf("%s%s", tabCharacter, lines[0])
	return strings.Join(lines, tabCharacter)
}
