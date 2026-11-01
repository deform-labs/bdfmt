package worldgen

import (
	"bytes"
	"testing"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
	"github.com/twlve-secs/bdfmt/pkg/nbt"
)

func TestFastStreamChunkNBT(t *testing.T) {
	var secTypes [24]uint8
	var secData [24][256]int64

	secTypes[0] = 3  // mixed
	secTypes[1] = 1  // deepslate
	secTypes[20] = 0 // air

	for i := 0; i < 256; i++ {
		secData[0][i] = int64(i * 12345)
	}

	var buf bytes.Buffer
	if err := FastStreamChunkNBT(&buf, 10, -20, secTypes, secData); err != nil {
		t.Fatalf("FastStreamChunkNBT failed: %v", err)
	}

	rawBytes := buf.Bytes()
	if len(rawBytes) == 0 {
		t.Fatalf("FastStreamChunkNBT wrote 0 bytes")
	}

	// Verify NBT parser can read it back cleanly
	_, tag, err := nbt.Read(rawBytes)
	if err != nil {
		t.Fatalf("Failed to parse FastStreamChunkNBT output with standard NBT reader: %v", err)
	}

	comp, ok := tag.(*bdf.TagCompound)
	if !ok {
		t.Fatalf("Expected TagCompound, got %T", tag)
	}

	xPos, ok := comp.GetInt("xPos")
	if !ok || xPos != 10 {
		t.Errorf("Expected xPos 10, got %d", xPos)
	}

	zPos, ok := comp.GetInt("zPos")
	if !ok || zPos != -20 {
		t.Errorf("Expected zPos -20, got %d", zPos)
	}

	sections, ok := comp.GetList("sections")
	if !ok || len(sections.Elements) != 24 {
		t.Fatalf("Expected 24 sections, got %d", len(sections.Elements))
	}

	// Compare section zero against the same Y value used by the chunk writer.
	// The minimum section Y is -4 (0xfc); comparing against Y=0 creates a false
	// mismatch even though the serialized section is correct.
	sectionListHeader := []byte{9, 0, 8, 's', 'e', 'c', 't', 'i', 'o', 'n', 's', 10, 0, 0, 0, 24}
	sectionStart := bytes.Index(rawBytes, sectionListHeader)
	if sectionStart < 0 {
		t.Fatal("Could not find sections list in serialized chunk")
	}
	sectionStart += len(sectionListHeader)
	var expectedSection bytes.Buffer
	writeSectionNBT(&expectedSection, int8(MinSectionY), 3, secData[0][:])
	sectionEnd := sectionStart + expectedSection.Len()
	if sectionEnd > len(rawBytes) {
		t.Fatal("Serialized chunk ended before the first section was complete")
	}
	if got := rawBytes[sectionStart:sectionEnd]; !bytes.Equal(got, expectedSection.Bytes()) {
		t.Fatal("First serialized section does not match the reference section at MinSectionY")
	}
}
