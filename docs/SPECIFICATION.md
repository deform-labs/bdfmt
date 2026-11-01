# BDF (Binary Data Format) Specification
**Version 1.0 — 64-bit Optimized Minecraft NBT Replacement**

---

## 1. Overview & Motivation

Minecraft's native data format, **NBT (Named Binary Tag)**, was designed for 32-bit Big-Endian Java architectures in 2010. On modern 64-bit Little-Endian hardware (x86_64, aarch64, riscv64), standard NBT suffers from severe performance bottlenecks:

1. **Big-Endian Penalty**: Every 16, 32, and 64-bit integer, float, and double requires `BSWAP` instructions.
2. **Unaligned Memory Access**: Fields are variable-length and packed tightly without padding. As a result, 64-bit words, SIMD vectors, and doubles cross 64-byte CPU cache lines and cause unaligned read penalties.
3. **Repeated UTF-8 Strings**: Compound tag keys (e.g., `"Pos"`, `"Motion"`, `"id"`, `"Count"`, `"BlockStates"`) are stored inline as raw strings every single time they appear.
4. **Sequential Traversal ($O(N)$)**: Compounds have no index or jump table. To read field `#50`, the reader must decode fields `1` through `49`.
5. **Heavy Object Allocations**: Reading NBT allocates thousands of small heap objects (`CompoundTag`, `IntTag`, etc.), placing massive pressure on the JVM garbage collector.

**BDF (Binary Data Format)** solves all five problems:
- **Native Little-Endian**: 0 CPU byte-swapping overhead on modern 64-bit systems.
- **Strict 8-Byte (64-bit) Word Alignment**: Every tag payload, composite structure, and array begins at an offset divisible by 8 (`offset % 8 == 0`).
- **Global String Interning Pool**: Tag names and repeated strings are deduplicated into a centralized string pool, converted into 32-bit integer IDs.
- **Direct 16-Byte Compound Entries with Inlined Primitives**: Each compound entry occupies 16 bytes (two 64-bit words). Primitive values (Byte, Short, Int, Long, Float, Double) and String IDs are stored directly inside the entry.
- **Zero-Copy & Memory-Mapping ($O(1)$ and $O(\log N)$ Access)**: Compounds are sorted by key name, allowing instant binary search or direct memory lookup without allocating intermediate heap objects.

---

## 2. File Layout & Header

A BDF file consists of:
```
+--------------------------------------------------------------+
| Header (64 Bytes, Cache-Line Aligned)                         |
+--------------------------------------------------------------+
| String Table (Interned Strings, Aligned to 8 Bytes)          |
+--------------------------------------------------------------+
| Tag Payloads (Root Compound & Nested Structures, Aligned 8B) |
+--------------------------------------------------------------+
```

### 2.1 File Header (64 Bytes)
The file header is exactly 64 bytes to align with modern CPU L1/L2 cache lines:

| Offset | Size (Bytes) | Type | Name | Description |
|---|---|---|---|---|
| `0x00` | 4 | `byte[4]` | `Magic` | ASCII `'B', 'D', 'F', '1'` (`0x31464442` in LE) |
| `0x04` | 2 | `uint16` | `Version` | Current format version: `1` |
| `0x06` | 2 | `uint16` | `Flags` | Bitfield flags (see below) |
| `0x08` | 8 | `uint64` | `RootOffset` | File offset to the root tag (8-byte aligned) |
| `0x10` | 8 | `uint64` | `StringTableOffset` | File offset to String Pool (starts at `0x40`) |
| `0x18` | 8 | `uint64` | `StringTableSize` | Size of String Pool in bytes |
| `0x20` | 8 | `uint64` | `UncompressedSize` | Total uncompressed file size |
| `0x28` | 8 | `uint64` | `TotalTags` | Total count of tags in the file |
| `0x30` | 16 | `byte[16]` | `Reserved` | Reserved for future extensions (all zeros) |

### 2.2 Header Flags
- **Bit 0 (`0x0001`)**: `FlagLittleEndian` — Always 1 for native 64-bit architectures.
- **Bits 1..3 (`0x000E`)**: `CompressionMode`:
  - `0`: None (uncompressed, instant `mmap` zero-copy ready).
  - `1`: Zstandard (`zstd`).
  - `2`: Gzip.
- **Bit 4 (`0x0010`)**: `FlagStringInterned` — String Table is present.
- **Bit 5 (`0x0020`)**: `FlagSortedCompound` — Compound entries are sorted alphabetically for binary search.

---

## 3. String Table (Deduplication Pool)

Placed immediately after the 64-byte header (`StringTableOffset = 0x40`):

```
+-------------------------------------------------------+
| Count (uint32)                                        | 4 bytes
| TotalByteLength (uint32)                              | 4 bytes
| StringOffsets (Count * uint32)                        | Count * 4 bytes
| StringLengths (Count * uint32)                        | Count * 4 bytes
| UTF-8 Bytes Pool                                      | TotalByteLength
| 8-Byte Alignment Padding                              | 0..7 bytes
+-------------------------------------------------------+
```

Every string (compound keys and string tag values) is stored once. Any reference to a string is an index (`uint32`) into this table.

---

## 4. Tag System & Memory Layout

Tag Types match the Minecraft NBT data model:

| Tag ID | Type Name | Go Type | Java Type |
|---|---|---|---|
| `0` | `TAG_End` | - | - |
| `1` | `TAG_Byte` | `int8` | `byte` |
| `2` | `TAG_Short` | `int16` | `short` |
| `3` | `TAG_Int` | `int32` | `int` |
| `4` | `TAG_Long` | `int64` | `long` |
| `5` | `TAG_Float` | `float32` | `float` |
| `6` | `TAG_Double` | `float64` | `double` |
| `7` | `TAG_Byte_Array` | `[]byte` | `byte[]` |
| `8` | `TAG_String` | `string` | `String` |
| `9` | `TAG_List` | `[]Tag` | `ListTag` |
| `10` | `TAG_Compound` | `map[string]Tag` | `CompoundTag` |
| `11` | `TAG_Int_Array` | `[]int32` | `int[]` |
| `12` | `TAG_Long_Array` | `[]int64` | `long[]` |

---

## 5. Compound Tag Structure (Fast Lookups)

In BDF, a Compound Tag is an aligned table of 16-byte descriptors:

```
Compound Header (8 bytes):
  Count  : uint32
  Flags  : uint32 (bit 0 = 1 for sorted)

Entry Descriptors (Count * 16 bytes):
  +------------------+---------+------------+----------+--------------------+
  | KeyID (uint32)   | Type(u8)| Inline(u8) | Pad(u16) | ValueOrOffset(u64) |
  | (4 bytes)        | (1 byte)| (1 byte)   | (2 bytes)| (8 bytes)          |
  +------------------+---------+------------+----------+--------------------+
```

### Inlining Rules:
1. **Primitives & Interned Strings**: If `Inline == 1`:
   - `TAG_Byte`: Lower 8 bits of `ValueOrOffset`.
   - `TAG_Short`: Lower 16 bits of `ValueOrOffset`.
   - `TAG_Int`: Lower 32 bits of `ValueOrOffset`.
   - `TAG_Long`: Full 64 bits of `ValueOrOffset`.
   - `TAG_Float`: IEEE-754 32-bit float in lower 32 bits.
   - `TAG_Double`: IEEE-754 64-bit double in `ValueOrOffset`.
   - `TAG_String`: `uint32` String ID stored in lower 32 bits.
   *(Result: 7 out of 12 tag types require ZERO memory dereferences!)*
2. **Composites**: If `Inline == 0`:
   - `ValueOrOffset` contains the 64-bit absolute file offset to the payload (`TAG_List`, `TAG_Compound`, `TAG_Byte_Array`, `TAG_Int_Array`, `TAG_Long_Array`).

---

## 6. Arrays and Lists

- **`TAG_Long_Array`**:
  - `Length`: `uint64`
  - Elements: `Length * int64` directly packed and 8-byte aligned. Allows direct 64-bit memory loading or AVX2 / AVX-512 vectorization.
- **`TAG_Int_Array`**:
  - `Length`: `uint32`, 4 bytes padding (for 8-byte alignment)
  - Elements: `Length * int32` directly packed.
- **`TAG_Byte_Array`**:
  - `Length`: `uint32`, 4 bytes padding
  - Elements: `Length * byte`, followed by 0..7 padding bytes.
- **`TAG_List`**:
  - `ElemType` (1 byte), 3 bytes padding, `Count` (`uint32`). Total 8 bytes.
  - If elements are primitives (e.g. `TAG_Double` for Minecraft `Pos: [x, y, z]`): packed directly as an array of 64-bit floats.
  - If elements are composites (e.g. List of item Compounds): array of `Count * uint64` offsets.

---

## 7. Minecraft Mod Integration Guide (Fabric / NeoForge)

To replace Minecraft's NBT files (`level.dat`, `player.dat`, region chunks `.mca`) with BDF:

### 7.1 Architecture Replacement
In Minecraft's codebase:
- Replace `net.minecraft.nbt.NbtIo.readCompressed(File)` with `BdfIo.read(Path)`
- Replace `net.minecraft.nbt.NbtIo.writeCompressed(CompoundTag, File)` with `BdfIo.write(BdfCompound, Path)`

### 7.2 Java Fast Access with `MemorySegment` / `ByteBuffer`
Using Java 21+ Foreign Function & Memory API (Panama) or `ByteBuffer`:

```java
public class BdfCompoundView {
    private final MemorySegment segment;
    private final long tableStart;
    private final int count;
    private final String[] stringTable;

    public int getInt(String key) {
        long entry = binarySearch(key);
        if (entry == -1) return 0;
        // Read 64-bit word at offset + 8
        return (int) segment.get(ValueLayout.JAVA_LONG_UNALIGNED, entry + 8);
    }

    public double getDouble(String key) {
        long entry = binarySearch(key);
        if (entry == -1) return 0.0;
        return Double.longBitsToDouble(segment.get(ValueLayout.JAVA_LONG_UNALIGNED, entry + 8));
    }
}
```

Because BDF is natively Little-Endian and 64-bit aligned, Java reads primitives directly from the buffer with zero byte-swapping and zero intermediate object allocations.
