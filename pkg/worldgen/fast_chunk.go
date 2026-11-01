package worldgen

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync"

	"github.com/klauspost/compress/zlib"
)

// yByteOffset is the byte offset of the Y value in a pre-rendered static section blob.
// Layout of writeSectionNBT output:
//
//	01             TAG_Byte
//	00 01 59       name length (1) + 'Y'
//	XX             ← Y value here (offset 4)
//
// A section's Y is relative to the dimension's minimum section Y. The first
// serialized section therefore contains -4 (0xfc), not zero. Comparisons with
// a reference section must use the same Y value; the full chunk root adds a
// three-byte prefix, but this template itself starts at offset zero.
const yByteOffset = 4

var (
	staticAirSectionBytes       []byte
	staticDeepslateSectionBytes []byte
	staticStoneSectionBytes     []byte
	initStaticOnce              sync.Once
)

func initStaticByteTemplates() {
	initStaticOnce.Do(func() {
		var buf bytes.Buffer

		writeSectionNBT(&buf, 0, 0, nil)
		staticAirSectionBytes = make([]byte, buf.Len())
		copy(staticAirSectionBytes, buf.Bytes())

		buf.Reset()
		writeSectionNBT(&buf, 0, 1, nil)
		staticDeepslateSectionBytes = make([]byte, buf.Len())
		copy(staticDeepslateSectionBytes, buf.Bytes())

		buf.Reset()
		writeSectionNBT(&buf, 0, 2, nil)
		staticStoneSectionBytes = make([]byte, buf.Len())
		copy(staticStoneSectionBytes, buf.Bytes())
	})
}

func writeNBTString(w io.Writer, s string) {
	_ = binary.Write(w, binary.BigEndian, uint16(len(s)))
	_, _ = io.WriteString(w, s)
}

// writeStaticSection writes a cached section template with its per-section Y
// value without allocating and copying the complete template for every chunk.
func writeStaticSection(w io.Writer, template []byte, secY int8) error {
	if len(template) <= yByteOffset {
		return io.ErrUnexpectedEOF
	}
	if _, err := w.Write(template[:yByteOffset]); err != nil {
		return err
	}
	if _, err := w.Write([]byte{byte(secY)}); err != nil {
		return err
	}
	_, err := w.Write(template[yByteOffset+1:])
	return err
}

// writeSectionNBT writes the payload of one section compound (without a surrounding root tag).
// The first 5 bytes are always:  01 00 01 59 YY  (TAG_Byte "Y" value)
func writeSectionNBT(w io.Writer, secY int8, secType uint8, data []int64) {
	// TAG_Byte "Y" value
	_, _ = w.Write([]byte{1, 0, 1, 'Y'})
	_ = binary.Write(w, binary.BigEndian, secY) // offset 4 = Y value

	// biomes compound
	_, _ = w.Write([]byte{10, 0, 6, 'b', 'i', 'o', 'm', 'e', 's'})
	// palette list of strings
	_, _ = w.Write([]byte{9, 0, 7, 'p', 'a', 'l', 'e', 't', 't', 'e'})
	_, _ = w.Write([]byte{8}) // element type TAG_String
	_ = binary.Write(w, binary.BigEndian, int32(1))
	writeNBTString(w, "minecraft:plains")
	_, _ = w.Write([]byte{0}) // end biomes

	// block_states compound
	_, _ = w.Write([]byte{10, 0, 12, 'b', 'l', 'o', 'c', 'k', '_', 's', 't', 'a', 't', 'e', 's'})
	_, _ = w.Write([]byte{9, 0, 7, 'p', 'a', 'l', 'e', 't', 't', 'e'})

	if secType <= 2 {
		// single-entry palette, no data array
		_, _ = w.Write([]byte{10}) // element type TAG_Compound
		_ = binary.Write(w, binary.BigEndian, int32(1))
		_, _ = w.Write([]byte{8, 0, 4, 'N', 'a', 'm', 'e'})
		switch secType {
		case 0:
			writeNBTString(w, "minecraft:air")
		case 1:
			writeNBTString(w, "minecraft:deepslate")
		case 2:
			writeNBTString(w, "minecraft:stone")
		}
		_, _ = w.Write([]byte{0}) // end entry
	} else {
		// full 8-entry palette
		_, _ = w.Write([]byte{10}) // element type TAG_Compound
		_ = binary.Write(w, binary.BigEndian, int32(len(standardVoxelPalette)))
		for _, name := range standardVoxelPalette {
			_, _ = w.Write([]byte{8, 0, 4, 'N', 'a', 'm', 'e'})
			writeNBTString(w, name)
			_, _ = w.Write([]byte{0}) // end entry
		}

		// data TAG_Long_Array
		_, _ = w.Write([]byte{12, 0, 4, 'd', 'a', 't', 'a'})
		_ = binary.Write(w, binary.BigEndian, int32(len(data)))
		var encodedData [256 * 8]byte
		for i, val := range data {
			binary.BigEndian.PutUint64(encodedData[i*8:], uint64(val))
		}
		_, _ = w.Write(encodedData[:len(data)*8])
	}

	_, _ = w.Write([]byte{0}) // end block_states
	_, _ = w.Write([]byte{0}) // end section compound
}

// FastStreamChunkNBT writes a full 1.20+ chunk NBT directly to w without heap allocation.
// w should be a pooled zlib.Writer for maximum throughput.
func FastStreamChunkNBT(w io.Writer, chunkX, chunkZ int, secTypes [24]uint8, secData [24][256]int64) error {
	var packed [24 * 256]int64
	for section := range secData {
		copy(packed[section*256:(section+1)*256], secData[section][:])
	}
	return fastStreamChunkNBTPacked(w, chunkX, chunkZ, secTypes, packed[:])
}

func fastStreamChunkNBTPacked(w io.Writer, chunkX, chunkZ int, secTypes [24]uint8, secData []int64) error {
	initStaticByteTemplates()

	// Root compound (type=10, empty name "")
	if _, err := w.Write([]byte{10, 0, 0}); err != nil {
		return err
	}

	// DataVersion TAG_Int
	_, _ = w.Write([]byte{3, 0, 11, 'D', 'a', 't', 'a', 'V', 'e', 'r', 's', 'i', 'o', 'n'})
	_ = binary.Write(w, binary.BigEndian, int32(DefaultDataVersion))

	// xPos TAG_Int
	_, _ = w.Write([]byte{3, 0, 4, 'x', 'P', 'o', 's'})
	_ = binary.Write(w, binary.BigEndian, int32(chunkX))

	// zPos TAG_Int
	_, _ = w.Write([]byte{3, 0, 4, 'z', 'P', 'o', 's'})
	_ = binary.Write(w, binary.BigEndian, int32(chunkZ))

	// yPos TAG_Int = -4
	_, _ = w.Write([]byte{3, 0, 4, 'y', 'P', 'o', 's', 0xff, 0xff, 0xff, 0xfc})

	// Status TAG_String = "minecraft:full"
	_, _ = w.Write([]byte{8, 0, 6, 'S', 't', 'a', 't', 'u', 's', 0, 14})
	_, _ = io.WriteString(w, "minecraft:full")

	// sections TAG_List of 24 TAG_Compound
	_, _ = w.Write([]byte{9, 0, 8, 's', 'e', 'c', 't', 'i', 'o', 'n', 's', 10, 0, 0, 0, 24})

	// Write all 24 sections
	for s := 0; s < 24; s++ {
		secY := int8(MinSectionY + s)
		stype := secTypes[s]

		switch {
		case stype == 0:
			if err := writeStaticSection(w, staticAirSectionBytes, secY); err != nil {
				return err
			}

		case stype == 1:
			if err := writeStaticSection(w, staticDeepslateSectionBytes, secY); err != nil {
				return err
			}

		case stype == 2:
			if err := writeStaticSection(w, staticStoneSectionBytes, secY); err != nil {
				return err
			}

		default:
			start := s * 256
			writeSectionNBT(w, secY, 3, secData[start:start+256])
		}
	}

	// PostProcessing TAG_List of 24 empty TAG_Short lists
	_, _ = w.Write([]byte{9, 0, 14})
	_, _ = io.WriteString(w, "PostProcessing")
	_, _ = w.Write([]byte{9, 0, 0, 0, 24}) // element type=9 (list), length=24
	for i := 0; i < 24; i++ {
		_, _ = w.Write([]byte{2, 0, 0, 0, 0}) // inner list: element type=2 (short), length=0
	}

	// structures compound
	_, _ = w.Write([]byte{10, 0, 10})
	_, _ = io.WriteString(w, "structures")
	_, _ = w.Write([]byte{10, 0, 6})
	_, _ = io.WriteString(w, "starts")
	_, _ = w.Write([]byte{0}) // end starts
	_, _ = w.Write([]byte{10, 0, 10})
	_, _ = io.WriteString(w, "References")
	_, _ = w.Write([]byte{0}) // end References
	_, _ = w.Write([]byte{0}) // end structures

	// block_entities empty list
	_, _ = w.Write([]byte{9, 0, 14})
	_, _ = io.WriteString(w, "block_entities")
	_, _ = w.Write([]byte{10, 0, 0, 0, 0})

	// block_ticks empty list
	_, _ = w.Write([]byte{9, 0, 11})
	_, _ = io.WriteString(w, "block_ticks")
	_, _ = w.Write([]byte{10, 0, 0, 0, 0})

	// fluid_ticks empty list
	_, _ = w.Write([]byte{9, 0, 11})
	_, _ = io.WriteString(w, "fluid_ticks")
	_, _ = w.Write([]byte{10, 0, 0, 0, 0})

	// End root compound
	_, _ = w.Write([]byte{0})
	return nil
}

// FastCompressGPUChunk streams NBT directly into pooled SIMD Zlib — zero BDF/NBT heap allocations
func FastCompressGPUChunk(chunkX, chunkZ int, secTypes [24]uint8, secData [24][256]int64) ([]byte, error) {
	var packed [24 * 256]int64
	for section := range secData {
		copy(packed[section*256:(section+1)*256], secData[section][:])
	}
	return FastCompressGPUChunkPacked(chunkX, chunkZ, secTypes, packed[:])
}

// FastCompressGPUChunkPacked consumes GPU section output without copying it.
func FastCompressGPUChunkPacked(chunkX, chunkZ int, secTypes [24]uint8, secData []int64) ([]byte, error) {
	pc := zlibPool.Get().(*pooledCompressor)
	defer zlibPool.Put(pc)

	pc.buf.Reset()
	pc.zw.Reset(pc.buf)

	if len(secData) != 24*256 {
		return nil, io.ErrUnexpectedEOF
	}
	if err := fastStreamChunkNBTPacked(pc.zw, chunkX, chunkZ, secTypes, secData); err != nil {
		_ = pc.zw.Close()
		return nil, err
	}

	if err := pc.zw.Close(); err != nil {
		return nil, err
	}

	res := make([]byte, pc.buf.Len())
	copy(res, pc.buf.Bytes())
	return res, nil
}

var zlibPool = sync.Pool{
	New: func() interface{} {
		var buf bytes.Buffer
		w, _ := zlib.NewWriterLevel(&buf, zlib.BestSpeed)
		return &pooledCompressor{
			buf: &buf,
			zw:  w,
		}
	},
}

type pooledCompressor struct {
	buf *bytes.Buffer
	zw  *zlib.Writer
}
