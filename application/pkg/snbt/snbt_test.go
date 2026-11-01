package snbt

import (
	"testing"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
)

func TestSNBTRoundTrip(t *testing.T) {
	inputSNBT := `{Count: 1b, Damage: 0s, id: "minecraft:diamond_sword", tag: {Enchantments: [{id: "minecraft:sharpness", lvl: 5s}], Unbreakable: 1b}}`

	parsed, err := Parse(inputSNBT)
	if err != nil {
		t.Fatalf("Failed to parse SNBT: %v", err)
	}

	comp, ok := parsed.(*bdf.TagCompound)
	if !ok {
		t.Fatalf("Expected TagCompound, got %T", parsed)
	}

	count, ok := comp.GetByte("Count")
	if !ok || count != 1 {
		t.Errorf("Expected Count=1b, got %d", count)
	}

	id, ok := comp.GetString("id")
	if !ok || id != "minecraft:diamond_sword" {
		t.Errorf("Expected id='minecraft:diamond_sword', got %q", id)
	}

	tagSub, ok := comp.GetCompound("tag")
	if !ok || tagSub == nil {
		t.Fatalf("Expected 'tag' compound")
	}

	unbreakable, ok := tagSub.GetByte("Unbreakable")
	if !ok || unbreakable != 1 {
		t.Errorf("Expected Unbreakable=1b, got %d", unbreakable)
	}

	// Test re-serializing
	printed := Print(parsed, false)
	reparsed, err := Parse(printed)
	if err != nil {
		t.Fatalf("Failed to re-parse printed SNBT: %v\nPrinted:\n%s", err, printed)
	}

	if !parsed.Equal(reparsed) {
		t.Errorf("Re-parsed tag does not match original parsed tag")
	}
}

func TestSNBTArrays(t *testing.T) {
	inputSNBT := `{bytes: [B; 1b, 2b, -5b], ints: [I; 100, 200, 300], longs: [L; 1000L, 2000L]}`

	parsed, err := Parse(inputSNBT)
	if err != nil {
		t.Fatalf("Failed to parse arrays: %v", err)
	}

	comp := parsed.(*bdf.TagCompound)
	bytesTag, _ := comp.Get("bytes")
	if bArr, ok := bytesTag.(bdf.TagByteArray); !ok || len(bArr) != 3 {
		t.Errorf("Unexpected byte array: %v", bytesTag)
	}

	intsTag, _ := comp.Get("ints")
	if iArr, ok := intsTag.(bdf.TagIntArray); !ok || len(iArr) != 3 {
		t.Errorf("Unexpected int array: %v", intsTag)
	}

	longsTag, _ := comp.Get("longs")
	if lArr, ok := longsTag.(bdf.TagLongArray); !ok || len(lArr) != 2 {
		t.Errorf("Unexpected long array: %v", longsTag)
	}
}
