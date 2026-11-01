package worldgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
	"github.com/twlve-secs/bdfmt/pkg/nbt"
)

func TestPerlinNoiseDeterministic(t *testing.T) {
	n1 := NewPerlinNoise(42)
	n2 := NewPerlinNoise(42)

	val1 := n1.Noise2D(12.345, 67.89)
	val2 := n2.Noise2D(12.345, 67.89)

	if val1 != val2 {
		t.Errorf("Perlin noise not deterministic with same seed: %f vs %f", val1, val2)
	}
}

func TestChunkBuilder(t *testing.T) {
	chunk := NewChunkData(3, -7, "minecraft:plains")

	// Set some blocks
	chunk.SetBlock(0, -64, 0, "minecraft:bedrock")
	chunk.SetBlock(5, 64, 5, "minecraft:grass_block")
	chunk.SetBlock(5, 63, 5, "minecraft:dirt")

	tag := chunk.BuildChunkTag()
	if tag == nil {
		t.Fatalf("BuildChunkTag returned nil")
	}

	xPos, ok := tag.GetInt("xPos")
	if !ok || xPos != 3 {
		t.Errorf("Expected xPos 3, got %d", xPos)
	}

	zPos, ok := tag.GetInt("zPos")
	if !ok || zPos != -7 {
		t.Errorf("Expected zPos -7, got %d", zPos)
	}

	sections, ok := tag.GetList("sections")
	if !ok || len(sections.Elements) != TotalSections {
		t.Fatalf("Expected %d sections, got %d", TotalSections, len(sections.Elements))
	}
}

func TestLevelDat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "level_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	opts := DefaultLevelOptions("TestWorld", 9999)
	if err := GenerateLevelDat(tempDir, opts); err != nil {
		t.Fatalf("GenerateLevelDat failed: %v", err)
	}

	levelPath := filepath.Join(tempDir, "level.dat")
	data, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatalf("Failed to read level.dat: %v", err)
	}

	_, root, err := nbt.Read(data)
	if err != nil {
		t.Fatalf("Failed to parse level.dat NBT: %v", err)
	}

	rootComp := root.(*bdf.TagCompound)
	dataComp, ok := rootComp.GetCompound("Data")
	if !ok || dataComp == nil {
		t.Fatalf("Missing Data compound in level.dat")
	}

	levelName, ok := dataComp.GetString("LevelName")
	if !ok || levelName != "TestWorld" {
		t.Errorf("Expected LevelName 'TestWorld', got %q", levelName)
	}
}
