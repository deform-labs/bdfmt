package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const defaultConfigTemplate = `# BDF-Pregen preferences. Command-line flags override these values.

[world]
output = "./world"
name = "BDF-Pregen-World"
seed = 1337

[generation]
radius = 16
center_x = 0
center_z = 0
terrain = "noise" # noise or flat
workers = 4
sea_level = 64
base_height = 68 # average terrain surface height
noise_scale = 0.005 # lower values create broader terrain features
height_scale = 26.0 # vertical terrain variation around base_height
noise_octaves = 4 # detail layers; higher values add finer variation
persistence = 0.5 # amplitude falloff per octave
lacunarity = 2.0 # frequency multiplier per octave
gpu = true
gpu_device = "" # blank selects the first available OpenCL GPU
gpu_batch_size = 256 # tune batch size to your GPU memory and throughput

[datapacks]
paths = [] # local .zip files or directories containing pack.mcmeta

[level]
spawn_x = 0
spawn_y = 65
spawn_z = 0
game_mode = 0 # 0 survival, 1 creative, 2 adventure, 3 spectator
difficulty = 2 # 0 peaceful, 1 easy, 2 normal, 3 hard
difficulty_locked = false
allow_commands = true
hardcore = false
generate_features = true
bonus_chest = false
datapacks_enabled = ["vanilla"]
datapacks_disabled = []
`

func initializeConfig(args []string) error {
	fs := flag.NewFlagSet("init-config", flag.ContinueOnError)
	path := fs.String("out", "bdf-pregen.toml", "Path for the new TOML configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	file, err := os.OpenFile(*path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%s already exists; move or remove it before initializing a new config", *path)
		}
		return fmt.Errorf("create config %q: %w", *path, err)
	}
	if _, err := file.WriteString(defaultConfigTemplate); err != nil {
		file.Close()
		os.Remove(*path)
		return fmt.Errorf("write config %q: %w", *path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config %q: %w", *path, err)
	}
	fmt.Printf("Created %s\nEdit it to set your preferred world, generation, and datapack settings.\n", *path)
	return nil
}
