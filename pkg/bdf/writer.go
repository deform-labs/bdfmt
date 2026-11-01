package bdf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

// WriterOptions configures BDF encoding
type WriterOptions struct {
	Compression uint16 // CompressionNone, CompressionZstd, CompressionGzip
}

// DefaultWriterOptions returns default writer options (Zstd compression, 64-bit aligned)
func DefaultWriterOptions() WriterOptions {
	return WriterOptions{
		Compression: CompressionNone,
	}
}

// Writer encodes BDF tags into 64-bit optimized binary format
type Writer struct {
	opts        WriterOptions
	stringTable *stringTableBuilder
	buf         *bytes.Buffer
}

// NewWriter creates a new BDF writer
func NewWriter(opts WriterOptions) *Writer {
	return &Writer{
		opts:        opts,
		stringTable: newStringTableBuilder(),
		buf:         new(bytes.Buffer),
	}
}

type stringTableBuilder struct {
	strings []string
	lookup  map[string]uint32
}

func newStringTableBuilder() *stringTableBuilder {
	return &stringTableBuilder{
		strings: make([]string, 0),
		lookup:  make(map[string]uint32),
	}
}

func (s *stringTableBuilder) Intern(str string) uint32 {
	if id, ok := s.lookup[str]; ok {
		return id
	}
	id := uint32(len(s.strings))
	s.strings = append(s.strings, str)
	s.lookup[str] = id
	return id
}

// Write encodes a root Tag into BDF binary bytes
func Write(root Tag, opts WriterOptions) ([]byte, error) {
	w := NewWriter(opts)
	return w.Encode(root)
}

// Encode serializes the tag tree into BDF format
func (w *Writer) Encode(root Tag) ([]byte, error) {
	if root == nil {
		return nil, fmt.Errorf("root tag cannot be nil")
	}

	// 1. Traverse and collect all strings into the string table
	w.collectStrings(root)

	// 2. Prepare uncompressed buffer
	rawBuf := new(bytes.Buffer)

	// Write 64-byte Header placeholder
	header := make([]byte, HeaderSize)
	rawBuf.Write(header)

	// Write String Table immediately after Header (starts at offset 64)
	strTableOffset := uint64(rawBuf.Len())
	strTableSize := w.writeStringTable(rawBuf)

	// Pad to 8-byte boundary before root tag
	w.padTo64(rawBuf)

	// Write root tag payload
	rootOffset := uint64(rawBuf.Len())
	totalTags, err := w.writeTag(rawBuf, root)
	if err != nil {
		return nil, fmt.Errorf("error serializing root tag: %w", err)
	}

	// Fill in Header
	flags := FlagLittleEndian | FlagStringInterned | FlagSortedCompound
	flags |= (w.opts.Compression << 1) & FlagCompressionMask

	copy(header[0:4], MagicHeader[:])
	binary.LittleEndian.PutUint16(header[4:6], CurrentVersion)
	binary.LittleEndian.PutUint16(header[6:8], flags)
	binary.LittleEndian.PutUint64(header[8:16], rootOffset)
	binary.LittleEndian.PutUint64(header[16:24], strTableOffset)
	binary.LittleEndian.PutUint64(header[24:32], strTableSize)
	binary.LittleEndian.PutUint64(header[32:40], uint64(rawBuf.Len()))
	binary.LittleEndian.PutUint64(header[40:48], totalTags)

	// Copy updated header back into buffer
	rawBytes := rawBuf.Bytes()
	copy(rawBytes[0:HeaderSize], header)

	// 3. Apply compression if requested
	if w.opts.Compression == CompressionNone {
		return rawBytes, nil
	}

	// Compress the uncompressed stream
	compressedPayload, err := CompressPayload(rawBytes[HeaderSize:], w.opts.Compression)
	if err != nil {
		return nil, fmt.Errorf("compression failed: %w", err)
	}

	// Result file: 64-byte header + compressed payload
	result := make([]byte, HeaderSize+len(compressedPayload))
	copy(result[:HeaderSize], header)
	copy(result[HeaderSize:], compressedPayload)
	return result, nil
}

func (w *Writer) collectStrings(t Tag) {
	switch v := t.(type) {
	case TagString:
		w.stringTable.Intern(string(v))
	case *TagCompound:
		for _, entry := range v.Entries() {
			w.stringTable.Intern(entry.Key)
			w.collectStrings(entry.Value)
		}
	case *TagList:
		for _, el := range v.Elements {
			w.collectStrings(el)
		}
	}
}

func (w *Writer) writeStringTable(buf *bytes.Buffer) uint64 {
	startOffset := buf.Len()
	count := uint32(len(w.stringTable.strings))

	// Collect total byte length of all strings
	totalLen := uint32(0)
	for _, s := range w.stringTable.strings {
		totalLen += uint32(len(s))
	}

	// String table structure:
	// - Count (uint32)
	// - TotalByteLength (uint32)
	// - Offsets (Count * uint32)
	// - Lengths (Count * uint32)
	// - Raw UTF-8 bytes
	_ = binary.Write(buf, binary.LittleEndian, count)
	_ = binary.Write(buf, binary.LittleEndian, totalLen)

	currOffset := uint32(0)
	for _, s := range w.stringTable.strings {
		_ = binary.Write(buf, binary.LittleEndian, currOffset)
		currOffset += uint32(len(s))
	}
	for _, s := range w.stringTable.strings {
		_ = binary.Write(buf, binary.LittleEndian, uint32(len(s)))
	}
	for _, s := range w.stringTable.strings {
		buf.WriteString(s)
	}

	w.padTo64(buf)
	return uint64(buf.Len() - startOffset)
}

func (w *Writer) padTo64(buf *bytes.Buffer) {
	pad := AlignPad(uint64(buf.Len()))
	if pad > 0 {
		zeros := make([]byte, pad)
		buf.Write(zeros)
	}
}

// writeTag serializes a tag at an 8-byte aligned position and returns total tag count
func (w *Writer) writeTag(buf *bytes.Buffer, t Tag) (uint64, error) {
	w.padTo64(buf)
	switch v := t.(type) {
	case TagByte:
		_ = binary.Write(buf, binary.LittleEndian, int8(v))
		w.padTo64(buf)
		return 1, nil
	case TagShort:
		_ = binary.Write(buf, binary.LittleEndian, int16(v))
		w.padTo64(buf)
		return 1, nil
	case TagInt:
		_ = binary.Write(buf, binary.LittleEndian, int32(v))
		w.padTo64(buf)
		return 1, nil
	case TagLong:
		_ = binary.Write(buf, binary.LittleEndian, int64(v))
		return 1, nil
	case TagFloat:
		_ = binary.Write(buf, binary.LittleEndian, float32(v))
		w.padTo64(buf)
		return 1, nil
	case TagDouble:
		_ = binary.Write(buf, binary.LittleEndian, float64(v))
		return 1, nil
	case TagString:
		strID := w.stringTable.Intern(string(v))
		_ = binary.Write(buf, binary.LittleEndian, strID)
		w.padTo64(buf)
		return 1, nil
	case TagByteArray:
		_ = binary.Write(buf, binary.LittleEndian, uint32(len(v)))
		_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // 4 bytes padding
		buf.Write(v)
		w.padTo64(buf)
		return 1, nil
	case TagIntArray:
		_ = binary.Write(buf, binary.LittleEndian, uint32(len(v)))
		_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // 4 bytes padding
		for _, n := range v {
			_ = binary.Write(buf, binary.LittleEndian, n)
		}
		w.padTo64(buf)
		return 1, nil
	case TagLongArray:
		_ = binary.Write(buf, binary.LittleEndian, uint64(len(v)))
		for _, n := range v {
			_ = binary.Write(buf, binary.LittleEndian, n)
		}
		return 1, nil
	case *TagList:
		return w.writeList(buf, v)
	case *TagCompound:
		return w.writeCompound(buf, v)
	default:
		return 0, fmt.Errorf("unknown tag type %T", t)
	}
}

func (w *Writer) writeList(buf *bytes.Buffer, list *TagList) (uint64, error) {
	elemType := list.ElementType
	count := uint32(len(list.Elements))

	// List header: 1 byte ElemType, 3 bytes padding, 4 bytes Count (total 8 bytes!)
	buf.WriteByte(byte(elemType))
	buf.Write([]byte{0, 0, 0})
	_ = binary.Write(buf, binary.LittleEndian, count)

	var totalTags uint64 = 1

	if count == 0 {
		return totalTags, nil
	}

	// Optimize primitive lists directly into aligned contiguous arrays!
	switch elemType {
	case TagTypeByte:
		for _, el := range list.Elements {
			buf.WriteByte(byte(el.(TagByte)))
		}
		w.padTo64(buf)
		return totalTags + uint64(count), nil

	case TagTypeShort:
		for _, el := range list.Elements {
			_ = binary.Write(buf, binary.LittleEndian, int16(el.(TagShort)))
		}
		w.padTo64(buf)
		return totalTags + uint64(count), nil

	case TagTypeInt:
		for _, el := range list.Elements {
			_ = binary.Write(buf, binary.LittleEndian, int32(el.(TagInt)))
		}
		w.padTo64(buf)
		return totalTags + uint64(count), nil

	case TagTypeLong:
		for _, el := range list.Elements {
			_ = binary.Write(buf, binary.LittleEndian, int64(el.(TagLong)))
		}
		return totalTags + uint64(count), nil

	case TagTypeFloat:
		for _, el := range list.Elements {
			_ = binary.Write(buf, binary.LittleEndian, float32(el.(TagFloat)))
		}
		w.padTo64(buf)
		return totalTags + uint64(count), nil

	case TagTypeDouble:
		for _, el := range list.Elements {
			_ = binary.Write(buf, binary.LittleEndian, float64(el.(TagDouble)))
		}
		return totalTags + uint64(count), nil

	case TagTypeString:
		// Array of uint32 string IDs, padded to 8 bytes
		for _, el := range list.Elements {
			strID := w.stringTable.Intern(string(el.(TagString)))
			_ = binary.Write(buf, binary.LittleEndian, strID)
		}
		w.padTo64(buf)
		return totalTags + uint64(count), nil

	default:
		// Complex elements (Compound, List, Arrays): array of uint64 offsets
		offsetsPos := buf.Len()
		offsetPlaceholders := make([]byte, int(count)*8)
		buf.Write(offsetPlaceholders)

		offsets := make([]uint64, count)
		for i, el := range list.Elements {
			w.padTo64(buf)
			offsets[i] = uint64(buf.Len())
			subCount, err := w.writeTag(buf, el)
			if err != nil {
				return 0, err
			}
			totalTags += subCount
		}

		// Patch offsets
		data := buf.Bytes()
		for i, off := range offsets {
			binary.LittleEndian.PutUint64(data[offsetsPos+i*8:offsetsPos+(i+1)*8], off)
		}
		return totalTags, nil
	}
}

type entrySortItem struct {
	entry CompoundEntry
	keyID uint32
}

func (w *Writer) writeCompound(buf *bytes.Buffer, comp *TagCompound) (uint64, error) {
	entries := comp.Entries()
	count := uint32(len(entries))

	// Sort entries by Key string for deterministic binary search support
	sorted := make([]entrySortItem, len(entries))
	for i, e := range entries {
		sorted[i] = entrySortItem{
			entry: e,
			keyID: w.stringTable.Intern(e.Key),
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].entry.Key < sorted[j].entry.Key
	})

	// Compound Header: 4 bytes Count, 4 bytes Flags (bit 0 = sorted)
	_ = binary.Write(buf, binary.LittleEndian, count)
	_ = binary.Write(buf, binary.LittleEndian, uint32(1)) // Flags = 1 (Sorted)

	var totalTags uint64 = 1

	if count == 0 {
		return totalTags, nil
	}

	// Each entry is 16 bytes:
	// - KeyID (uint32)
	// - TagType (uint8)
	// - InlineFlag (uint8) (1 if value is inlined in ValueOrOffset)
	// - Reserved (uint16)
	// - ValueOrOffset (uint64)
	tableStart := buf.Len()
	tableSize := int(count) * 16
	tablePlaceholders := make([]byte, tableSize)
	buf.Write(tablePlaceholders)

	type pendingPayload struct {
		index uint32
		val   Tag
	}
	var pending []pendingPayload

	for i, item := range sorted {
		val := item.entry.Value
		tagType := val.Type()
		var inlineFlag uint8 = 0
		var valueOrOffset uint64 = 0

		switch v := val.(type) {
		case TagByte:
			inlineFlag = 1
			valueOrOffset = uint64(uint8(v))
			totalTags++
		case TagShort:
			inlineFlag = 1
			valueOrOffset = uint64(uint16(v))
			totalTags++
		case TagInt:
			inlineFlag = 1
			valueOrOffset = uint64(uint32(v))
			totalTags++
		case TagLong:
			inlineFlag = 1
			valueOrOffset = uint64(v)
			totalTags++
		case TagFloat:
			inlineFlag = 1
			valueOrOffset = uint64(math.Float32bits(float32(v)))
			totalTags++
		case TagDouble:
			inlineFlag = 1
			valueOrOffset = math.Float64bits(float64(v))
			totalTags++
		case TagString:
			// Strings are interned! String ID is inlined directly in the 16-byte entry!
			inlineFlag = 1
			strID := w.stringTable.Intern(string(v))
			valueOrOffset = uint64(strID)
			totalTags++
		default:
			// Composite types (List, Compound, ByteArray, IntArray, LongArray)
			inlineFlag = 0
			pending = append(pending, pendingPayload{index: uint32(i), val: val})
		}

		// Write entry into placeholder
		entryOffset := tableStart + i*16
		data := buf.Bytes()
		binary.LittleEndian.PutUint32(data[entryOffset:entryOffset+4], item.keyID)
		data[entryOffset+4] = byte(tagType)
		data[entryOffset+5] = inlineFlag
		binary.LittleEndian.PutUint16(data[entryOffset+6:entryOffset+8], 0)
		binary.LittleEndian.PutUint64(data[entryOffset+8:entryOffset+16], valueOrOffset)
	}

	// Write all non-inlined payloads
	for _, p := range pending {
		w.padTo64(buf)
		payloadOffset := uint64(buf.Len())
		subCount, err := w.writeTag(buf, p.val)
		if err != nil {
			return 0, err
		}
		totalTags += subCount

		// Patch entry's ValueOrOffset with payload offset
		entryOffset := tableStart + int(p.index)*16
		data := buf.Bytes()
		binary.LittleEndian.PutUint64(data[entryOffset+8:entryOffset+16], payloadOffset)
	}

	return totalTags, nil
}
