// Package render turns slices of record structs into one of the output formats
// dlmf-cli supports: table, json, jsonl, csv, tsv, url, and raw. It works
// off struct reflection and json tags, so any record type renders without
// per-type code.
package render

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"
	"text/template"
)

// Format is an output rendering format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatJSONL Format = "jsonl"
	FormatCSV   Format = "csv"
	FormatTSV   Format = "tsv"
	FormatURL   Format = "url"
	FormatRaw   Format = "raw"
)

// Valid reports whether f is one of the supported formats.
func (f Format) Valid() bool {
	switch f {
	case FormatTable, FormatJSON, FormatJSONL, FormatCSV, FormatTSV, FormatURL, FormatRaw:
		return true
	}
	return false
}

// Renderer writes records in a chosen format.
type Renderer struct {
	Format   Format
	Fields   []string
	NoHeader bool
	Template string
	w        io.Writer
}

// New builds a Renderer writing to w.
func New(w io.Writer, format Format, fields []string, noHeader bool, tmpl string) *Renderer {
	return &Renderer{Format: format, Fields: fields, NoHeader: noHeader, Template: tmpl, w: w}
}

// Render writes records (a slice of structs, or a single struct) in the configured format.
func (r *Renderer) Render(records any) error {
	rv := reflect.ValueOf(records)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		s := reflect.MakeSlice(reflect.SliceOf(rv.Type()), 1, 1)
		s.Index(0).Set(rv)
		rv = s
	}
	n := rv.Len()
	items := make([]any, n)
	for i := 0; i < n; i++ {
		items[i] = rv.Index(i).Interface()
	}

	if r.Template != "" {
		return r.renderTemplate(items)
	}
	switch r.Format {
	case FormatJSON:
		return r.renderJSON(items)
	case FormatJSONL:
		return r.renderJSONL(items)
	case FormatCSV:
		return r.renderDelimited(items, ',', false)
	case FormatTSV:
		return r.renderDelimited(items, '\t', false)
	case FormatTable:
		return r.renderTable(items)
	case FormatURL:
		return r.renderURL(items)
	case FormatRaw:
		return r.renderRaw(items)
	default:
		return r.renderJSONL(items)
	}
}

func (r *Renderer) renderJSON(items []any) error {
	enc := json.NewEncoder(r.w)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

func (r *Renderer) renderJSONL(items []any) error {
	enc := json.NewEncoder(r.w)
	for _, item := range items {
		if err := enc.Encode(item); err != nil {
			return err
		}
	}
	return nil
}

func (r *Renderer) renderTemplate(items []any) error {
	t, err := template.New("").Parse(r.Template)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	for _, item := range items {
		if err := t.Execute(r.w, item); err != nil {
			return err
		}
		fmt.Fprintln(r.w)
	}
	return nil
}

func (r *Renderer) renderDelimited(items []any, sep rune, isTable bool) error {
	if len(items) == 0 {
		return nil
	}
	cols := r.columns(items[0])

	var w *csv.Writer
	if isTable {
		w = csv.NewWriter(r.w)
	} else {
		w = csv.NewWriter(r.w)
	}
	w.Comma = sep

	if !r.NoHeader {
		if err := w.Write(cols); err != nil {
			return err
		}
	}
	for _, item := range items {
		row := r.row(item, cols)
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func (r *Renderer) renderTable(items []any) error {
	if len(items) == 0 {
		return nil
	}
	cols := r.columns(items[0])
	tw := tabwriter.NewWriter(r.w, 0, 0, 2, ' ', 0)
	if !r.NoHeader {
		fmt.Fprintln(tw, strings.Join(cols, "\t"))
	}
	for _, item := range items {
		row := r.row(item, cols)
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}

func (r *Renderer) renderURL(items []any) error {
	for _, item := range items {
		rv := reflect.ValueOf(item)
		if rv.Kind() == reflect.Pointer {
			rv = rv.Elem()
		}
		rt := rv.Type()
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			tag := f.Tag.Get("json")
			name := strings.Split(tag, ",")[0]
			if name == "url" || strings.EqualFold(f.Name, "url") {
				fmt.Fprintln(r.w, rv.Field(i).String())
				break
			}
		}
	}
	return nil
}

func (r *Renderer) renderRaw(items []any) error {
	for _, item := range items {
		rv := reflect.ValueOf(item)
		if rv.Kind() == reflect.Pointer {
			rv = rv.Elem()
		}
		rt := rv.Type()
		for i := 0; i < rt.NumField(); i++ {
			fmt.Fprintf(r.w, "%s\t%v\n", rt.Field(i).Name, rv.Field(i).Interface())
		}
		fmt.Fprintln(r.w)
	}
	return nil
}

// columns returns the field names (from json tags or field names) to render,
// filtered by r.Fields if set.
func (r *Renderer) columns(item any) []string {
	rv := reflect.ValueOf(item)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	rt := rv.Type()

	var all []string
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := f.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			name = f.Name
		}
		all = append(all, name)
	}

	if len(r.Fields) == 0 {
		return all
	}
	// filter to requested fields, preserving request order
	set := make(map[string]bool, len(all))
	for _, c := range all {
		set[c] = true
	}
	var filtered []string
	for _, f := range r.Fields {
		if set[f] {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// row returns string values for the given columns from item.
func (r *Renderer) row(item any, cols []string) []string {
	rv := reflect.ValueOf(item)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	rt := rv.Type()

	// build a map from json-tag-name -> field value
	fieldMap := make(map[string]string, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := f.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			name = f.Name
		}
		fieldMap[name] = fmt.Sprintf("%v", rv.Field(i).Interface())
	}

	row := make([]string, len(cols))
	for i, c := range cols {
		row[i] = fieldMap[c]
	}
	return row
}
