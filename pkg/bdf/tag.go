package bdf

import (
	"fmt"
	"sort"
)

// Tag is the universal interface for any BDF tag
type Tag interface {
	Type() TagType
	Equal(other Tag) bool
	Clone() Tag
}

// TagByte represents an 8-bit signed integer (TAG_Byte)
type TagByte int8

func (t TagByte) Type() TagType { return TagTypeByte }
func (t TagByte) Equal(other Tag) bool {
	o, ok := other.(TagByte)
	return ok && t == o
}
func (t TagByte) Clone() Tag { return t }

// TagShort represents a 16-bit signed integer (TAG_Short)
type TagShort int16

func (t TagShort) Type() TagType { return TagTypeShort }
func (t TagShort) Equal(other Tag) bool {
	o, ok := other.(TagShort)
	return ok && t == o
}
func (t TagShort) Clone() Tag { return t }

// TagInt represents a 32-bit signed integer (TAG_Int)
type TagInt int32

func (t TagInt) Type() TagType { return TagTypeInt }
func (t TagInt) Equal(other Tag) bool {
	o, ok := other.(TagInt)
	return ok && t == o
}
func (t TagInt) Clone() Tag { return t }

// TagLong represents a 64-bit signed integer (TAG_Long)
type TagLong int64

func (t TagLong) Type() TagType { return TagTypeLong }
func (t TagLong) Equal(other Tag) bool {
	o, ok := other.(TagLong)
	return ok && t == o
}
func (t TagLong) Clone() Tag { return t }

// TagFloat represents a 32-bit IEEE 754 floating point number (TAG_Float)
type TagFloat float32

func (t TagFloat) Type() TagType { return TagTypeFloat }
func (t TagFloat) Equal(other Tag) bool {
	o, ok := other.(TagFloat)
	return ok && t == o
}
func (t TagFloat) Clone() Tag { return t }

// TagDouble represents a 64-bit IEEE 754 floating point number (TAG_Double)
type TagDouble float64

func (t TagDouble) Type() TagType { return TagTypeDouble }
func (t TagDouble) Equal(other Tag) bool {
	o, ok := other.(TagDouble)
	return ok && t == o
}
func (t TagDouble) Clone() Tag { return t }

// TagByteArray represents an array of 8-bit bytes (TAG_Byte_Array)
type TagByteArray []byte

func (t TagByteArray) Type() TagType { return TagTypeByteArray }
func (t TagByteArray) Equal(other Tag) bool {
	o, ok := other.(TagByteArray)
	if !ok || len(t) != len(o) {
		return false
	}
	for i := range t {
		if t[i] != o[i] {
			return false
		}
	}
	return true
}
func (t TagByteArray) Clone() Tag {
	cp := make([]byte, len(t))
	copy(cp, t)
	return TagByteArray(cp)
}

// TagString represents a UTF-8 string (TAG_String)
type TagString string

func (t TagString) Type() TagType { return TagTypeString }
func (t TagString) Equal(other Tag) bool {
	o, ok := other.(TagString)
	return ok && t == o
}
func (t TagString) Clone() Tag { return t }

// TagList represents a typed list of tags (TAG_List)
type TagList struct {
	ElementType TagType
	Elements    []Tag
}

func (t *TagList) Type() TagType { return TagTypeList }
func (t *TagList) Equal(other Tag) bool {
	o, ok := other.(*TagList)
	if !ok || t.ElementType != o.ElementType || len(t.Elements) != len(o.Elements) {
		return false
	}
	for i := range t.Elements {
		if !t.Elements[i].Equal(o.Elements[i]) {
			return false
		}
	}
	return true
}
func (t *TagList) Clone() Tag {
	cp := &TagList{
		ElementType: t.ElementType,
		Elements:    make([]Tag, len(t.Elements)),
	}
	for i, el := range t.Elements {
		cp.Elements[i] = el.Clone()
	}
	return cp
}

// TagIntArray represents an array of 32-bit signed integers (TAG_Int_Array)
type TagIntArray []int32

func (t TagIntArray) Type() TagType { return TagTypeIntArray }
func (t TagIntArray) Equal(other Tag) bool {
	o, ok := other.(TagIntArray)
	if !ok || len(t) != len(o) {
		return false
	}
	for i := range t {
		if t[i] != o[i] {
			return false
		}
	}
	return true
}
func (t TagIntArray) Clone() Tag {
	cp := make([]int32, len(t))
	copy(cp, t)
	return TagIntArray(cp)
}

// TagLongArray represents an array of 64-bit signed integers (TAG_Long_Array)
type TagLongArray []int64

func (t TagLongArray) Type() TagType { return TagTypeLongArray }
func (t TagLongArray) Equal(other Tag) bool {
	o, ok := other.(TagLongArray)
	if !ok || len(t) != len(o) {
		return false
	}
	for i := range t {
		if t[i] != o[i] {
			return false
		}
	}
	return true
}
func (t TagLongArray) Clone() Tag {
	cp := make([]int64, len(t))
	copy(cp, t)
	return TagLongArray(cp)
}

// CompoundEntry is a single named entry in a TagCompound
type CompoundEntry struct {
	Key   string
	Value Tag
}

// TagCompound represents a map of string keys to Tags (TAG_Compound)
type TagCompound struct {
	entries []CompoundEntry
	index   map[string]int
}

// NewTagCompound creates an empty TagCompound
func NewTagCompound() *TagCompound {
	return &TagCompound{
		entries: make([]CompoundEntry, 0),
		index:   make(map[string]int),
	}
}

func (c *TagCompound) Type() TagType { return TagTypeCompound }

// Len returns the number of entries
func (c *TagCompound) Len() int {
	return len(c.entries)
}

// Entries returns all entries
func (c *TagCompound) Entries() []CompoundEntry {
	return c.entries
}

// SortedKeys returns all keys sorted alphabetically
func (c *TagCompound) SortedKeys() []string {
	keys := make([]string, len(c.entries))
	for i, e := range c.entries {
		keys[i] = e.Key
	}
	sort.Strings(keys)
	return keys
}

// Get returns the tag associated with key, and whether it exists
func (c *TagCompound) Get(key string) (Tag, bool) {
	if c.index == nil {
		c.rebuildIndex()
	}
	idx, ok := c.index[key]
	if !ok {
		return nil, false
	}
	return c.entries[idx].Value, true
}

// Set sets or replaces a key-value entry
func (c *TagCompound) Set(key string, val Tag) {
	if c.index == nil {
		c.rebuildIndex()
	}
	if idx, ok := c.index[key]; ok {
		c.entries[idx].Value = val
		return
	}
	c.index[key] = len(c.entries)
	c.entries = append(c.entries, CompoundEntry{Key: key, Value: val})
}

// Delete removes a key
func (c *TagCompound) Delete(key string) {
	if c.index == nil {
		c.rebuildIndex()
	}
	idx, ok := c.index[key]
	if !ok {
		return
	}
	c.entries = append(c.entries[:idx], c.entries[idx+1:]...)
	c.rebuildIndex()
}

func (c *TagCompound) rebuildIndex() {
	c.index = make(map[string]int, len(c.entries))
	for i, e := range c.entries {
		c.index[e.Key] = i
	}
}

// Helper methods for accessing typed fields comfortably
func (c *TagCompound) GetByte(key string) (int8, bool) {
	if t, ok := c.Get(key); ok {
		if v, ok := t.(TagByte); ok {
			return int8(v), true
		}
	}
	return 0, false
}

func (c *TagCompound) GetShort(key string) (int16, bool) {
	if t, ok := c.Get(key); ok {
		if v, ok := t.(TagShort); ok {
			return int16(v), true
		}
	}
	return 0, false
}

func (c *TagCompound) GetInt(key string) (int32, bool) {
	if t, ok := c.Get(key); ok {
		if v, ok := t.(TagInt); ok {
			return int32(v), true
		}
	}
	return 0, false
}

func (c *TagCompound) GetLong(key string) (int64, bool) {
	if t, ok := c.Get(key); ok {
		if v, ok := t.(TagLong); ok {
			return int64(v), true
		}
	}
	return 0, false
}

func (c *TagCompound) GetFloat(key string) (float32, bool) {
	if t, ok := c.Get(key); ok {
		if v, ok := t.(TagFloat); ok {
			return float32(v), true
		}
	}
	return 0, false
}

func (c *TagCompound) GetDouble(key string) (float64, bool) {
	if t, ok := c.Get(key); ok {
		if v, ok := t.(TagDouble); ok {
			return float64(v), true
		}
	}
	return 0, false
}

func (c *TagCompound) GetString(key string) (string, bool) {
	if t, ok := c.Get(key); ok {
		if v, ok := t.(TagString); ok {
			return string(v), true
		}
	}
	return "", false
}

func (c *TagCompound) GetCompound(key string) (*TagCompound, bool) {
	if t, ok := c.Get(key); ok {
		if v, ok := t.(*TagCompound); ok {
			return v, true
		}
	}
	return nil, false
}

func (c *TagCompound) GetList(key string) (*TagList, bool) {
	if t, ok := c.Get(key); ok {
		if v, ok := t.(*TagList); ok {
			return v, true
		}
	}
	return nil, false
}

func (c *TagCompound) Equal(other Tag) bool {
	o, ok := other.(*TagCompound)
	if !ok || len(c.entries) != len(o.entries) {
		return false
	}
	for _, entry := range c.entries {
		otherVal, exists := o.Get(entry.Key)
		if !exists || !entry.Value.Equal(otherVal) {
			return false
		}
	}
	return true
}

func (c *TagCompound) Clone() Tag {
	cp := NewTagCompound()
	for _, e := range c.entries {
		cp.Set(e.Key, e.Value.Clone())
	}
	return cp
}

// String representations for quick debug
func (c *TagCompound) String() string {
	return fmt.Sprintf("Compound[%d entries]", len(c.entries))
}
