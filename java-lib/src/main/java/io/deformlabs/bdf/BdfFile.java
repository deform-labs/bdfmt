package io.deformlabs.bdf;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.CodingErrorAction;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.OptionalDouble;
import java.util.OptionalInt;
import java.util.OptionalLong;
import java.util.zip.GZIPInputStream;

/**
 * Read-only, random-access reader for BDF v1 files.
 *
 * <p>Compound values are exposed as views into the file data; the reader does not
 * build a tree of Java objects. Zstandard-compressed files require an external
 * decoder and are not supported by this dependency-free implementation.
 */
public final class BdfFile {
    private static final int HEADER_SIZE = 64;
    private static final int FLAG_LITTLE_ENDIAN = 1;
    private static final int FLAG_COMPRESSION_MASK = 0x000e;
    private static final int FLAG_STRING_INTERNED = 1 << 4;
    private static final int FLAG_SORTED_COMPOUND = 1 << 5;

    private final ByteBuffer data;
    private final String[] strings;
    private final long rootOffset;

    private BdfFile(ByteBuffer data, String[] strings, long rootOffset) {
        this.data = data.asReadOnlyBuffer().order(ByteOrder.LITTLE_ENDIAN);
        this.strings = strings;
        this.rootOffset = rootOffset;
    }

    /** Opens a BDF file from disk. */
    public static BdfFile open(Path path) throws IOException {
        return fromBytes(Files.readAllBytes(path));
    }

    /** Parses BDF bytes. Input bytes are copied so later caller mutations are safe. */
    public static BdfFile fromBytes(byte[] bytes) {
        if (bytes == null || bytes.length < HEADER_SIZE) {
            throw new BdfFormatException("data too short for BDF header");
        }

        ByteBuffer header = ByteBuffer.wrap(bytes).order(ByteOrder.LITTLE_ENDIAN);
        if (bytes[0] != 'B' || bytes[1] != 'D' || bytes[2] != 'F' || bytes[3] != '1') {
            throw new BdfFormatException("invalid BDF magic");
        }
        int version = Short.toUnsignedInt(header.getShort(4));
        if (version != 1) {
            throw new BdfFormatException("unsupported BDF version: " + version);
        }

        int flags = Short.toUnsignedInt(header.getShort(6));
        if ((flags & FLAG_LITTLE_ENDIAN) == 0) {
            throw new BdfFormatException("BDF little-endian flag is not set");
        }
        int compression = (flags & FLAG_COMPRESSION_MASK) >>> 1;
        long rootOffset = header.getLong(8);
        long stringTableOffset = header.getLong(16);
        long stringTableSize = header.getLong(24);
        long uncompressedSize = header.getLong(32);

        byte[] raw = switch (compression) {
            case 0 -> bytes.clone();
            case 1 -> throw new BdfFormatException(
                    "Zstandard-compressed BDF files are not supported");
            case 2 -> decompressGzip(bytes, uncompressedSize);
            default -> throw new BdfFormatException(
                    "unsupported BDF compression mode: " + compression);
        };
        if (raw.length < HEADER_SIZE) {
            throw new BdfFormatException("decompressed BDF data is shorter than its header");
        }

        ByteBuffer data = ByteBuffer.wrap(raw).asReadOnlyBuffer().order(ByteOrder.LITTLE_ENDIAN);
        if (uncompressedSize != 0 && uncompressedSize != raw.length) {
            throw new BdfFormatException("uncompressed size does not match the BDF payload");
        }
        if (rootOffset % 8 != 0) {
            throw new BdfFormatException("root tag offset is not 8-byte aligned");
        }
        if ((flags & FLAG_STRING_INTERNED) == 0) {
            throw new BdfFormatException("BDF string table flag is not set");
        }

        String[] strings = parseStringTable(data, stringTableOffset, stringTableSize);
        BdfFile file = new BdfFile(data, strings, rootOffset);
        file.compoundAt(rootOffset);
        return file;
    }

    /** Returns a view of the root compound. */
    public Compound root() {
        return compoundAt(rootOffset);
    }

    private static byte[] decompressGzip(byte[] bytes, long expectedSize) {
        if (expectedSize < HEADER_SIZE || expectedSize > Integer.MAX_VALUE) {
            throw new BdfFormatException("invalid uncompressed BDF size: " + expectedSize);
        }
        int expectedPayload = (int) expectedSize - HEADER_SIZE;
        try (InputStream input = new GZIPInputStream(
                new ByteArrayInputStream(bytes, HEADER_SIZE, bytes.length - HEADER_SIZE));
                ByteArrayOutputStream output = new ByteArrayOutputStream(expectedPayload)) {
            byte[] chunk = new byte[8192];
            int total = 0;
            for (int count; (count = input.read(chunk)) != -1;) {
                if (count > expectedPayload - total) {
                    throw new BdfFormatException("decompressed payload exceeds declared size");
                }
                output.write(chunk, 0, count);
                total += count;
            }
            if (total != expectedPayload) {
                throw new BdfFormatException("decompressed payload size does not match header");
            }
            byte[] raw = new byte[(int) expectedSize];
            System.arraycopy(bytes, 0, raw, 0, HEADER_SIZE);
            System.arraycopy(output.toByteArray(), 0, raw, HEADER_SIZE, expectedPayload);
            return raw;
        } catch (IOException e) {
            throw new BdfFormatException("failed to decompress gzip BDF payload", e);
        }
    }

    private static String[] parseStringTable(ByteBuffer data, long offset, long tableSize) {
        requireRange(data, offset, 8, "string table header");
        if (tableSize < 8) {
            throw new BdfFormatException("invalid string table size");
        }
        long tableEnd = checkedEnd(offset, tableSize, "string table");
        requireRange(data, offset, tableSize, "string table");
        long count = Integer.toUnsignedLong(data.getInt((int) offset));
        long totalBytes = Integer.toUnsignedLong(data.getInt((int) offset + 4));
        long dataStart = offset + 8 + count * 8;
        if (dataStart > tableEnd || totalBytes > tableEnd - dataStart) {
            throw new BdfFormatException("string table contents are out of bounds");
        }
        if (count > Integer.MAX_VALUE) {
            throw new BdfFormatException("string table has too many entries");
        }

        String[] strings = new String[(int) count];
        long offsetsStart = offset + 8;
        long lengthsStart = offsetsStart + count * 4;
        for (int i = 0; i < strings.length; i++) {
            long stringOffset = Integer.toUnsignedLong(data.getInt((int) offsetsStart + i * 4));
            long stringLength = Integer.toUnsignedLong(data.getInt((int) lengthsStart + i * 4));
            if (stringOffset > totalBytes || stringLength > totalBytes - stringOffset) {
                throw new BdfFormatException("string table entry " + i + " is out of bounds");
            }
            int start = (int) (dataStart + stringOffset);
            int length = (int) stringLength;
            try {
                strings[i] = StandardCharsets.UTF_8.newDecoder()
                        .onMalformedInput(CodingErrorAction.REPORT)
                        .onUnmappableCharacter(CodingErrorAction.REPORT)
                        .decode(data.duplicate().position(start).limit(start + length))
                        .toString();
            } catch (CharacterCodingException e) {
                throw new BdfFormatException("invalid UTF-8 in string table entry " + i, e);
            }
        }
        return strings;
    }

    private Compound compoundAt(long offset) {
        requireRange(data, offset, 8, "compound header");
        long count = Integer.toUnsignedLong(data.getInt((int) offset));
        long tableStart = offset + 8;
        requireRange(data, tableStart, count * 16, "compound entries");
        int flags = data.getInt((int) offset + 4);
        return new Compound(this, tableStart, (int) count,
                (flags & 1) != 0 || (this.flags() & FLAG_SORTED_COMPOUND) != 0);
    }

    private int flags() {
        return Short.toUnsignedInt(data.getShort(6));
    }

    private static long checkedEnd(long offset, long length, String description) {
        if (offset < 0 || length < 0 || offset > Long.MAX_VALUE - length) {
            throw new BdfFormatException(description + " range overflows");
        }
        return offset + length;
    }

    private static void requireRange(ByteBuffer data, long offset, long length, String description) {
        if (offset < 0 || length < 0 || offset > data.limit() || length > data.limit() - offset) {
            throw new BdfFormatException(description + " is out of bounds");
        }
    }

    private int stringId(long id, String description) {
        if (id < 0 || id >= strings.length) {
            throw new BdfFormatException("invalid string ID for " + description + ": " + id);
        }
        return (int) id;
    }

    private static int compareCodePoints(String left, String right) {
        int leftIndex = 0;
        int rightIndex = 0;
        while (leftIndex < left.length() && rightIndex < right.length()) {
            int leftCodePoint = left.codePointAt(leftIndex);
            int rightCodePoint = right.codePointAt(rightIndex);
            if (leftCodePoint != rightCodePoint) {
                return Integer.compare(leftCodePoint, rightCodePoint);
            }
            leftIndex += Character.charCount(leftCodePoint);
            rightIndex += Character.charCount(rightCodePoint);
        }
        return Integer.compare(left.length() - leftIndex, right.length() - rightIndex);
    }

    private record Entry(int type, int inline, long value) {}

    /** Zero-copy view over a compound tag. */
    public static final class Compound {
        private final BdfFile file;
        private final long tableStart;
        private final int count;
        private final boolean sorted;

        private Compound(BdfFile file, long tableStart, int count, boolean sorted) {
            this.file = file;
            this.tableStart = tableStart;
            this.count = count;
            this.sorted = sorted;
        }

        public int size() {
            return count;
        }

        public List<String> keys() {
            List<String> keys = new ArrayList<>(count);
            for (int i = 0; i < count; i++) {
                keys.add(keyAt(i));
            }
            return List.copyOf(keys);
        }

        public boolean contains(String key) {
            return find(key) != null;
        }

        public OptionalInt getByte(String key) {
            Entry entry = find(key);
            return entry != null && entry.type == 1 && entry.inline == 1
                    ? OptionalInt.of((byte) entry.value) : OptionalInt.empty();
        }

        public OptionalInt getShort(String key) {
            Entry entry = find(key);
            return entry != null && entry.type == 2 && entry.inline == 1
                    ? OptionalInt.of((short) entry.value) : OptionalInt.empty();
        }

        public OptionalInt getInt(String key) {
            Entry entry = find(key);
            return entry != null && entry.type == 3 && entry.inline == 1
                    ? OptionalInt.of((int) entry.value) : OptionalInt.empty();
        }

        public OptionalLong getLong(String key) {
            Entry entry = find(key);
            return entry != null && entry.type == 4 && entry.inline == 1
                    ? OptionalLong.of(entry.value) : OptionalLong.empty();
        }

        public Optional<Float> getFloat(String key) {
            Entry entry = find(key);
            return entry != null && entry.type == 5 && entry.inline == 1
                    ? Optional.of(Float.intBitsToFloat((int) entry.value)) : Optional.empty();
        }

        public OptionalDouble getDouble(String key) {
            Entry entry = find(key);
            return entry != null && entry.type == 6 && entry.inline == 1
                    ? OptionalDouble.of(Double.longBitsToDouble(entry.value)) : OptionalDouble.empty();
        }

        public Optional<String> getString(String key) {
            Entry entry = find(key);
            if (entry == null || entry.type != 8 || entry.inline != 1) {
                return Optional.empty();
            }
            return Optional.of(file.strings[file.stringId(
                    Integer.toUnsignedLong((int) entry.value), key)]);
        }

        public Optional<Compound> getCompound(String key) {
            Entry entry = find(key);
            if (entry == null || entry.type != 10 || entry.inline != 0) {
                return Optional.empty();
            }
            return Optional.of(file.compoundAt(entry.value));
        }

        /** Returns a read-only little-endian view of the byte-array contents. */
        public Optional<ByteBuffer> getByteArray(String key) {
            Entry entry = find(key);
            if (entry == null || entry.type != 7 || entry.inline != 0) {
                return Optional.empty();
            }
            long offset = entry.value;
            requireRange(file.data, offset, 8, "byte array header");
            long length = Integer.toUnsignedLong(file.data.getInt((int) offset));
            long start = offset + 8;
            requireRange(file.data, start, length, "byte array payload");
            ByteBuffer view = file.data.asReadOnlyBuffer();
            view.position((int) start).limit((int) (start + length));
            return Optional.of(view.slice().asReadOnlyBuffer());
        }

        public Optional<int[]> getIntArray(String key) {
            Entry entry = find(key);
            if (entry == null || entry.type != 11 || entry.inline != 0) {
                return Optional.empty();
            }
            long offset = entry.value;
            requireRange(file.data, offset, 8, "int array header");
            long length = Integer.toUnsignedLong(file.data.getInt((int) offset));
            long start = offset + 8;
            requireRange(file.data, start, length * 4, "int array payload");
            if (length > Integer.MAX_VALUE) {
                throw new BdfFormatException("int array is too large");
            }
            int[] values = new int[(int) length];
            for (int i = 0; i < values.length; i++) {
                values[i] = file.data.getInt((int) start + i * 4);
            }
            return Optional.of(values);
        }

        public Optional<long[]> getLongArray(String key) {
            Entry entry = find(key);
            if (entry == null || entry.type != 12 || entry.inline != 0) {
                return Optional.empty();
            }
            long offset = entry.value;
            requireRange(file.data, offset, 8, "long array header");
            long length = file.data.getLong((int) offset);
            if (length < 0) {
                throw new BdfFormatException("long array length exceeds supported size");
            }
            long start = offset + 8;
            requireRange(file.data, start, length * 8, "long array payload");
            if (length > Integer.MAX_VALUE) {
                throw new BdfFormatException("long array is too large");
            }
            long[] values = new long[(int) length];
            for (int i = 0; i < values.length; i++) {
                values[i] = file.data.getLong((int) start + i * 8);
            }
            return Optional.of(values);
        }

        public Optional<ListView> getList(String key) {
            Entry entry = find(key);
            if (entry == null || entry.type != 9 || entry.inline != 0) {
                return Optional.empty();
            }
            return Optional.of(new ListView(file, entry.value));
        }

        private String keyAt(int index) {
            long entryOffset = tableStart + (long) index * 16;
            long keyId = Integer.toUnsignedLong(file.data.getInt((int) entryOffset));
            return file.strings[file.stringId(keyId, "compound key")];
        }

        private Entry find(String key) {
            if (key == null) {
                throw new NullPointerException("key");
            }
            int low = 0;
            int high = count - 1;
            if (sorted) {
                while (low <= high) {
                    int mid = (low + high) >>> 1;
                    int entryOffset = (int) (tableStart + (long) mid * 16);
                    long keyId = Integer.toUnsignedLong(file.data.getInt(entryOffset));
                    String entryKey = file.strings[file.stringId(keyId, "compound key")];
                    int comparison = compareCodePoints(entryKey, key);
                    if (comparison < 0) {
                        low = mid + 1;
                    } else if (comparison > 0) {
                        high = mid - 1;
                    } else {
                        return entryAt(entryOffset);
                    }
                }
                return null;
            }
            for (int i = 0; i < count; i++) {
                int entryOffset = (int) (tableStart + (long) i * 16);
                long keyId = Integer.toUnsignedLong(file.data.getInt(entryOffset));
                if (file.strings[file.stringId(keyId, "compound key")].equals(key)) {
                    return entryAt(entryOffset);
                }
            }
            return null;
        }

        private Entry entryAt(int offset) {
            return new Entry(Byte.toUnsignedInt(file.data.get(offset + 4)),
                    Byte.toUnsignedInt(file.data.get(offset + 5)),
                    file.data.getLong(offset + 8));
        }
    }

    /** Zero-copy view over a BDF list. */
    public static final class ListView {
        private final BdfFile file;
        private final long offset;
        private final int elementType;
        private final int count;

        private ListView(BdfFile file, long offset) {
            requireRange(file.data, offset, 8, "list header");
            this.file = file;
            this.offset = offset;
            this.elementType = Byte.toUnsignedInt(file.data.get((int) offset));
            this.count = (int) Integer.toUnsignedLong(file.data.getInt((int) offset + 4));
            if (elementType == 0 && count == 0) {
                return;
            }
            long elementSize = switch (elementType) {
                case 1 -> 1;
                case 2 -> 2;
                case 3, 5, 8 -> 4;
                case 4, 6, 7, 9, 10, 11, 12 -> 8;
                default -> throw new BdfFormatException("invalid list element type: " + elementType);
            };
            requireRange(file.data, offset + 8, (long) count * elementSize, "list payload");
        }

        public int size() {
            return count;
        }

        public int elementType() {
            return elementType;
        }

        public OptionalInt getInt(int index) {
            checkIndex(index);
            int position = (int) offset + 8;
            return switch (elementType) {
                case 1 -> OptionalInt.of(file.data.get(position + index));
                case 2 -> OptionalInt.of(file.data.getShort(position + index * 2));
                case 3 -> OptionalInt.of(file.data.getInt(position + index * 4));
                default -> OptionalInt.empty();
            };
        }

        public OptionalLong getLong(int index) {
            checkIndex(index);
            return elementType == 4
                    ? OptionalLong.of(file.data.getLong((int) offset + 8 + index * 8))
                    : OptionalLong.empty();
        }

        public Optional<Float> getFloat(int index) {
            checkIndex(index);
            int position = (int) offset + 8;
            return switch (elementType) {
                case 5 -> Optional.of(Float.intBitsToFloat(file.data.getInt(position + index * 4)));
                default -> Optional.empty();
            };
        }

        public OptionalDouble getDouble(int index) {
            checkIndex(index);
            int position = (int) offset + 8;
            return switch (elementType) {
                case 6 -> OptionalDouble.of(file.data.getDouble(position + index * 8));
                default -> OptionalDouble.empty();
            };
        }

        public Optional<String> getString(int index) {
            checkIndex(index);
            if (elementType != 8) {
                return Optional.empty();
            }
            long id = Integer.toUnsignedLong(file.data.getInt((int) offset + 8 + index * 4));
            return Optional.of(file.strings[file.stringId(id, "list element")]);
        }

        public Optional<Compound> getCompound(int index) {
            checkIndex(index);
            if (elementType != 10) {
                return Optional.empty();
            }
            long childOffset = file.data.getLong((int) offset + 8 + index * 8);
            return Optional.of(file.compoundAt(childOffset));
        }

        private void checkIndex(int index) {
            if (index < 0 || index >= count) {
                throw new IndexOutOfBoundsException(index);
            }
        }
    }

    /** Indicates malformed or unsupported BDF data. */
    public static final class BdfFormatException extends IllegalArgumentException {
        private static final long serialVersionUID = 1L;

        public BdfFormatException(String message) {
            super(message);
        }

        public BdfFormatException(String message, Throwable cause) {
            super(message, cause);
        }
    }
}
