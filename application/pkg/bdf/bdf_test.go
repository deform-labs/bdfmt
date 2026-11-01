package bdf

import (
	"testing"
)

func createTestCompound() *TagCompound {
	comp := NewTagCompound()
	comp.Set("ByteVal", TagByte(42))
	comp.Set("ShortVal", TagShort(1234))
	comp.Set("IntVal", TagInt(987654))
	comp.Set("LongVal", TagLong(123456789012345))
	comp.Set("FloatVal", TagFloat(3.14159))
	comp.Set("DoubleVal", TagDouble(2.718281828459045))
	comp.Set("StringVal", TagString("Hello Minecraft!"))

	// Arrays
	comp.Set("ByteArray", TagByteArray([]byte{1, 2, 3, 4, 5, 255}))
	comp.Set("IntArray", TagIntArray([]int32{10, 20, 30, 40, -100}))
	comp.Set("LongArray", TagLongArray([]int64{10000000000, 20000000000, -30000000000}))

	// List of primitives
	comp.Set("DoubleList", &TagList{
		ElementType: TagTypeDouble,
		Elements: []Tag{
			TagDouble(0.5),
			TagDouble(64.0),
			TagDouble(-10.25),
		},
	})

	// Nested Compound
	nested := NewTagCompound()
	nested.Set("SubKey1", TagString("Nested Value"))
	nested.Set("SubKey2", TagInt(999))
	comp.Set("NestedCompound", nested)

	// List of Compounds (typical for Minecraft inventory)
	invItem1 := NewTagCompound()
	invItem1.Set("id", TagString("minecraft:diamond_sword"))
	invItem1.Set("Count", TagByte(1))
	invItem1.Set("Damage", TagShort(0))

	invItem2 := NewTagCompound()
	invItem2.Set("id", TagString("minecraft:apple"))
	invItem2.Set("Count", TagByte(64))

	comp.Set("Inventory", &TagList{
		ElementType: TagTypeCompound,
		Elements:    []Tag{invItem1, invItem2},
	})

	return comp
}

func TestRoundTripUncompressed(t *testing.T) {
	original := createTestCompound()

	encoded, err := Write(original, WriterOptions{Compression: CompressionNone})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	decodedTag, err := Read(encoded)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	decoded, ok := decodedTag.(*TagCompound)
	if !ok {
		t.Fatalf("Expected TagCompound, got %T", decodedTag)
	}

	if !original.Equal(decoded) {
		t.Fatalf("Decoded compound does not match original")
	}
}

func TestRoundTripZstd(t *testing.T) {
	original := createTestCompound()

	encoded, err := Write(original, WriterOptions{Compression: CompressionZstd})
	if err != nil {
		t.Fatalf("Write with Zstd failed: %v", err)
	}

	decodedTag, err := Read(encoded)
	if err != nil {
		t.Fatalf("Read with Zstd failed: %v", err)
	}

	decoded, ok := decodedTag.(*TagCompound)
	if !ok {
		t.Fatalf("Expected TagCompound, got %T", decodedTag)
	}

	if !original.Equal(decoded) {
		t.Fatalf("Decoded compound does not match original")
	}
}

func Test64BitAlignment(t *testing.T) {
	original := createTestCompound()
	encoded, err := Write(original, WriterOptions{Compression: CompressionNone})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	r, err := NewReader(encoded)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	// Verify root offset alignment
	if r.rootOffset%8 != 0 {
		t.Errorf("Root offset 0x%X is not 8-byte aligned!", r.rootOffset)
	}
}

func TestZeroCopyView(t *testing.T) {
	original := createTestCompound()
	encoded, err := Write(original, WriterOptions{Compression: CompressionNone})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	view, err := NewView(encoded)
	if err != nil {
		t.Fatalf("NewView failed: %v", err)
	}

	rootComp, err := view.RootCompound()
	if err != nil {
		t.Fatalf("RootCompound failed: %v", err)
	}

	// Test primitive lookups
	intVal, ok := rootComp.GetInt("IntVal")
	if !ok || intVal != 987654 {
		t.Errorf("Expected IntVal 987654, got %d (found=%v)", intVal, ok)
	}

	longVal, ok := rootComp.GetLong("LongVal")
	if !ok || longVal != 123456789012345 {
		t.Errorf("Expected LongVal 123456789012345, got %d (found=%v)", longVal, ok)
	}

	strVal, ok := rootComp.GetString("StringVal")
	if !ok || strVal != "Hello Minecraft!" {
		t.Errorf("Expected StringVal 'Hello Minecraft!', got %q (found=%v)", strVal, ok)
	}

	// Test nested compound zero-copy traversal
	nestedComp, ok := rootComp.GetCompound("NestedCompound")
	if !ok || nestedComp == nil {
		t.Fatalf("Failed to get NestedCompound view")
	}

	nestedSub, ok := nestedComp.GetString("SubKey1")
	if !ok || nestedSub != "Nested Value" {
		t.Errorf("Expected NestedCompound.SubKey1 'Nested Value', got %q (found=%v)", nestedSub, ok)
	}

	nestedInt, ok := nestedComp.GetInt("SubKey2")
	if !ok || nestedInt != 999 {
		t.Errorf("Expected NestedCompound.SubKey2 999, got %d (found=%v)", nestedInt, ok)
	}
}
