package pbxproj

// InsertObject adds a new object to the objects dict with the given UUID and comment.
// It is idempotent — if the UUID already exists, it does nothing.
func InsertObject(objects *Dict, uuid, comment string, obj *Dict) {
	if objects.Has(uuid) {
		return
	}
	objects.Entries = append(objects.Entries, DictEntry{
		Key:        uuid,
		KeyComment: comment,
		Value:      obj,
	})
}

// RemoveObject removes an object by UUID from the objects dict.
// Returns true if the object was found and removed.
func RemoveObject(objects *Dict, uuid string) bool {
	return objects.Remove(uuid)
}

// EnsureArray ensures that the dict has an array at the given key.
// If it doesn't exist, an empty array is created.
// Returns the (possibly new) array.
func EnsureArray(dict *Dict, key string) *Array {
	existing := dict.GetArray(key)
	if existing != nil {
		return existing
	}
	arr := &Array{}
	dict.Set(key, arr)
	return arr
}

// AppendToArrayIfAbsent appends a string value with a comment to an array
// property on a dict, creating the array if needed.
// It is idempotent — skips if the value is already present.
func AppendToArrayIfAbsent(dict *Dict, key, value, comment string) {
	arr := EnsureArray(dict, key)
	if arr.Contains(value) {
		return
	}
	arr.Append(&String{Value: value}, comment)
}

// RemoveFromArray removes items with the given string value from an array
// property on a dict.
// Returns true if any items were removed.
func RemoveFromArray(dict *Dict, key, value string) bool {
	arr := dict.GetArray(key)
	if arr == nil {
		return false
	}
	return arr.RemoveByValue(value)
}

// RemoveEmptyEntry removes a key from the dict if its value is an empty
// array or empty dict.
func RemoveEmptyEntry(dict *Dict, key string) {
	node := dict.Get(key)
	if node == nil {
		return
	}
	switch v := node.(type) {
	case *Array:
		if len(v.Items) == 0 {
			dict.Remove(key)
		}
	case *Dict:
		if len(v.Entries) == 0 {
			dict.Remove(key)
		}
	}
}

// BuildObject creates a new Dict with the given isa and key-value string pairs.
// Keys and values are provided as alternating arguments after isa.
func BuildObject(isa string, kvPairs ...string) *Dict {
	dict := &Dict{}
	dict.SetString("isa", isa, false)
	for i := 0; i+1 < len(kvPairs); i += 2 {
		key := kvPairs[i]
		val := kvPairs[i+1]
		dict.SetString(key, val, needsQuoting(val))
	}
	return dict
}

// BuildObjectWithDict creates a new Dict with the given isa and one nested dict entry.
func BuildObjectWithDict(isa, dictKey string, nested *Dict, kvPairs ...string) *Dict {
	dict := BuildObject(isa, kvPairs...)
	dict.Set(dictKey, nested)
	return dict
}
