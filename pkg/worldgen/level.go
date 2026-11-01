package worldgen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
	"github.com/twlve-secs/bdfmt/pkg/nbt"
)

// LevelOptions holds settings for level.dat creation
type LevelOptions struct {
	LevelName         string
	Seed              int64
	SpawnX            int
	SpawnY            int
	SpawnZ            int
	GameType          int // 0: Survival, 1: Creative, 2: Adventure, 3: Spectator
	Difficulty        int
	DifficultyLocked  bool
	AllowCommands     bool
	Hardcore          bool
	GenerateFeatures  bool
	BonusChest        bool
	DataPacksEnabled  []string
	DataPacksDisabled []string
}

// DefaultLevelOptions returns standard default level options
func DefaultLevelOptions(levelName string, seed int64) LevelOptions {
	return LevelOptions{
		LevelName:        levelName,
		Seed:             seed,
		SpawnX:           0,
		SpawnY:           65,
		SpawnZ:           0,
		GameType:         0,
		Difficulty:       2,
		AllowCommands:    true,
		GenerateFeatures: true,
		DataPacksEnabled: []string{"vanilla"},
	}
}

// GenerateLevelDat creates a standard Minecraft level.dat file
func GenerateLevelDat(worldDir string, opts LevelOptions) error {
	root := bdf.NewTagCompound()
	data := bdf.NewTagCompound()

	data.Set("DataVersion", bdf.TagInt(DefaultDataVersion))
	data.Set("LevelName", bdf.TagString(opts.LevelName))
	data.Set("generatorName", bdf.TagString("minecraft:normal"))
	data.Set("GameType", bdf.TagInt(int32(opts.GameType)))
	data.Set("Difficulty", bdf.TagByte(byte(opts.Difficulty)))
	data.Set("DifficultyLocked", boolByte(opts.DifficultyLocked))
	data.Set("CustomBossEvents", bdf.NewTagCompound())
	dataPacks := bdf.NewTagCompound()
	dataPacks.Set("Disabled", stringTagList(opts.DataPacksDisabled))
	dataPacks.Set("Enabled", stringTagList(opts.DataPacksEnabled))
	data.Set("DataPacks", dataPacks)
	data.Set("Time", bdf.TagLong(0))
	data.Set("DayTime", bdf.TagLong(1000))
	spawn := bdf.NewTagCompound()
	spawn.Set("dimension", bdf.TagString("minecraft:overworld"))
	spawn.Set("pitch", bdf.TagFloat(0))
	spawn.Set("pos", bdf.TagIntArray{int32(opts.SpawnX), int32(opts.SpawnY), int32(opts.SpawnZ)})
	spawn.Set("yaw", bdf.TagFloat(0))
	data.Set("spawn", spawn)
	data.Set("raining", bdf.TagByte(0))
	data.Set("thundering", bdf.TagByte(0))
	data.Set("clearWeatherTime", bdf.TagInt(60000))
	data.Set("rainTime", bdf.TagInt(120000))
	data.Set("thunderTime", bdf.TagInt(120000))
	data.Set("hardcore", boolByte(opts.Hardcore))
	data.Set("initialized", bdf.TagByte(1))
	data.Set("allowCommands", boolByte(opts.AllowCommands))
	data.Set("version", bdf.TagInt(19133)) // Anvil region format

	// Version info
	version := bdf.NewTagCompound()
	version.Set("Id", bdf.TagInt(DefaultDataVersion))
	version.Set("Name", bdf.TagString("1.21.11"))
	version.Set("Series", bdf.TagString("main"))
	version.Set("Snapshot", bdf.TagByte(0))
	data.Set("Version", version)

	// WorldGenSettings
	worldGen := bdf.NewTagCompound()
	worldGen.Set("seed", bdf.TagLong(opts.Seed))
	worldGen.Set("generate_features", boolByte(opts.GenerateFeatures))
	worldGen.Set("bonus_chest", boolByte(opts.BonusChest))
	dimensions := bdf.NewTagCompound()
	dimensions.Set("minecraft:overworld", makeNoiseDimension("minecraft:overworld", "minecraft:multi_noise", "minecraft:overworld", "minecraft:overworld"))
	dimensions.Set("minecraft:the_nether", makeNoiseDimension("minecraft:the_nether", "minecraft:multi_noise", "minecraft:nether", "minecraft:nether"))
	dimensions.Set("minecraft:the_end", makeNoiseDimension("minecraft:the_end", "minecraft:the_end", "minecraft:end", ""))
	worldGen.Set("dimensions", dimensions)
	data.Set("WorldGenSettings", worldGen)

	root.Set("Data", data)

	// Write standard Gzip compressed NBT file
	levelBytes, err := nbt.Write("", root, nbt.CompressionGzip)
	if err != nil {
		return fmt.Errorf("failed to encode level.dat: %w", err)
	}

	if err := os.MkdirAll(worldDir, 0755); err != nil {
		return err
	}

	outPath := filepath.Join(worldDir, "level.dat")
	return os.WriteFile(outPath, levelBytes, 0644)
}

func stringTagList(values []string) *bdf.TagList {
	elements := make([]bdf.Tag, len(values))
	for i, value := range values {
		elements[i] = bdf.TagString(value)
	}
	return &bdf.TagList{ElementType: bdf.TagTypeString, Elements: elements}
}

func boolByte(value bool) bdf.TagByte {
	if value {
		return 1
	}
	return 0
}

func makeNoiseDimension(dimensionType, biomeSourceType, settings, biomePreset string) *bdf.TagCompound {
	biomeSource := bdf.NewTagCompound()
	if biomePreset != "" {
		biomeSource.Set("preset", bdf.TagString(biomePreset))
	}
	biomeSource.Set("type", bdf.TagString(biomeSourceType))

	generator := bdf.NewTagCompound()
	generator.Set("biome_source", biomeSource)
	generator.Set("settings", bdf.TagString(settings))
	generator.Set("type", bdf.TagString("minecraft:noise"))

	dimension := bdf.NewTagCompound()
	dimension.Set("generator", generator)
	dimension.Set("type", bdf.TagString(dimensionType))
	return dimension
}
