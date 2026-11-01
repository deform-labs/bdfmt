package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/twlve-secs/bdfmt/pkg/worldgen"
)

// appConfig is the user-facing TOML format. Pointer fields let false and zero
// explicitly override defaults.
type appConfig struct {
	DataPacks struct {
		Paths []string `toml:"paths"`
	} `toml:"datapacks"`
	World struct {
		Output *string `toml:"output"`
		Name   *string `toml:"name"`
		Seed   *int64  `toml:"seed"`
	} `toml:"world"`
	Generation struct {
		Radius        *int     `toml:"radius"`
		CenterX       *int     `toml:"center_x"`
		CenterZ       *int     `toml:"center_z"`
		Terrain       *string  `toml:"terrain"`
		Workers       *int     `toml:"workers"`
		SeaLevel      *int     `toml:"sea_level"`
		BaseHeight    *int     `toml:"base_height"`
		NoiseScale    *float64 `toml:"noise_scale"`
		HeightScale   *float64 `toml:"height_scale"`
		NoiseOctaves  *int     `toml:"noise_octaves"`
		Persistence   *float64 `toml:"persistence"`
		Lacunarity    *float64 `toml:"lacunarity"`
		GPU           *bool    `toml:"gpu"`
		GPUDevice     *string  `toml:"gpu_device"`
		GPUChunkBatch *int     `toml:"gpu_batch_size"`
	} `toml:"generation"`
	Level struct {
		SpawnX            *int     `toml:"spawn_x"`
		SpawnY            *int     `toml:"spawn_y"`
		SpawnZ            *int     `toml:"spawn_z"`
		GameMode          *int     `toml:"game_mode"`
		Difficulty        *int     `toml:"difficulty"`
		DifficultyLocked  *bool    `toml:"difficulty_locked"`
		AllowCommands     *bool    `toml:"allow_commands"`
		Hardcore          *bool    `toml:"hardcore"`
		GenerateFeatures  *bool    `toml:"generate_features"`
		BonusChest        *bool    `toml:"bonus_chest"`
		DataPacksEnabled  []string `toml:"datapacks_enabled"`
		DataPacksDisabled []string `toml:"datapacks_disabled"`
	} `toml:"level"`
}

func loadConfig(args []string) (appConfig, error) {
	path := "bdf-pregen.toml"
	for i, arg := range args {
		if strings.HasPrefix(arg, "--config=") {
			path = strings.TrimPrefix(arg, "--config=")
		}
		if arg == "--config" && i+1 < len(args) {
			path = args[i+1]
		}
	}
	var cfg appConfig
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) && path == "bdf-pregen.toml" {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %q: %w", path, err)
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, nil
}

func (c appConfig) levelOptions(name string, seed int64) worldgen.LevelOptions {
	o := worldgen.DefaultLevelOptions(name, seed)
	if c.Level.SpawnX != nil {
		o.SpawnX = *c.Level.SpawnX
	}
	if c.Level.SpawnY != nil {
		o.SpawnY = *c.Level.SpawnY
	}
	if c.Level.SpawnZ != nil {
		o.SpawnZ = *c.Level.SpawnZ
	}
	if c.Level.GameMode != nil {
		o.GameType = *c.Level.GameMode
	}
	if c.Level.Difficulty != nil {
		o.Difficulty = *c.Level.Difficulty
	}
	if c.Level.DifficultyLocked != nil {
		o.DifficultyLocked = *c.Level.DifficultyLocked
	}
	if c.Level.AllowCommands != nil {
		o.AllowCommands = *c.Level.AllowCommands
	}
	if c.Level.Hardcore != nil {
		o.Hardcore = *c.Level.Hardcore
	}
	if c.Level.GenerateFeatures != nil {
		o.GenerateFeatures = *c.Level.GenerateFeatures
	}
	if c.Level.BonusChest != nil {
		o.BonusChest = *c.Level.BonusChest
	}
	if c.Level.DataPacksEnabled != nil {
		o.DataPacksEnabled = c.Level.DataPacksEnabled
	}
	if c.Level.DataPacksDisabled != nil {
		o.DataPacksDisabled = c.Level.DataPacksDisabled
	}
	return o
}
