package pbxproj

import (
	"fmt"
	"sort"
	"strings"
)

// Serialize writes a parsed pbxproj AST back to the canonical Xcode format.
// Objects are grouped by isa type into sections, sorted by UUID within each section.
// Comments are regenerated dynamically to match Xcode's conventions.
func Serialize(root *Dict) string {
	var b strings.Builder
	b.WriteString("// !$*UTF8*$!\n")
	writeRootDict(&b, root)
	return b.String()
}

func writeRootDict(b *strings.Builder, root *Dict) {
	b.WriteString("{\n")
	for _, entry := range root.Entries {
		if entry.Key == "objects" {
			writeObjectsDict(b, entry)
		} else {
			writeEntry(b, entry, 1)
		}
	}
	b.WriteString("}\n")
}

func writeObjectsDict(b *strings.Builder, entry DictEntry) {
	objects, ok := entry.Value.(*Dict)
	if !ok {
		writeEntry(b, entry, 1)
		return
	}

	b.WriteString("\tobjects = {\n")

	sections := groupByISA(objects)
	sortedISAs := sortISAs(sections)

	for i, isa := range sortedISAs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(fmt.Sprintf("\n/* Begin %s section */\n", isa))

		entries := sections[isa]
		sort.Slice(entries, func(a, c int) bool {
			return entries[a].Key < entries[c].Key
		})

		for _, e := range entries {
			writeObjectEntry(b, e, objects)
		}

		b.WriteString(fmt.Sprintf("/* End %s section */\n", isa))
	}

	b.WriteString("\t};\n")
}

func writeObjectEntry(b *strings.Builder, entry DictEntry, objects *Dict) {
	comment := entry.KeyComment
	if comment == "" {
		comment = ResolveComment(objects, entry.Key)
	}

	dict, isDict := entry.Value.(*Dict)
	if !isDict {
		writeEntry(b, entry, 2)
		return
	}

	// Check if this is a single-line object (PBXBuildFile pattern)
	if isSingleLineObject(dict) {
		b.WriteString("\t\t")
		b.WriteString(entry.Key)
		if comment != "" {
			b.WriteString(" /* ")
			b.WriteString(comment)
			b.WriteString(" */")
		}
		b.WriteString(" = {")
		writeSingleLineDict(b, dict, objects)
		b.WriteString("};\n")
		return
	}

	// Multi-line object
	b.WriteString("\t\t")
	b.WriteString(entry.Key)
	if comment != "" {
		b.WriteString(" /* ")
		b.WriteString(comment)
		b.WriteString(" */")
	}
	b.WriteString(" = {\n")

	for _, sub := range dict.Entries {
		writeEntry(b, sub, 3)
	}

	b.WriteString("\t\t};\n")
}

func writeEntry(b *strings.Builder, entry DictEntry, indent int) {
	prefix := strings.Repeat("\t", indent)
	b.WriteString(prefix)
	b.WriteString(entry.Key)

	if entry.KeyComment != "" {
		b.WriteString(" /* ")
		b.WriteString(entry.KeyComment)
		b.WriteString(" */")
	}

	b.WriteString(" = ")
	writeValue(b, entry.Value, indent)
	if entry.ValueComment != "" {
		b.WriteString(" /* ")
		b.WriteString(entry.ValueComment)
		b.WriteString(" */")
	}
	b.WriteString(";\n")
}

func writeValue(b *strings.Builder, node Node, indent int) {
	switch v := node.(type) {
	case *String:
		writeString(b, v)
	case *Dict:
		writeNestedDict(b, v, indent)
	case *Array:
		writeArray(b, v, indent)
	case *Data:
		b.WriteString("<")
		b.WriteString(v.Hex)
		b.WriteString(">")
	}
}

func writeString(b *strings.Builder, s *String) {
	if s.Quoted || needsQuoting(s.Value) {
		b.WriteByte('"')
		b.WriteString(escapeString(s.Value))
		b.WriteByte('"')
	} else {
		b.WriteString(s.Value)
	}
}

func writeNestedDict(b *strings.Builder, dict *Dict, indent int) {
	if len(dict.Entries) == 0 {
		b.WriteString("{\n")
		b.WriteString(strings.Repeat("\t", indent))
		b.WriteByte('}')
		return
	}
	b.WriteString("{\n")
	for _, entry := range dict.Entries {
		writeEntry(b, entry, indent+1)
	}
	b.WriteString(strings.Repeat("\t", indent))
	b.WriteByte('}')
}

func writeArray(b *strings.Builder, arr *Array, indent int) {
	if len(arr.Items) == 0 {
		b.WriteString("(\n")
		b.WriteString(strings.Repeat("\t", indent))
		b.WriteByte(')')
		return
	}

	b.WriteString("(\n")
	for _, item := range arr.Items {
		b.WriteString(strings.Repeat("\t", indent+1))
		writeValue(b, item.Value, indent+1)
		if item.Comment != "" {
			b.WriteString(" /* ")
			b.WriteString(item.Comment)
			b.WriteString(" */")
		}
		b.WriteString(",\n")
	}
	b.WriteString(strings.Repeat("\t", indent))
	b.WriteByte(')')
}

func writeSingleLineDict(b *strings.Builder, dict *Dict, objects *Dict) {
	for _, entry := range dict.Entries {
		b.WriteString(entry.Key)
		b.WriteString(" = ")
		writeValue(b, entry.Value, 0)

		if entry.ValueComment != "" {
			b.WriteString(" /* ")
			b.WriteString(entry.ValueComment)
			b.WriteString(" */")
		}

		b.WriteString("; ")
	}
}

// --- Grouping and sorting ---

func groupByISA(objects *Dict) map[string][]DictEntry {
	groups := make(map[string][]DictEntry)
	for _, entry := range objects.Entries {
		dict, ok := entry.Value.(*Dict)
		if !ok {
			continue
		}
		isa := dict.GetString("isa")
		if isa == "" {
			isa = "_unknown"
		}
		groups[isa] = append(groups[isa], entry)
	}
	return groups
}

func sortISAs(sections map[string][]DictEntry) []string {
	isas := make([]string, 0, len(sections))
	for isa := range sections {
		isas = append(isas, isa)
	}
	sort.Slice(isas, func(i, j int) bool {
		ri, rj := rankForISA(isas[i]), rankForISA(isas[j])
		if ri != rj {
			return ri < rj
		}
		return isas[i] < isas[j]
	})
	return isas
}

// isSingleLineObject returns true for object types that Xcode serializes on one line.
// Currently only PBXBuildFile uses this pattern.
func isSingleLineObject(dict *Dict) bool {
	isa := dict.GetString("isa")
	return isa == "PBXBuildFile"
}

// --- String quoting ---

func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for i := 0; i < len(s); i++ {
		if !isUnquotedChar(s[i]) {
			return true
		}
	}
	return false
}

func escapeString(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
