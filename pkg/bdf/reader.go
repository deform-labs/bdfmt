package bdf

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Reader decodes BDF binary data into Tag structures
type Reader struct {
	data       []byte
	strings    []string
	flags      uint16
	version    uint16
	rootOffset uint64
	totalTags  uint64
}

// Read decodes BDF binary bytes into a Tag
func Read(bdfData []byte) (Tag, error) {
	r, err := NewReader(bdfData)
	if err != nil {
		return nil, err
	}
	return r.Decode()
}

// NewReader initializes a BDF reader by parsing the header and decompression if needed
func NewReader(bdfData []byte) (*Reader, error) {
	if len(bdfData) < HeaderSize {
		return nil, fmt.Errorf("data too short for BDF header: got %d bytes, need at least %d", len(bdfData), HeaderSize)
	}

	// Verify magic
	if bdfData[0] != MagicHeader[0] || bdfData[1] != MagicHeader[1] ||
		bdfData[2] != MagicHeader[2] || bdfData[3] != MagicHeader[3] {
		return nil, fmt.Errorf("invalid BDF magic header: %x", bdfData[0:4])
	}

	version := binary.LittleEndian.Uint16(bdfData[4:6])
	if version != CurrentVersion {
		return nil, fmt.Errorf("unsupported BDF version: %d (expected %d)", version, CurrentVersion)
	}

	flags := binary.LittleEndian.Uint16(bdfData[6:8])
	rootOffset := binary.LittleEndian.Uint64(bdfData[8:16])
	strTableOffset := binary.LittleEndian.Uint64(bdfData[16:24])
	strTableSize := binary.LittleEndian.Uint64(bdfData[24:32])
	_ = binary.LittleEndian.Uint64(bdfData[32:40]) // uncompressedSize
	totalTags := binary.LittleEndian.Uint64(bdfData[40:48])

	compressionMode := (flags & FlagCompressionMask) >> 1

	var uncompressed []byte
	if compressionMode == CompressionNone {
		uncompressed = bdfData
	} else {
		// Payload is after HeaderSize
		decompressedPayload, err := DecompressPayload(bdfData[HeaderSize:], compressionMode)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress BDF payload: %w", err)
		}
		// Reconstruct uncompressed buffer with header
		uncompressed = make([]byte, HeaderSize+len(decompressedPayload))
		copy(uncompressed[:HeaderSize], bdfData[:HeaderSize])
		copy(uncompressed[HeaderSize:], decompressedPayload)
	}

	r := &Reader{
		data:       uncompressed,
		flags:      flags,
		version:    version,
		rootOffset: rootOffset,
		totalTags:  totalTags,
	}

	// Parse string table if present
	if flags&FlagStringInterned != 0 && strTableOffset > 0 && strTableSize > 0 {
		if err := r.parseStringTable(strTableOffset); err != nil {
			return nil, fmt.Errorf("failed to parse string table: %w", err)
		}
	}

	return r, nil
}

func (r *Reader) parseStringTable(offset uint64) error {
	if offset+8 > uint64(len(r.data)) {
		return fmt.Errorf("string table offset out of bounds")
	}

	count := binary.LittleEndian.Uint32(r.data[offset : offset+4])
	totalLen := binary.LittleEndian.Uint32(r.data[offset+4 : offset+8])

	headerBytes := 8 + int(count)*8 // 8 bytes for count+len, count*4 for offsets, count*4 for lengths
	if offset+uint64(headerBytes)+uint64(totalLen) > uint64(len(r.data)) {
		return fmt.Errorf("string table data out of bounds")
	}

	offsetsPos := offset + 8
	lengthsPos := offsetsPos + uint64(count)*4
	dataPos := lengthsPos + uint64(count)*4

	r.strings = make([]string, count)
	for i := uint32(0); i < count; i++ {
		strOff := binary.LittleEndian.Uint32(r.data[offsetsPos+uint64(i)*4 : offsetsPos+uint64(i+1)*4])
		strLen := binary.LittleEndian.Uint32(r.data[lengthsPos+uint64(i)*4 : lengthsPos+uint64(i+1)*4])

		start := dataPos + uint64(strOff)
		end := start + uint64(strLen)
		if end > uint64(len(r.data)) {
			return fmt.Errorf("string index %d extends beyond data buffer", i)
		}
		r.strings[i] = string(r.data[start:end])
	}

	return nil
}

// Decode extracts the root tag from the BDF data
func (r *Reader) Decode() (Tag, error) {
	return r.readCompound(r.rootOffset)
}

func (r *Reader) readTagAt(offset uint64, tagType TagType) (Tag, error) {
	if offset >= uint64(len(r.data)) {
		return nil, fmt.Errorf("tag offset %d out of bounds (len %d)", offset, len(r.data))
	}

	switch tagType {
	case TagTypeByte:
		return TagByte(int8(r.data[offset])), nil
	case TagTypeShort:
		if offset+2 > uint64(len(r.data)) {
			return nil, fmt.Errorf("short out of bounds")
		}
		return TagShort(int16(binary.LittleEndian.Uint16(r.data[offset : offset+2]))), nil
	case TagTypeInt:
		if offset+4 > uint64(len(r.data)) {
			return nil, fmt.Errorf("int out of bounds")
		}
		return TagInt(int32(binary.LittleEndian.Uint32(r.data[offset : offset+4]))), nil
	case TagTypeLong:
		if offset+8 > uint64(len(r.data)) {
			return nil, fmt.Errorf("long out of bounds")
		}
		return TagLong(int64(binary.LittleEndian.Uint64(r.data[offset : offset+8]))), nil
	case TagTypeFloat:
		if offset+4 > uint64(len(r.data)) {
			return nil, fmt.Errorf("float out of bounds")
		}
		bits := binary.LittleEndian.Uint32(r.data[offset : offset+4])
		return TagFloat(math.Float32frombits(bits)), nil
	case TagTypeDouble:
		if offset+8 > uint64(len(r.data)) {
			return nil, fmt.Errorf("double out of bounds")
		}
		bits := binary.LittleEndian.Uint64(r.data[offset : offset+8])
		return TagDouble(math.Float64frombits(bits)), nil
	case TagTypeString:
		if offset+4 > uint64(len(r.data)) {
			return nil, fmt.Errorf("string ID out of bounds")
		}
		strID := binary.LittleEndian.Uint32(r.data[offset : offset+4])
		if int(strID) >= len(r.strings) {
			return nil, fmt.Errorf("invalid string ID %d (total %d)", strID, len(r.strings))
		}
		return TagString(r.strings[strID]), nil
	case TagTypeByteArray:
		return r.readByteArray(offset)
	case TagTypeIntArray:
		return r.readIntArray(offset)
	case TagTypeLongArray:
		return r.readLongArray(offset)
	case TagTypeList:
		return r.readList(offset)
	case TagTypeCompound:
		return r.readCompound(offset)
	default:
		return nil, fmt.Errorf("unknown tag type: %d", tagType)
	}
}

func (r *Reader) readByteArray(offset uint64) (Tag, error) {
	if offset+8 > uint64(len(r.data)) {
		return nil, fmt.Errorf("byte array header out of bounds")
	}
	length := binary.LittleEndian.Uint32(r.data[offset : offset+4])
	dataStart := offset + 8
	if dataStart+uint64(length) > uint64(len(r.data)) {
		return nil, fmt.Errorf("byte array payload out of bounds")
	}
	bytesCopy := make([]byte, length)
	copy(bytesCopy, r.data[dataStart:dataStart+uint64(length)])
	return TagByteArray(bytesCopy), nil
}

func (r *Reader) readIntArray(offset uint64) (Tag, error) {
	if offset+8 > uint64(len(r.data)) {
		return nil, fmt.Errorf("int array header out of bounds")
	}
	length := binary.LittleEndian.Uint32(r.data[offset : offset+4])
	dataStart := offset + 8
	if dataStart+uint64(length)*4 > uint64(len(r.data)) {
		return nil, fmt.Errorf("int array payload out of bounds")
	}
	ints := make([]int32, length)
	for i := uint32(0); i < length; i++ {
		pos := dataStart + uint64(i)*4
		ints[i] = int32(binary.LittleEndian.Uint32(r.data[pos : pos+4]))
	}
	return TagIntArray(ints), nil
}

func (r *Reader) readLongArray(offset uint64) (Tag, error) {
	if offset+8 > uint64(len(r.data)) {
		return nil, fmt.Errorf("long array header out of bounds")
	}
	length := binary.LittleEndian.Uint64(r.data[offset : offset+8])
	dataStart := offset + 8
	if dataStart+length*8 > uint64(len(r.data)) {
		return nil, fmt.Errorf("long array payload out of bounds")
	}
	longs := make([]int64, length)
	for i := uint64(0); i < length; i++ {
		pos := dataStart + i*8
		longs[i] = int64(binary.LittleEndian.Uint64(r.data[pos : pos+8]))
	}
	return TagLongArray(longs), nil
}

func (r *Reader) readList(offset uint64) (Tag, error) {
	if offset+8 > uint64(len(r.data)) {
		return nil, fmt.Errorf("list header out of bounds")
	}
	elemType := TagType(r.data[offset])
	count := binary.LittleEndian.Uint32(r.data[offset+4 : offset+8])

	list := &TagList{
		ElementType: elemType,
		Elements:    make([]Tag, count),
	}
	if count == 0 {
		return list, nil
	}

	payloadStart := offset + 8

	switch elemType {
	case TagTypeByte:
		if payloadStart+uint64(count) > uint64(len(r.data)) {
			return nil, fmt.Errorf("byte list out of bounds")
		}
		for i := uint32(0); i < count; i++ {
			list.Elements[i] = TagByte(int8(r.data[payloadStart+uint64(i)]))
		}
	case TagTypeShort:
		if payloadStart+uint64(count)*2 > uint64(len(r.data)) {
			return nil, fmt.Errorf("short list out of bounds")
		}
		for i := uint32(0); i < count; i++ {
			pos := payloadStart + uint64(i)*2
			list.Elements[i] = TagShort(int16(binary.LittleEndian.Uint16(r.data[pos : pos+2])))
		}
	case TagTypeInt:
		if payloadStart+uint64(count)*4 > uint64(len(r.data)) {
			return nil, fmt.Errorf("int list out of bounds")
		}
		for i := uint32(0); i < count; i++ {
			pos := payloadStart + uint64(i)*4
			list.Elements[i] = TagInt(int32(binary.LittleEndian.Uint32(r.data[pos : pos+4])))
		}
	case TagTypeLong:
		if payloadStart+uint64(count)*8 > uint64(len(r.data)) {
			return nil, fmt.Errorf("long list out of bounds")
		}
		for i := uint32(0); i < count; i++ {
			pos := payloadStart + uint64(i)*8
			list.Elements[i] = TagLong(int64(binary.LittleEndian.Uint64(r.data[pos : pos+8])))
		}
	case TagTypeFloat:
		if payloadStart+uint64(count)*4 > uint64(len(r.data)) {
			return nil, fmt.Errorf("float list out of bounds")
		}
		for i := uint32(0); i < count; i++ {
			pos := payloadStart + uint64(i)*4
			bits := binary.LittleEndian.Uint32(r.data[pos : pos+4])
			list.Elements[i] = TagFloat(math.Float32frombits(bits))
		}
	case TagTypeDouble:
		if payloadStart+uint64(count)*8 > uint64(len(r.data)) {
			return nil, fmt.Errorf("double list out of bounds")
		}
		for i := uint32(0); i < count; i++ {
			pos := payloadStart + uint64(i)*8
			bits := binary.LittleEndian.Uint64(r.data[pos : pos+8])
			list.Elements[i] = TagDouble(math.Float64frombits(bits))
		}
	case TagTypeString:
		if payloadStart+uint64(count)*4 > uint64(len(r.data)) {
			return nil, fmt.Errorf("string list out of bounds")
		}
		for i := uint32(0); i < count; i++ {
			pos := payloadStart + uint64(i)*4
			strID := binary.LittleEndian.Uint32(r.data[pos : pos+4])
			if int(strID) >= len(r.strings) {
				return nil, fmt.Errorf("invalid string ID %d in list", strID)
			}
			list.Elements[i] = TagString(r.strings[strID])
		}
	default:
		// Complex types: payloadStart contains array of uint64 offsets
		if payloadStart+uint64(count)*8 > uint64(len(r.data)) {
			return nil, fmt.Errorf("complex list offsets out of bounds")
		}
		for i := uint32(0); i < count; i++ {
			pos := payloadStart + uint64(i)*8
			elemOffset := binary.LittleEndian.Uint64(r.data[pos : pos+8])
			elem, err := r.readTagAt(elemOffset, elemType)
			if err != nil {
				return nil, fmt.Errorf("failed to read list element %d: %w", i, err)
			}
			list.Elements[i] = elem
		}
	}

	return list, nil
}

func (r *Reader) readCompound(offset uint64) (*TagCompound, error) {
	if offset+8 > uint64(len(r.data)) {
		return nil, fmt.Errorf("compound header out of bounds")
	}
	count := binary.LittleEndian.Uint32(r.data[offset : offset+4])
	_ = binary.LittleEndian.Uint32(r.data[offset+4 : offset+8]) // flags

	comp := NewTagCompound()
	if count == 0 {
		return comp, nil
	}

	tableStart := offset + 8
	tableEnd := tableStart + uint64(count)*16
	if tableEnd > uint64(len(r.data)) {
		return nil, fmt.Errorf("compound entry table out of bounds")
	}

	for i := uint32(0); i < count; i++ {
		entryPos := tableStart + uint64(i)*16
		keyID := binary.LittleEndian.Uint32(r.data[entryPos : entryPos+4])
		tagType := TagType(r.data[entryPos+4])
		inlineFlag := r.data[entryPos+5]
		valOrOffset := binary.LittleEndian.Uint64(r.data[entryPos+8 : entryPos+16])

		if int(keyID) >= len(r.strings) {
			return nil, fmt.Errorf("invalid key ID %d in compound (total strings: %d)", keyID, len(r.strings))
		}
		key := r.strings[keyID]

		var val Tag
		if inlineFlag == 1 {
			switch tagType {
			case TagTypeByte:
				val = TagByte(int8(valOrOffset))
			case TagTypeShort:
				val = TagShort(int16(valOrOffset))
			case TagTypeInt:
				val = TagInt(int32(valOrOffset))
			case TagTypeLong:
				val = TagLong(int64(valOrOffset))
			case TagTypeFloat:
				val = TagFloat(math.Float32frombits(uint32(valOrOffset)))
			case TagTypeDouble:
				val = TagDouble(math.Float64frombits(valOrOffset))
			case TagTypeString:
				strID := uint32(valOrOffset)
				if int(strID) >= len(r.strings) {
					return nil, fmt.Errorf("invalid string ID %d for key %q", strID, key)
				}
				val = TagString(r.strings[strID])
			default:
				return nil, fmt.Errorf("unexpected inlined tag type: %d for key %q", tagType, key)
			}
		} else {
			var err error
			val, err = r.readTagAt(valOrOffset, tagType)
			if err != nil {
				return nil, fmt.Errorf("failed to read tag for key %q at offset %d: %w", key, valOrOffset, err)
			}
		}

		comp.Set(key, val)
	}

	return comp, nil
}
