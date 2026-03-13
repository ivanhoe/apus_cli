package pbxproj

// Node is the interface satisfied by all plist AST values.
type Node interface {
	nodeTag() nodeTag
}

type nodeTag int

const (
	nodeDict nodeTag = iota
	nodeArray
	nodeString
	nodeData
)

// Dict represents an ordered { key = value; ... } dictionary.
// Insertion order is preserved for deterministic serialization.
type Dict struct {
	Entries []DictEntry
}

func (*Dict) nodeTag() nodeTag { return nodeDict }

// DictEntry is a single key-value pair inside a Dict.
type DictEntry struct {
	Key          string
	KeyComment   string // inline /* comment */ following the key
	Value        Node
	ValueComment string // inline /* comment */ following the value
}

// Array represents an ordered ( item, item, ) list.
type Array struct {
	Items []ArrayItem
}

func (*Array) nodeTag() nodeTag { return nodeArray }

// ArrayItem is a single element inside an Array.
type ArrayItem struct {
	Value   Node
	Comment string // inline /* comment */ following the value
}

// String represents a scalar plist value.
type String struct {
	Value  string
	Quoted bool // true if the original source was "quoted"
}

func (*String) nodeTag() nodeTag { return nodeString }

// Data represents a <hex> plist value (rarely used in pbxproj).
type Data struct {
	Hex string
}

func (*Data) nodeTag() nodeTag { return nodeData }

// --- Dict convenience methods ---

// Get returns the value for key, or nil if not found.
func (d *Dict) Get(key string) Node {
	for i := range d.Entries {
		if d.Entries[i].Key == key {
			return d.Entries[i].Value
		}
	}
	return nil
}

// GetString returns the string value for key, or empty string.
func (d *Dict) GetString(key string) string {
	n := d.Get(key)
	if n == nil {
		return ""
	}
	if s, ok := n.(*String); ok {
		return s.Value
	}
	return ""
}

// GetDict returns the Dict value for key, or nil.
func (d *Dict) GetDict(key string) *Dict {
	n := d.Get(key)
	if n == nil {
		return nil
	}
	dd, _ := n.(*Dict)
	return dd
}

// GetArray returns the Array value for key, or nil.
func (d *Dict) GetArray(key string) *Array {
	n := d.Get(key)
	if n == nil {
		return nil
	}
	a, _ := n.(*Array)
	return a
}

// Set creates or updates a key-value entry.
func (d *Dict) Set(key string, value Node) {
	for i := range d.Entries {
		if d.Entries[i].Key == key {
			d.Entries[i].Value = value
			return
		}
	}
	d.Entries = append(d.Entries, DictEntry{Key: key, Value: value})
}

// SetString creates or updates a string entry.
func (d *Dict) SetString(key, value string, quoted bool) {
	d.Set(key, &String{Value: value, Quoted: quoted})
}

// Remove deletes the first entry matching key. Returns true if found.
func (d *Dict) Remove(key string) bool {
	for i := range d.Entries {
		if d.Entries[i].Key == key {
			d.Entries = append(d.Entries[:i], d.Entries[i+1:]...)
			return true
		}
	}
	return false
}

// Has returns true if key exists in the dict.
func (d *Dict) Has(key string) bool {
	return d.Get(key) != nil
}

// Keys returns all keys in insertion order.
func (d *Dict) Keys() []string {
	keys := make([]string, len(d.Entries))
	for i, e := range d.Entries {
		keys[i] = e.Key
	}
	return keys
}

// --- Array convenience methods ---

// Contains returns true if any item's string value equals v.
func (a *Array) Contains(v string) bool {
	for _, item := range a.Items {
		if s, ok := item.Value.(*String); ok && s.Value == v {
			return true
		}
	}
	return false
}

// Append adds an item to the array.
func (a *Array) Append(value Node, comment string) {
	a.Items = append(a.Items, ArrayItem{Value: value, Comment: comment})
}

// RemoveByValue removes the first item whose string value matches v.
// Returns true if an item was removed.
func (a *Array) RemoveByValue(v string) bool {
	for i, item := range a.Items {
		if s, ok := item.Value.(*String); ok && s.Value == v {
			a.Items = append(a.Items[:i], a.Items[i+1:]...)
			return true
		}
	}
	return false
}
