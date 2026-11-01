package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/twlve-secs/bdfmt/pkg/worldgen"
	"github.com/twlve-secs/bdfmt/pkg/worldgen/opencl"
)

func renderProgressBar(current, total int64, width int) string {
	if total <= 0 {
		return ""
	}
	ratio := float64(current) / float64(total)
	if ratio > 1.0 {
		ratio = 1.0
	}
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bar
}

type stringListFlag []string

func (values *stringListFlag) String() string { return strings.Join(*values, ",") }
func (values *stringListFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func regenerateLevelDat(args []string) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("leveldat", flag.ContinueOnError)
	fs.String("config", "bdfgen.toml", "TOML configuration file")
	outDefault, nameDefault, seedDefault := "", "BDF-Pregen-World", int64(1337)
	if cfg.World.Output != nil {
		outDefault = *cfg.World.Output
	}
	if cfg.World.Name != nil {
		nameDefault = *cfg.World.Name
	}
	if cfg.World.Seed != nil {
		seedDefault = *cfg.World.Seed
	}
	defaults := cfg.levelOptions(nameDefault, seedDefault)
	out := fs.String("out", outDefault, "Existing world directory (required)")
	name := fs.String("name", nameDefault, "World name stored in level.dat")
	seed := fs.Int64("seed", seedDefault, "World generation seed")
	spawnX := fs.Int("spawn-x", defaults.SpawnX, "Spawn X coordinate")
	spawnY := fs.Int("spawn-y", defaults.SpawnY, "Spawn Y coordinate")
	spawnZ := fs.Int("spawn-z", defaults.SpawnZ, "Spawn Z coordinate")
	gameType := fs.Int("game-mode", defaults.GameType, "Game mode: 0 survival, 1 creative, 2 adventure, 3 spectator")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("--out is required; specify the existing world directory")
	}
	if *gameType < 0 || *gameType > 3 {
		return fmt.Errorf("--game-mode must be between 0 and 3")
	}

	opts := cfg.levelOptions(*name, *seed)
	opts.SpawnX, opts.SpawnY, opts.SpawnZ, opts.GameType = *spawnX, *spawnY, *spawnZ, *gameType
	if err := worldgen.GenerateLevelDat(*out, opts); err != nil {
		return err
	}
	fmt.Printf("Regenerated %s/level.dat (no chunks or region files were changed)\n", *out)
	return nil
}

func runStart(args []string) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	defaults := worldgen.DefaultGeneratorOptions()
	outDefault, seedDefault := defaults.WorldDir, defaults.Seed
	if cfg.World.Output != nil {
		outDefault = *cfg.World.Output
	}
	if cfg.World.Seed != nil {
		seedDefault = *cfg.World.Seed
	}
	nameDefault := "BDFGen-World"
	if cfg.World.Name != nil {
		nameDefault = *cfg.World.Name
	}
	radiusDefault, centerXDefault, centerZDefault := defaults.RadiusChunks, defaults.CenterX, defaults.CenterZ
	typeDefault, workersDefault, seaLevelDefault, gpuDefault := defaults.TerrainType, defaults.Workers, defaults.SeaLevel, defaults.EnableGPU
	if cfg.Generation.Radius != nil {
		radiusDefault = *cfg.Generation.Radius
	}
	if cfg.Generation.CenterX != nil {
		centerXDefault = *cfg.Generation.CenterX
	}
	if cfg.Generation.CenterZ != nil {
		centerZDefault = *cfg.Generation.CenterZ
	}
	if cfg.Generation.Terrain != nil {
		typeDefault = *cfg.Generation.Terrain
	}
	if cfg.Generation.Workers != nil {
		workersDefault = *cfg.Generation.Workers
	}
	if cfg.Generation.SeaLevel != nil {
		seaLevelDefault = *cfg.Generation.SeaLevel
	}
	baseHeightDefault, noiseScaleDefault, heightScaleDefault := defaults.BaseHeight, defaults.NoiseScale, defaults.HeightScale
	noiseOctavesDefault, persistenceDefault, lacunarityDefault := defaults.NoiseOctaves, defaults.NoisePersistence, defaults.NoiseLacunarity
	if cfg.Generation.BaseHeight != nil {
		baseHeightDefault = *cfg.Generation.BaseHeight
	}
	if cfg.Generation.NoiseScale != nil {
		noiseScaleDefault = *cfg.Generation.NoiseScale
	}
	if cfg.Generation.HeightScale != nil {
		heightScaleDefault = *cfg.Generation.HeightScale
	}
	if cfg.Generation.NoiseOctaves != nil {
		noiseOctavesDefault = *cfg.Generation.NoiseOctaves
	}
	if cfg.Generation.Persistence != nil {
		persistenceDefault = *cfg.Generation.Persistence
	}
	if cfg.Generation.Lacunarity != nil {
		lacunarityDefault = *cfg.Generation.Lacunarity
	}
	if cfg.Generation.GPU != nil {
		gpuDefault = *cfg.Generation.GPU
	}
	gpuDeviceDefault := ""
	if cfg.Generation.GPUDevice != nil {
		gpuDeviceDefault = *cfg.Generation.GPUDevice
	}
	gpuBatchDefault := defaults.GPUChunkBatch
	if cfg.Generation.GPUChunkBatch != nil {
		gpuBatchDefault = *cfg.Generation.GPUChunkBatch
	}
	fs := flag.NewFlagSet("bdfgen start", flag.ContinueOnError)
	fs.String("config", "bdf-pregen.toml", "TOML configuration file")
	outFlag := fs.String("out", outDefault, "Output world directory")
	radiusFlag := fs.Int("radius", radiusDefault, "Pregeneration radius in chunks")
	centerXFlag := fs.Int("center-x", centerXDefault, "Center chunk X")
	centerZFlag := fs.Int("center-z", centerZDefault, "Center chunk Z")
	seedFlag := fs.Int64("seed", seedDefault, "World generation seed")
	typeFlag := fs.String("type", typeDefault, "Terrain type: 'noise' (procedural 3D hills) or 'flat'")
	workersFlag := fs.Int("workers", workersDefault, "Concurrent worker threads")
	seaLevelFlag := fs.Int("sea-level", seaLevelDefault, "Sea level height")
	baseHeightFlag := fs.Int("base-height", baseHeightDefault, "Average terrain surface height")
	noiseScaleFlag := fs.Float64("noise-scale", noiseScaleDefault, "Terrain feature scale; lower values make broader features")
	heightScaleFlag := fs.Float64("height-scale", heightScaleDefault, "Vertical terrain variation around base height")
	noiseOctavesFlag := fs.Int("noise-octaves", noiseOctavesDefault, "Number of terrain noise detail layers")
	persistenceFlag := fs.Float64("persistence", persistenceDefault, "Noise amplitude multiplier per octave")
	lacunarityFlag := fs.Float64("lacunarity", lacunarityDefault, "Noise frequency multiplier per octave")
	gpuFlag := fs.Bool("gpu", gpuDefault, "Enable OpenCL GPU compute acceleration")
	gpuDeviceFlag := fs.String("gpu-device", gpuDeviceDefault, "Preferred OpenCL device name (substring; see 'bdfgen devices')")
	gpuBatchFlag := fs.Int("gpu-batch-size", gpuBatchDefault, "Chunks per OpenCL dispatch (default 256)")
	datapackPaths := stringListFlag(append([]string(nil), cfg.DataPacks.Paths...))
	fs.Var(&datapackPaths, "datapack", "Datapack .zip or directory to install (repeatable; drag paths after command also works)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	datapackPaths = append(datapackPaths, fs.Args()...)
	if *gpuBatchFlag < 1 || *gpuBatchFlag > 1024 {
		return fmt.Errorf("--gpu-batch-size must be between 1 and 1024")
	}
	if *noiseScaleFlag <= 0 || *heightScaleFlag <= 0 || *noiseOctavesFlag < 1 || *noiseOctavesFlag > 16 || *persistenceFlag <= 0 || *persistenceFlag > 1 || *lacunarityFlag < 1 {
		return fmt.Errorf("invalid terrain noise settings: scale and height must be positive, octaves 1..16, persistence (0..1], and lacunarity >= 1")
	}
	if float64(*baseHeightFlag)-*heightScaleFlag < float64(worldgen.MinY+1) || float64(*baseHeightFlag)+*heightScaleFlag > float64(worldgen.MaxY) {
		return fmt.Errorf("base-height +/- height-scale must stay within world Y=%d..%d", worldgen.MinY+1, worldgen.MaxY)
	}
	if *seaLevelFlag < worldgen.MinY || *seaLevelFlag > worldgen.MaxY {
		return fmt.Errorf("sea-level must be within world Y=%d..%d", worldgen.MinY, worldgen.MaxY)
	}

	fmt.Printf("Output World:      %s\n", *outFlag)
	fmt.Printf("Chunk Radius:      %d (Diameter: %d chunks)\n", *radiusFlag, *radiusFlag*2+1)
	fmt.Printf("Center Chunk:      (%d, %d)\n", *centerXFlag, *centerZFlag)
	fmt.Printf("Seed:              %d\n", *seedFlag)
	fmt.Printf("Terrain Type:      %s\n", *typeFlag)
	fmt.Printf("Terrain Shape:     base=%d scale=%g height=%g octaves=%d persistence=%g lacunarity=%g\n", *baseHeightFlag, *noiseScaleFlag, *heightScaleFlag, *noiseOctavesFlag, *persistenceFlag, *lacunarityFlag)
	fmt.Printf("Workers:           %d\n", *workersFlag)
	fmt.Printf("OpenCL GPU:        %v\n", *gpuFlag)
	if *gpuDeviceFlag != "" {
		fmt.Printf("OpenCL Device:     %s\n", *gpuDeviceFlag)
	}
	fmt.Printf("GPU Batch Size:    %d chunks\n", *gpuBatchFlag)

	opts := worldgen.GeneratorOptions{
		WorldDir:         *outFlag,
		RadiusChunks:     *radiusFlag,
		CenterX:          *centerXFlag,
		CenterZ:          *centerZFlag,
		Seed:             *seedFlag,
		TerrainType:      *typeFlag,
		Workers:          *workersFlag,
		SeaLevel:         *seaLevelFlag,
		BaseHeight:       *baseHeightFlag,
		NoiseScale:       *noiseScaleFlag,
		HeightScale:      *heightScaleFlag,
		NoiseOctaves:     *noiseOctavesFlag,
		NoisePersistence: *persistenceFlag,
		NoiseLacunarity:  *lacunarityFlag,
		EnableGPU:        *gpuFlag,
		GPUDevice:        *gpuDeviceFlag,
		GPUChunkBatch:    *gpuBatchFlag,
		FlatLayers:       []string{"minecraft:bedrock", "minecraft:dirt", "minecraft:dirt", "minecraft:grass_block"},
		LevelOptions:     cfg.levelOptions(nameDefault, *seedFlag),
	}
	if len(datapackPaths) > 0 {
		if err := worldgen.InstallDataPacks(*outFlag, datapackPaths, &opts.LevelOptions); err != nil {
			return fmt.Errorf("install datapacks: %w", err)
		}
		fmt.Printf("Datapacks installed and enabled in world metadata: %d\n", len(datapackPaths))
		fmt.Println("Note: datapacks are installed in the world, but BDFGen's built-in terrain does not execute datapack worldgen rules.")
	}
	if cfg.Level.SpawnY == nil {
		opts.LevelOptions.SpawnY = *seaLevelFlag + 1
	}

	gen := worldgen.NewGenerator(opts)

	lastLen := 0
	progressCallback := func(p *worldgen.ProgressStats) {
		completed := atomic.LoadInt64(&p.CompletedChunks)
		total := p.TotalChunks
		percent := p.Percent()
		cps := p.ChunksPerSec()
		elapsed := time.Since(p.StartTime).Round(100 * time.Millisecond)

		bar := renderProgressBar(completed, total, 25)
		statusLine := fmt.Sprintf("\r[%s] %4.1f%% (%d/%d chunks) | %.1f chunks/s | Elapsed: %v",
			bar, percent, completed, total, cps, elapsed)

		if len(statusLine) < lastLen {
			statusLine += strings.Repeat(" ", lastLen-len(statusLine))
		}
		lastLen = len(statusLine)
		fmt.Print(statusLine)
	}

	startTime := time.Now()
	err = gen.Run(progressCallback)
	fmt.Println()

	if err != nil {
		return fmt.Errorf("pregeneration failed: %w", err)
	}

	elapsed := time.Since(startTime)
	totalChunks := int64((*radiusFlag*2 + 1) * (*radiusFlag*2 + 1))
	avgCPS := float64(totalChunks) / elapsed.Seconds()

	fmt.Printf("Pregeneration Completed in %v!\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Total Chunks Generated:  %d\n", totalChunks)
	fmt.Printf("Average Speed:           %.1f chunks/sec\n", avgCPS)
	fmt.Printf("World Folder Ready:      %s\n", *outFlag)
	return nil
}

const cliUsage = `BDF-Pregen: Minecraft world pregenerator

Usage:
  bdfgen <command> [options]

Commands:
  init       Create a bdf-pregen.toml preferences file
  start      Pregenerate a world
  devices    List OpenCL acceleration devices
  leveldat   Regenerate only an existing world's level.dat
  help       Show help for a command

Run "bdfgen <command> --help" for command options.
`

func printHelp(command string) {
	switch command {
	case "init", "init-config":
		fmt.Print("Create a TOML preferences file.\n\nUsage:\n  bdfgen init [--out=PATH]\n\nOptions:\n  --out   Output config path (default bdf-pregen.toml)\n  -h      Show this help\n")
	case "start":
		fmt.Print("Pregenerate a Minecraft world.\n\nUsage:\n  bdfgen start [options]\n\nOptions:\n")
		fmt.Print("  --config, --out, --radius, --center-x, --center-z, --seed,\n  --type, --workers, --sea-level, --base-height, --noise-scale, --height-scale,\n  --noise-octaves, --persistence, --lacunarity, --gpu, --gpu-device, --gpu-batch-size,\n  --datapack PATH (repeatable; dropped paths may follow options)\n")
	case "devices":
		fmt.Print("List available OpenCL devices.\n\nUsage:\n  bdfgen devices\n")
	case "leveldat":
		fmt.Print("Regenerate only level.dat in an existing world.\n\nUsage:\n  bdfgen leveldat [options]\n\nOptions:\n  --config, --out, --name, --seed, --spawn-x, --spawn-y, --spawn-z, --game-mode\n")
	default:
		fmt.Print(cliUsage)
	}
}

func main() {
	if len(os.Args) < 2 {
		printHelp("")
		return
	}
	command, args := os.Args[1], os.Args[2:]
	if command == "-h" || command == "--help" || command == "help" {
		topic := ""
		if len(args) > 0 {
			topic = args[0]
		}
		printHelp(topic)
		return
	}
	var err error
	switch command {
	case "init", "init-config":
		if containsHelp(args) {
			printHelp("init")
			return
		}
		err = initializeConfig(args)
	case "start":
		if containsHelp(args) {
			printHelp("start")
			return
		}
		err = runStart(args)
	case "devices":
		if len(args) != 0 {
			fmt.Fprintln(os.Stderr, "devices takes no options")
			os.Exit(2)
		}
		var devices string
		devices, err = opencl.ListDevices()
		if err == nil {
			fmt.Print(devices)
		}
	case "leveldat":
		if containsHelp(args) {
			printHelp("leveldat")
			return
		}
		err = regenerateLevelDat(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q.\n\n", command)
		printHelp("")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func containsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}
