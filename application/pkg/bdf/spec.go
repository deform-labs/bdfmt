package bdf

// Magic header for Binary Data Format: 'B', 'D', 'F', '1'
var MagicHeader = [4]byte{'B', 'D', 'F', '1'}

const (
	// Current BDF Format Version
	CurrentVersion uint16 = 1

	// File Header Size in bytes (64 bytes for 64-byte CPU cache line alignment)
	HeaderSize = 64

	// Alignment constraint in bytes (64-bit word alignment)
	Alignment64 = 8
)

// Compression modes
const (
	CompressionNone uint16 = 0
	CompressionZstd uint16 = 1
	CompressionGzip uint16 = 2
)

// Header flags
const (
	FlagLittleEndian    uint16 = 1 << 0 // Always 1 for 64-bit optimized Little Endian
	FlagCompressionMask uint16 = 0x000E // Bits 1..3 indicate compression
	FlagStringInterned  uint16 = 1 << 4 // String table present
	FlagSortedCompound  uint16 = 1 << 5 // Compound entries sorted for binary search
)

// TagType IDs matching Minecraft NBT specification
type TagType uint8

const (
	TagTypeEnd       TagType = 0
	TagTypeByte      TagType = 1
	TagTypeShort     TagType = 2
	TagTypeInt       TagType = 3
	TagTypeLong      TagType = 4
	TagTypeFloat     TagType = 5
	TagTypeDouble    TagType = 6
	TagTypeByteArray TagType = 7
	TagTypeString    TagType = 8
	TagTypeList      TagType = 9
	TagTypeCompound  TagType = 10
	TagTypeIntArray  TagType = 11
	TagTypeLongArray TagType = 12
)

func (t TagType) String() string {
	switch t {
	case TagTypeEnd:
		return "TAG_End"
	case TagTypeByte:
		return "TAG_Byte"
	case TagTypeShort:
		return "TAG_Short"
	case TagTypeInt:
		return "TAG_Int"
	case TagTypeLong:
		return "TAG_Long"
	case TagTypeFloat:
		return "TAG_Float"
	case TagTypeDouble:
		return "TAG_Double"
	case TagTypeByteArray:
		return "TAG_Byte_Array"
	case TagTypeString:
		return "TAG_String"
	case TagTypeList:
		return "TAG_List"
	case TagTypeCompound:
		return "TAG_Compound"
	case TagTypeIntArray:
		return "TAG_Int_Array"
	case TagTypeLongArray:
		return "TAG_Long_Array"
	default:
		return "TAG_Unknown"
	}
}

// AlignTo64 aligns an offset to the next 8-byte boundary
func AlignTo64(offset uint64) uint64 {
	return (offset + 7) &^ 7
}

// AlignPad returns the number of zero padding bytes needed to reach the next 8-byte boundary
func AlignPad(offset uint64) int {
	aligned := AlignTo64(offset)
	return int(aligned - offset)
}
