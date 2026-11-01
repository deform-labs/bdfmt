package nbt

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
)

// Compression format for NBT files
type Compression int

const (
	CompressionAuto Compression = iota
	CompressionNone
	CompressionGzip
	CompressionZlib
)

// Read reads an NBT file or stream with automatic compression detection
func Read(data []byte) (string, bdf.Tag, error) {
	reader, err := detectReader(data)
	if err != nil {
		return "", nil, err
	}

	rootTypeByte := make([]byte, 1)
	if _, err := io.ReadFull(reader, rootTypeByte); err != nil {
		return "", nil, fmt.Errorf("failed to read root tag type: %w", err)
	}

	rootType := bdf.TagType(rootTypeByte[0])
	if rootType == bdf.TagTypeEnd {
		return "", nil, fmt.Errorf("root tag cannot be TAG_End")
	}

	rootName, err := readString(reader)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read root tag name: %w", err)
	}

	tag, err := readTagPayload(reader, rootType)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read root payload: %w", err)
	}

	return rootName, tag, nil
}

// Write writes a tag to standard Big-Endian NBT with optional compression
func Write(rootName string, tag bdf.Tag, compression Compression) ([]byte, error) {
	var rawBuf bytes.Buffer

	// Write root tag ID
	if err := rawBuf.WriteByte(byte(tag.Type())); err != nil {
		return nil, err
	}
	// Write root tag name
	if err := writeString(&rawBuf, rootName); err != nil {
		return nil, err
	}
	// Write payload
	if err := writeTagPayload(&rawBuf, tag); err != nil {
		return nil, err
	}

	rawBytes := rawBuf.Bytes()

	switch compression {
	case CompressionNone:
		return rawBytes, nil

	case CompressionGzip:
		var gzBuf bytes.Buffer
		gw := gzip.NewWriter(&gzBuf)
		if _, err := gw.Write(rawBytes); err != nil {
			_ = gw.Close()
			return nil, err
		}
		if err := gw.Close(); err != nil {
			return nil, err
		}
		return gzBuf.Bytes(), nil

	case CompressionZlib:
		var zBuf bytes.Buffer
		zw := zlib.NewWriter(&zBuf)
		if _, err := zw.Write(rawBytes); err != nil {
			_ = zw.Close()
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}
		return zBuf.Bytes(), nil

	default:
		return rawBytes, nil
	}
}

func detectReader(data []byte) (io.Reader, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		// Gzip
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		decompressed, err := io.ReadAll(gr)
		_ = gr.Close()
		if err != nil {
			return nil, fmt.Errorf("gzip read failed: %w", err)
		}
		return bytes.NewReader(decompressed), nil
	}

	if len(data) >= 2 && data[0] == 0x78 && (data[1] == 0x9c || data[1] == 0x01 || data[1] == 0xda) {
		// Zlib
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err == nil {
			decompressed, readErr := io.ReadAll(zr)
			_ = zr.Close()
			if readErr == nil {
				return bytes.NewReader(decompressed), nil
			}
		}
	}

	return bytes.NewReader(data), nil
}

func readString(r io.Reader) (string, error) {
	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func writeString(w io.Writer, s string) error {
	strBytes := []byte(s)
	if len(strBytes) > math.MaxUint16 {
		return fmt.Errorf("string length exceeds uint16: %d", len(strBytes))
	}
	if err := binary.Write(w, binary.BigEndian, uint16(len(strBytes))); err != nil {
		return err
	}
	_, err := w.Write(strBytes)
	return err
}

func readTagPayload(r io.Reader, tagType bdf.TagType) (bdf.Tag, error) {
	switch tagType {
	case bdf.TagTypeByte:
		var v int8
		err := binary.Read(r, binary.BigEndian, &v)
		return bdf.TagByte(v), err

	case bdf.TagTypeShort:
		var v int16
		err := binary.Read(r, binary.BigEndian, &v)
		return bdf.TagShort(v), err

	case bdf.TagTypeInt:
		var v int32
		err := binary.Read(r, binary.BigEndian, &v)
		return bdf.TagInt(v), err

	case bdf.TagTypeLong:
		var v int64
		err := binary.Read(r, binary.BigEndian, &v)
		return bdf.TagLong(v), err

	case bdf.TagTypeFloat:
		var v float32
		err := binary.Read(r, binary.BigEndian, &v)
		return bdf.TagFloat(v), err

	case bdf.TagTypeDouble:
		var v float64
		err := binary.Read(r, binary.BigEndian, &v)
		return bdf.TagDouble(v), err

	case bdf.TagTypeByteArray:
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		if length < 0 {
			return nil, fmt.Errorf("negative byte array length: %d", length)
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return bdf.TagByteArray(buf), nil

	case bdf.TagTypeString:
		s, err := readString(r)
		return bdf.TagString(s), err

	case bdf.TagTypeList:
		var elemTypeByte uint8
		if err := binary.Read(r, binary.BigEndian, &elemTypeByte); err != nil {
			return nil, err
		}
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		elemType := bdf.TagType(elemTypeByte)
		if length <= 0 {
			return &bdf.TagList{ElementType: elemType, Elements: []bdf.Tag{}}, nil
		}
		elements := make([]bdf.Tag, length)
		for i := int32(0); i < length; i++ {
			el, err := readTagPayload(r, elemType)
			if err != nil {
				return nil, fmt.Errorf("error reading list element %d: %w", i, err)
			}
			elements[i] = el
		}
		return &bdf.TagList{ElementType: elemType, Elements: elements}, nil

	case bdf.TagTypeCompound:
		comp := bdf.NewTagCompound()
		for {
			var entryTypeByte uint8
			if err := binary.Read(r, binary.BigEndian, &entryTypeByte); err != nil {
				return nil, err
			}
			entryType := bdf.TagType(entryTypeByte)
			if entryType == bdf.TagTypeEnd {
				break
			}
			name, err := readString(r)
			if err != nil {
				return nil, err
			}
			val, err := readTagPayload(r, entryType)
			if err != nil {
				return nil, fmt.Errorf("error reading compound field %q: %w", name, err)
			}
			comp.Set(name, val)
		}
		return comp, nil

	case bdf.TagTypeIntArray:
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		if length < 0 {
			return nil, fmt.Errorf("negative int array length: %d", length)
		}
		arr := make([]int32, length)
		for i := int32(0); i < length; i++ {
			if err := binary.Read(r, binary.BigEndian, &arr[i]); err != nil {
				return nil, err
			}
		}
		return bdf.TagIntArray(arr), nil

	case bdf.TagTypeLongArray:
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		if length < 0 {
			return nil, fmt.Errorf("negative long array length: %d", length)
		}
		arr := make([]int64, length)
		for i := int32(0); i < length; i++ {
			if err := binary.Read(r, binary.BigEndian, &arr[i]); err != nil {
				return nil, err
			}
		}
		return bdf.TagLongArray(arr), nil

	default:
		return nil, fmt.Errorf("unknown NBT tag type: %d", tagType)
	}
}

func writeTagPayload(w io.Writer, t bdf.Tag) error {
	switch v := t.(type) {
	case bdf.TagByte:
		return binary.Write(w, binary.BigEndian, int8(v))
	case bdf.TagShort:
		return binary.Write(w, binary.BigEndian, int16(v))
	case bdf.TagInt:
		return binary.Write(w, binary.BigEndian, int32(v))
	case bdf.TagLong:
		return binary.Write(w, binary.BigEndian, int64(v))
	case bdf.TagFloat:
		return binary.Write(w, binary.BigEndian, float32(v))
	case bdf.TagDouble:
		return binary.Write(w, binary.BigEndian, float64(v))
	case bdf.TagByteArray:
		if err := binary.Write(w, binary.BigEndian, int32(len(v))); err != nil {
			return err
		}
		_, err := w.Write(v)
		return err
	case bdf.TagString:
		return writeString(w, string(v))
	case *bdf.TagList:
		if err := binary.Write(w, binary.BigEndian, byte(v.ElementType)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, int32(len(v.Elements))); err != nil {
			return err
		}
		for _, el := range v.Elements {
			if err := writeTagPayload(w, el); err != nil {
				return err
			}
		}
		return nil
	case *bdf.TagCompound:
		for _, entry := range v.Entries() {
			if err := binary.Write(w, binary.BigEndian, byte(entry.Value.Type())); err != nil {
				return err
			}
			if err := writeString(w, entry.Key); err != nil {
				return err
			}
			if err := writeTagPayload(w, entry.Value); err != nil {
				return err
			}
		}
		// Write TAG_End
		return binary.Write(w, binary.BigEndian, byte(bdf.TagTypeEnd))
	case bdf.TagIntArray:
		if err := binary.Write(w, binary.BigEndian, int32(len(v))); err != nil {
			return err
		}
		for _, n := range v {
			if err := binary.Write(w, binary.BigEndian, n); err != nil {
				return err
			}
		}
		return nil
	case bdf.TagLongArray:
		if err := binary.Write(w, binary.BigEndian, int32(len(v))); err != nil {
			return err
		}
		for _, n := range v {
			if err := binary.Write(w, binary.BigEndian, n); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported tag type: %T", t)
	}
}
