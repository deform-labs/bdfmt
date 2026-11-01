package region

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
)

func TestRegionReadWrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "region_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	reg, err := Create(tempDir, 0, 0)
	if err != nil {
		t.Fatalf("Failed to create region: %v", err)
	}

	// Create test chunk compound
	chunkTag := bdf.NewTagCompound()
	chunkTag.Set("xPos", bdf.TagInt(5))
	chunkTag.Set("zPos", bdf.TagInt(10))
	chunkTag.Set("Status", bdf.TagString("minecraft:full"))

	// Write chunk (5, 10)
	if err := reg.WriteChunkBDF(5, 10, chunkTag); err != nil {
		t.Fatalf("WriteChunkBDF failed: %v", err)
	}

	// Write another chunk (0, 0)
	chunk0 := bdf.NewTagCompound()
	chunk0.Set("xPos", bdf.TagInt(0))
	chunk0.Set("zPos", bdf.TagInt(0))
	if err := reg.WriteChunkBDF(0, 0, chunk0); err != nil {
		t.Fatalf("WriteChunkBDF (0, 0) failed: %v", err)
	}

	if err := reg.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reopen and read back
	reopened, err := Create(tempDir, 0, 0)
	if err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}
	defer reopened.Close()

	readTag, err := reopened.ReadChunkBDF(5, 10)
	if err != nil {
		t.Fatalf("ReadChunkBDF failed: %v", err)
	}
	if readTag == nil {
		t.Fatalf("ReadChunkBDF returned nil")
	}

	comp, ok := readTag.(*bdf.TagCompound)
	if !ok {
		t.Fatalf("Expected TagCompound, got %T", readTag)
	}

	status, ok := comp.GetString("Status")
	if !ok || status != "minecraft:full" {
		t.Errorf("Expected Status 'minecraft:full', got %q", status)
	}

	// Verify ungenerated chunk returns nil
	emptyChunk, err := reopened.ReadChunkBDF(1, 1)
	if err != nil {
		t.Fatalf("Read ungenerated chunk error: %v", err)
	}
	if emptyChunk != nil {
		t.Errorf("Expected nil for ungenerated chunk, got %v", emptyChunk)
	}

	// Check file size is multiple of 4096 (sectors)
	mcaPath := filepath.Join(tempDir, RegionFileName(0, 0))
	info, err := os.Stat(mcaPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Size()%SectorSize != 0 {
		t.Errorf("Region file size %d is not a multiple of %d", info.Size(), SectorSize)
	}
}
