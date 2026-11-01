package bdf

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

// View provides high-speed, zero-allocation random access navigation over BDF binary data
type View struct {
	data       []byte
	strings    []string
	rootOffset uint64
}

// NewView creates a zero-copy View over uncompressed BDF bytes
func NewView(data []byte) (*View, error) {
	if len(data) < HeaderSize {
		return nil, fmt.Errorf("data too short for BDF header")
	}
	if data[0] != MagicHeader[0] || data[1] != MagicHeader[1] ||
		data[2] != MagicHeader[2] || data[3] != MagicHeader[3] {
		return nil, fmt.Errorf("invalid BDF magic")
	}

	flags := binary.LittleEndian.Uint16(data[6:8])
	compressionMode := (flags & FlagCompressionMask) >> 1
	var uncompressed []byte
	if compressionMode == CompressionNone {
		uncompressed = data
	} else {
		payload, err := DecompressPayload(data[HeaderSize:], compressionMode)
		if err != nil {
			return nil, err
		}
		uncompressed = make([]byte, HeaderSize+len(payload))
		copy(uncompressed[:HeaderSize], data[:HeaderSize])
		copy(uncompressed[HeaderSize:], payload)
	}

	rootOffset := binary.LittleEndian.Uint64(uncompressed[8:16])
	strTableOffset := binary.LittleEndian.Uint64(uncompressed[16:24])
	strTableSize := binary.LittleEndian.Uint64(uncompressed[24:32])

	v := &View{
		data:       uncompressed,
		rootOffset: rootOffset,
	}

	if flags&FlagStringInterned != 0 && strTableOffset > 0 && strTableSize > 0 {
		if err := v.parseStringTable(strTableOffset); err != nil {
			return nil, err
		}
	}

	return v, nil
}

func (v *View) parseStringTable(offset uint64) error {
	count := binary.LittleEndian.Uint32(v.data[offset : offset+4])
	offsetsPos := offset + 8
	lengthsPos := offsetsPos + uint64(count)*4
	dataPos := lengthsPos + uint64(count)*4

	v.strings = make([]string, count)
	for i := uint32(0); i < count; i++ {
		strOff := binary.LittleEndian.Uint32(v.data[offsetsPos+uint64(i)*4 : offsetsPos+uint64(i+1)*4])
		strLen := binary.LittleEndian.Uint32(v.data[lengthsPos+uint64(i)*4 : lengthsPos+uint64(i+1)*4])
		start := dataPos + uint64(strOff)
		v.strings[i] = string(v.data[start : start+uint64(strLen)])
	}
	return nil
}

// RootCompound returns the root CompoundView
func (v *View) RootCompound() (*CompoundView, error) {
	return v.CompoundAt(v.rootOffset)
}

// CompoundAt returns a CompoundView at a specific offset
func (v *View) CompoundAt(offset uint64) (*CompoundView, error) {
	if offset+8 > uint64(len(v.data)) {
		return nil, fmt.Errorf("compound header out of bounds")
	}
	count := binary.LittleEndian.Uint32(v.data[offset : offset+4])
	flags := binary.LittleEndian.Uint32(v.data[offset+4 : offset+8])
	return &CompoundView{
		v:          v,
		offset:     offset,
		count:      count,
		sorted:     (flags & 1) != 0,
		tableStart: offset + 8,
	}, nil
}

// CompoundView represents a zero-copy view of a Compound tag
type CompoundView struct {
	v          *View
	offset     uint64
	count      uint32
	sorted     bool
	tableStart uint64
}

// Len returns number of entries
func (cv *CompoundView) Len() int {
	return int(cv.count)
}

func (cv *CompoundView) findEntry(key string) (tagType TagType, inlineFlag uint8, valOrOffset uint64, found bool) {
	if cv.count == 0 {
		return 0, 0, 0, false
	}

	if cv.sorted {
		// Fast binary search directly across the 16-byte aligned entries!
		idx := sort.Search(int(cv.count), func(i int) bool {
			pos := cv.tableStart + uint64(i)*16
			keyID := binary.LittleEndian.Uint32(cv.v.data[pos : pos+4])
			entryKey := cv.v.strings[keyID]
			return entryKey >= key
		})

		if idx < int(cv.count) {
			pos := cv.tableStart + uint64(idx)*16
			keyID := binary.LittleEndian.Uint32(cv.v.data[pos : pos+4])
			if cv.v.strings[keyID] == key {
				tagType = TagType(cv.v.data[pos+4])
				inlineFlag = cv.v.data[pos+5]
				valOrOffset = binary.LittleEndian.Uint64(cv.v.data[pos+8 : pos+16])
				return tagType, inlineFlag, valOrOffset, true
			}
		}
		return 0, 0, 0, false
	}

	// Linear fallback
	for i := uint32(0); i < cv.count; i++ {
		pos := cv.tableStart + uint64(i)*16
		keyID := binary.LittleEndian.Uint32(cv.v.data[pos : pos+4])
		if cv.v.strings[keyID] == key {
			tagType = TagType(cv.v.data[pos+4])
			inlineFlag = cv.v.data[pos+5]
			valOrOffset = binary.LittleEndian.Uint64(cv.v.data[pos+8 : pos+16])
			return tagType, inlineFlag, valOrOffset, true
		}
	}
	return 0, 0, 0, false
}

// GetInt fetches a 32-bit int with zero allocations
func (cv *CompoundView) GetInt(key string) (int32, bool) {
	tagType, inline, val, ok := cv.findEntry(key)
	if !ok || tagType != TagTypeInt || inline != 1 {
		return 0, false
	}
	return int32(val), true
}

// GetLong fetches a 64-bit int with zero allocations
func (cv *CompoundView) GetLong(key string) (int64, bool) {
	tagType, inline, val, ok := cv.findEntry(key)
	if !ok || tagType != TagTypeLong || inline != 1 {
		return 0, false
	}
	return int64(val), true
}

// GetDouble fetches a 64-bit float with zero allocations
func (cv *CompoundView) GetDouble(key string) (float64, bool) {
	tagType, inline, val, ok := cv.findEntry(key)
	if !ok || tagType != TagTypeDouble || inline != 1 {
		return 0, false
	}
	return math.Float64frombits(val), true
}

// GetFloat fetches a 32-bit float with zero allocations
func (cv *CompoundView) GetFloat(key string) (float32, bool) {
	tagType, inline, val, ok := cv.findEntry(key)
	if !ok || tagType != TagTypeFloat || inline != 1 {
		return 0, false
	}
	return math.Float32frombits(uint32(val)), true
}

// GetByte fetches an 8-bit int with zero allocations
func (cv *CompoundView) GetByte(key string) (int8, bool) {
	tagType, inline, val, ok := cv.findEntry(key)
	if !ok || tagType != TagTypeByte || inline != 1 {
		return 0, false
	}
	return int8(val), true
}

// GetShort fetches a 16-bit int with zero allocations
func (cv *CompoundView) GetShort(key string) (int16, bool) {
	tagType, inline, val, ok := cv.findEntry(key)
	if !ok || tagType != TagTypeShort || inline != 1 {
		return 0, false
	}
	return int16(val), true
}

// GetString fetches a string directly from string pool
func (cv *CompoundView) GetString(key string) (string, bool) {
	tagType, inline, val, ok := cv.findEntry(key)
	if !ok || tagType != TagTypeString || inline != 1 {
		return "", false
	}
	strID := uint32(val)
	if int(strID) >= len(cv.v.strings) {
		return "", false
	}
	return cv.v.strings[strID], true
}

// GetCompound returns a nested CompoundView without decoding the rest of the file
func (cv *CompoundView) GetCompound(key string) (*CompoundView, bool) {
	tagType, inline, val, ok := cv.findEntry(key)
	if !ok || tagType != TagTypeCompound || inline != 0 {
		return nil, false
	}
	nested, err := cv.v.CompoundAt(val)
	if err != nil {
		return nil, false
	}
	return nested, true
}

// Keys returns all keys present in this compound
func (cv *CompoundView) Keys() []string {
	keys := make([]string, cv.count)
	for i := uint32(0); i < cv.count; i++ {
		pos := cv.tableStart + uint64(i)*16
		keyID := binary.LittleEndian.Uint32(cv.v.data[pos : pos+4])
		keys[i] = cv.v.strings[keyID]
	}
	return keys
}
