package worldgen

import (
	"math/bits"
	"sync"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
)

const (
	MinSectionY        = -4
	MaxSectionY        = 19
	TotalSections      = MaxSectionY - MinSectionY + 1 // 24 sections (-64 to 320)
	BlocksPerSection   = 16 * 16 * 16                  // 4096
	MinY               = MinSectionY * 16              // -64
	MaxY               = (MaxSectionY+1)*16 - 1        // 319
	DefaultDataVersion = 4671                          // Minecraft Java Edition 1.21.11
)

var standardVoxelPalette = []string{
	"minecraft:air",
	"minecraft:bedrock",
	"minecraft:deepslate",
	"minecraft:stone",
	"minecraft:dirt",
	"minecraft:grass_block",
	"minecraft:sand",
	"minecraft:water",
}

// SectionData represents 4096 blocks in a 16x16x16 section
type SectionData struct {
	Y      int8
	Blocks [BlocksPerSection]string
	Biome  string
}

// NewSectionData creates an empty section initialized with air
func NewSectionData(secY int8, biome string) *SectionData {
	if biome == "" {
		biome = "minecraft:plains"
	}
	s := &SectionData{
		Y:     secY,
		Biome: biome,
	}
	for i := range s.Blocks {
		s.Blocks[i] = "minecraft:air"
	}
	return s
}

// Reset clears section blocks back to air
func (s *SectionData) Reset(secY int8, biome string) {
	s.Y = secY
	s.Biome = biome
	for i := range s.Blocks {
		s.Blocks[i] = "minecraft:air"
	}
}

// BlockIndex converts local section (x, y, z) in [0..15] to 0..4095
func BlockIndex(x, y, z int) int {
	return (y << 8) | (z << 4) | x
}

// BuildSectionTag converts SectionData into a 1.18+ BDF Compound
func (s *SectionData) BuildSectionTag() *bdf.TagCompound {
	comp := bdf.NewTagCompound()
	comp.Set("Y", bdf.TagByte(s.Y))

	// 1. Build Block States Palette & Data
	paletteMap := make(map[string]int)
	var paletteList []string
	for _, block := range s.Blocks {
		if _, ok := paletteMap[block]; !ok {
			paletteMap[block] = len(paletteList)
			paletteList = append(paletteList, block)
		}
	}

	blockStates := bdf.NewTagCompound()
	paletteTagList := &bdf.TagList{
		ElementType: bdf.TagTypeCompound,
		Elements:    make([]bdf.Tag, len(paletteList)),
	}
	for i, name := range paletteList {
		entry := bdf.NewTagCompound()
		entry.Set("Name", bdf.TagString(name))
		paletteTagList.Elements[i] = entry
	}
	blockStates.Set("palette", paletteTagList)

	// If palette has > 1 block, pack indices into TAG_Long_Array
	if len(paletteList) > 1 {
		bitsPerBlock := bits.Len(uint(len(paletteList) - 1))
		if bitsPerBlock < 4 {
			bitsPerBlock = 4 // Minimum 4 bits per block in 1.18+
		}

		valuesPerLong := 64 / bitsPerBlock
		longsCount := (BlocksPerSection + valuesPerLong - 1) / valuesPerLong
		dataLongs := make([]int64, longsCount)

		for i, block := range s.Blocks {
			paletteIdx := paletteMap[block]
			longIdx := i / valuesPerLong
			subOffset := (i % valuesPerLong) * bitsPerBlock
			dataLongs[longIdx] |= int64(paletteIdx) << subOffset
		}

		blockStates.Set("data", bdf.TagLongArray(dataLongs))
	}
	comp.Set("block_states", blockStates)

	// 2. Build Biomes (Single biome per section optimization)
	biomes := bdf.NewTagCompound()
	biomePalette := &bdf.TagList{
		ElementType: bdf.TagTypeString,
		Elements:    []bdf.Tag{bdf.TagString(s.Biome)},
	}
	biomes.Set("palette", biomePalette)
	comp.Set("biomes", biomes)

	return comp
}

// ChunkData represents a full chunk from Y=-64 to 320 (24 sections)
type ChunkData struct {
	ChunkX   int
	ChunkZ   int
	Sections [TotalSections]*SectionData
	Biome    string
}

var chunkPool = sync.Pool{
	New: func() interface{} {
		c := &ChunkData{
			Biome: "minecraft:plains",
		}
		for i := 0; i < TotalSections; i++ {
			secY := int8(MinSectionY + i)
			c.Sections[i] = NewSectionData(secY, "minecraft:plains")
		}
		return c
	},
}

// GetPooledChunk retrieves a reusable ChunkData instance from the pool
func GetPooledChunk(chunkX, chunkZ int, biome string) *ChunkData {
	c := chunkPool.Get().(*ChunkData)
	c.ChunkX = chunkX
	c.ChunkZ = chunkZ
	if biome == "" {
		biome = "minecraft:plains"
	}
	c.Biome = biome
	for i := 0; i < TotalSections; i++ {
		secY := int8(MinSectionY + i)
		c.Sections[i].Reset(secY, biome)
	}
	return c
}

// PutPooledChunk returns a ChunkData instance to the pool
func PutPooledChunk(c *ChunkData) {
	chunkPool.Put(c)
}

// NewChunkData initializes a modern chunk
func NewChunkData(chunkX, chunkZ int, biome string) *ChunkData {
	if biome == "" {
		biome = "minecraft:plains"
	}
	c := &ChunkData{
		ChunkX: chunkX,
		ChunkZ: chunkZ,
		Biome:  biome,
	}
	for i := 0; i < TotalSections; i++ {
		secY := int8(MinSectionY + i)
		c.Sections[i] = NewSectionData(secY, biome)
	}
	return c
}

// SetBlock sets a block at world coordinates within the chunk (local x: 0..15, world y: -64..319, local z: 0..15)
func (c *ChunkData) SetBlock(localX, worldY, localZ int, blockName string) {
	if worldY < MinY || worldY > MaxY || localX < 0 || localX > 15 || localZ < 0 || localZ > 15 {
		return
	}
	secIndex := (worldY - MinY) >> 4
	localY := (worldY - MinY) & 15
	idx := BlockIndex(localX, localY, localZ)
	c.Sections[secIndex].Blocks[idx] = blockName
}

// BuildChunkTag constructs the full 1.18 - 1.20+ Chunk Compound Tag
func (c *ChunkData) BuildChunkTag() *bdf.TagCompound {
	root := bdf.NewTagCompound()

	root.Set("DataVersion", bdf.TagInt(DefaultDataVersion))
	root.Set("xPos", bdf.TagInt(int32(c.ChunkX)))
	root.Set("zPos", bdf.TagInt(int32(c.ChunkZ)))
	root.Set("yPos", bdf.TagInt(int32(MinSectionY)))
	root.Set("Status", bdf.TagString("minecraft:full"))

	// Sections list
	sectionsList := &bdf.TagList{
		ElementType: bdf.TagTypeCompound,
		Elements:    make([]bdf.Tag, TotalSections),
	}
	for i := 0; i < TotalSections; i++ {
		sectionsList.Elements[i] = c.Sections[i].BuildSectionTag()
	}
	root.Set("sections", sectionsList)

	// Post processing
	postProcessing := &bdf.TagList{
		ElementType: bdf.TagTypeList,
		Elements:    make([]bdf.Tag, TotalSections),
	}
	for i := 0; i < TotalSections; i++ {
		postProcessing.Elements[i] = &bdf.TagList{ElementType: bdf.TagTypeShort, Elements: []bdf.Tag{}}
	}
	root.Set("PostProcessing", postProcessing)

	// Empty structures & ticks required by modern Minecraft
	structures := bdf.NewTagCompound()
	structures.Set("starts", bdf.NewTagCompound())
	structures.Set("References", bdf.NewTagCompound())
	root.Set("structures", structures)

	root.Set("block_entities", &bdf.TagList{ElementType: bdf.TagTypeCompound, Elements: []bdf.Tag{}})
	root.Set("block_ticks", &bdf.TagList{ElementType: bdf.TagTypeCompound, Elements: []bdf.Tag{}})
	root.Set("fluid_ticks", &bdf.TagList{ElementType: bdf.TagTypeCompound, Elements: []bdf.Tag{}})

	return root
}

// BuildChunkTagFromGPU creates a 1.18+ Chunk Compound directly from GPU-generated voxel sections
func BuildChunkTagFromGPU(chunkX, chunkZ int, secTypes [24]uint8, secData [24][256]int64) *bdf.TagCompound {
	root := bdf.NewTagCompound()

	root.Set("DataVersion", bdf.TagInt(DefaultDataVersion))
	root.Set("xPos", bdf.TagInt(int32(chunkX)))
	root.Set("zPos", bdf.TagInt(int32(chunkZ)))
	root.Set("yPos", bdf.TagInt(int32(MinSectionY)))
	root.Set("Status", bdf.TagString("minecraft:full"))

	// Pre-built shared standard palette for mixed sections
	mixedPaletteList := &bdf.TagList{
		ElementType: bdf.TagTypeCompound,
		Elements:    make([]bdf.Tag, len(standardVoxelPalette)),
	}
	for i, name := range standardVoxelPalette {
		entry := bdf.NewTagCompound()
		entry.Set("Name", bdf.TagString(name))
		mixedPaletteList.Elements[i] = entry
	}

	biomePlains := &bdf.TagCompound{}
	biomePlainsList := &bdf.TagList{
		ElementType: bdf.TagTypeString,
		Elements:    []bdf.Tag{bdf.TagString("minecraft:plains")},
	}
	biomePlains.Set("palette", biomePlainsList)

	sectionsList := &bdf.TagList{
		ElementType: bdf.TagTypeCompound,
		Elements:    make([]bdf.Tag, TotalSections),
	}

	for s := 0; s < TotalSections; s++ {
		secY := int8(MinSectionY + s)
		secComp := bdf.NewTagCompound()
		secComp.Set("Y", bdf.TagByte(secY))
		secComp.Set("biomes", biomePlains)

		blockStates := bdf.NewTagCompound()
		stype := secTypes[s]

		switch stype {
		case 0: // All Air
			pal := &bdf.TagList{
				ElementType: bdf.TagTypeCompound,
				Elements:    []bdf.Tag{newSinglePalette("minecraft:air")},
			}
			blockStates.Set("palette", pal)
		case 1: // All Deepslate
			pal := &bdf.TagList{
				ElementType: bdf.TagTypeCompound,
				Elements:    []bdf.Tag{newSinglePalette("minecraft:deepslate")},
			}
			blockStates.Set("palette", pal)
		case 2: // All Stone
			pal := &bdf.TagList{
				ElementType: bdf.TagTypeCompound,
				Elements:    []bdf.Tag{newSinglePalette("minecraft:stone")},
			}
			blockStates.Set("palette", pal)
		default: // Mixed (GPU computed packed longs)
			blockStates.Set("palette", mixedPaletteList)
			dataCopy := make([]int64, 256)
			copy(dataCopy, secData[s][:])
			blockStates.Set("data", bdf.TagLongArray(dataCopy))
		}

		secComp.Set("block_states", blockStates)
		sectionsList.Elements[s] = secComp
	}
	root.Set("sections", sectionsList)

	// Post processing
	postProcessing := &bdf.TagList{
		ElementType: bdf.TagTypeList,
		Elements:    make([]bdf.Tag, TotalSections),
	}
	for i := 0; i < TotalSections; i++ {
		postProcessing.Elements[i] = &bdf.TagList{ElementType: bdf.TagTypeShort, Elements: []bdf.Tag{}}
	}
	root.Set("PostProcessing", postProcessing)

	structures := bdf.NewTagCompound()
	structures.Set("starts", bdf.NewTagCompound())
	structures.Set("References", bdf.NewTagCompound())
	root.Set("structures", structures)

	root.Set("block_entities", &bdf.TagList{ElementType: bdf.TagTypeCompound, Elements: []bdf.Tag{}})
	root.Set("block_ticks", &bdf.TagList{ElementType: bdf.TagTypeCompound, Elements: []bdf.Tag{}})
	root.Set("fluid_ticks", &bdf.TagList{ElementType: bdf.TagTypeCompound, Elements: []bdf.Tag{}})

	return root
}

func newSinglePalette(blockName string) *bdf.TagCompound {
	p := bdf.NewTagCompound()
	p.Set("Name", bdf.TagString(blockName))
	return p
}
