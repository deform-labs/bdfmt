package nbt

import (
	"testing"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
)

func TestNBTRoundTrip(t *testing.T) {
	comp := bdf.NewTagCompound()
	comp.Set("health", bdf.TagFloat(20.0))
	comp.Set("name", bdf.TagString("Player1"))
	comp.Set("score", bdf.TagInt(1500))
	comp.Set("pos", &bdf.TagList{
		ElementType: bdf.TagTypeDouble,
		Elements: []bdf.Tag{
			bdf.TagDouble(100.5),
			bdf.TagDouble(64.0),
			bdf.TagDouble(-250.75),
		},
	})

	// Test uncompressed
	uncompressedData, err := Write("Data", comp, CompressionNone)
	if err != nil {
		t.Fatalf("NBT Write uncompressed failed: %v", err)
	}

	name, readTag, err := Read(uncompressedData)
	if err != nil {
		t.Fatalf("NBT Read uncompressed failed: %v", err)
	}
	if name != "Data" {
		t.Errorf("Expected root name 'Data', got %q", name)
	}
	if !comp.Equal(readTag) {
		t.Errorf("Read tag did not match original")
	}

	// Test Gzip compressed
	gzipData, err := Write("Data", comp, CompressionGzip)
	if err != nil {
		t.Fatalf("NBT Write gzip failed: %v", err)
	}

	nameGz, readTagGz, err := Read(gzipData)
	if err != nil {
		t.Fatalf("NBT Read gzip failed: %v", err)
	}
	if nameGz != "Data" {
		t.Errorf("Expected root name 'Data', got %q", nameGz)
	}
	if !comp.Equal(readTagGz) {
		t.Errorf("Read tag gzip did not match original")
	}
}
