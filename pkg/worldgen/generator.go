package worldgen

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
	"github.com/twlve-secs/bdfmt/pkg/region"
	"github.com/twlve-secs/bdfmt/pkg/worldgen/opencl"
)

// GeneratorOptions configures world pregeneration
type GeneratorOptions struct {
	WorldDir         string
	RadiusChunks     int
	CenterX          int
	CenterZ          int
	Seed             int64
	TerrainType      string // "noise" or "flat"
	Workers          int
	FlatLayers       []string // Layers for flat mode (e.g., bedrock, dirt, dirt, grass_block)
	SeaLevel         int
	BaseHeight       int
	NoiseScale       float64
	HeightScale      float64
	NoiseOctaves     int
	NoisePersistence float64
	NoiseLacunarity  float64
	EnableGPU        bool
	GPUDevice        string
	GPUChunkBatch    int
	LevelOptions     LevelOptions
}

// DefaultGeneratorOptions returns standard generation settings
func DefaultGeneratorOptions() GeneratorOptions {
	return GeneratorOptions{
		WorldDir:         "./world",
		RadiusChunks:     16,
		CenterX:          0,
		CenterZ:          0,
		Seed:             1337,
		TerrainType:      "noise",
		Workers:          runtime.NumCPU(),
		FlatLayers:       []string{"minecraft:bedrock", "minecraft:dirt", "minecraft:dirt", "minecraft:grass_block"},
		SeaLevel:         64,
		BaseHeight:       68,
		NoiseScale:       0.005,
		HeightScale:      26,
		NoiseOctaves:     4,
		NoisePersistence: 0.5,
		NoiseLacunarity:  2,
		EnableGPU:        true,
		GPUChunkBatch:    256,
	}
}

// ProgressStats tracks live pregeneration metrics
type ProgressStats struct {
	TotalChunks     int64
	CompletedChunks int64
	StartTime       time.Time
}

// ChunksPerSec returns current throughput
func (p *ProgressStats) ChunksPerSec() float64 {
	elapsed := time.Since(p.StartTime).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&p.CompletedChunks)) / elapsed
}

// Percent returns completion percentage
func (p *ProgressStats) Percent() float64 {
	if p.TotalChunks == 0 {
		return 100.0
	}
	return (float64(atomic.LoadInt64(&p.CompletedChunks)) / float64(p.TotalChunks)) * 100.0
}

// Generator manages the world pregeneration pipeline
type Generator struct {
	opts        GeneratorOptions
	noise       *PerlinNoise
	regDir      string
	regMu       sync.Mutex
	regions     map[string]*region.Region
	stats       ProgressStats
	clEngine    *opencl.Engine
	isGPUActive bool
}

// NewGenerator creates a world pregeneration engine
func NewGenerator(opts GeneratorOptions) *Generator {
	if opts.Workers <= 0 {
		opts.Workers = runtime.NumCPU()
	}
	if opts.GPUChunkBatch <= 0 {
		opts.GPUChunkBatch = 256
	}
	defaults := DefaultGeneratorOptions()
	if opts.NoiseScale == 0 {
		opts.NoiseScale = defaults.NoiseScale
	}
	if opts.NoiseOctaves == 0 {
		opts.NoiseOctaves = defaults.NoiseOctaves
	}
	if opts.NoisePersistence == 0 {
		opts.NoisePersistence = defaults.NoisePersistence
	}
	if opts.NoiseLacunarity == 0 {
		opts.NoiseLacunarity = defaults.NoiseLacunarity
	}

	g := &Generator{
		opts:    opts,
		noise:   NewPerlinNoise(opts.Seed),
		regDir:  filepath.Join(opts.WorldDir, "region"),
		regions: make(map[string]*region.Region),
	}

	if opts.EnableGPU && opts.TerrainType == "noise" {
		engine, err := opencl.InitGPU(opts.GPUDevice)
		if err == nil {
			g.clEngine = engine
			g.isGPUActive = true
			info := engine.Info()
			fmt.Printf("[OpenCL] GPU 3D Voxel Generator ACTIVE: %s (%s, %d compute units, %d MHz)\n",
				info.DeviceName, info.VendorName, info.ComputeUnits, info.ClockFrequency)
		} else {
			fmt.Printf("[OpenCL] GPU initialization: %v\n", err)
			fmt.Printf("[OpenCL] Continuing with high-speed multi-core CPU SIMD engine.\n")
		}
	}

	return g
}

// IsGPUActive reports if OpenCL GPU compute is active
func (g *Generator) IsGPUActive() bool {
	return g.isGPUActive
}

// Close releases GPU and region resources
func (g *Generator) Close() {
	if g.clEngine != nil {
		g.clEngine.Close()
		g.clEngine = nil
	}
}

// Run executes the complete asynchronous multi-threaded world pregeneration
func (g *Generator) Run(progressCallback func(p *ProgressStats)) error {
	defer g.Close()
	if g.opts.EnableGPU && g.opts.TerrainType == "noise" && strings.TrimSpace(g.opts.GPUDevice) != "" && !g.isGPUActive {
		return fmt.Errorf("requested OpenCL device %q is unavailable; check that the driver exposes it (for Mesa Rusticl, set RUSTICL_ENABLE=iris) and confirm its name with 'bdfgen devices'", g.opts.GPUDevice)
	}

	// 1. Generate level.dat
	levelOpts := g.opts.LevelOptions
	if levelOpts.LevelName == "" {
		levelOpts = DefaultLevelOptions("BDF-Pregen-World", g.opts.Seed)
	} else {
		levelOpts.Seed = g.opts.Seed
	}
	levelOpts.SpawnY = g.opts.SeaLevel + 1
	if err := GenerateLevelDat(g.opts.WorldDir, levelOpts); err != nil {
		return fmt.Errorf("failed to create level.dat: %w", err)
	}

	// 2. Compute chunk bounds
	minChunkX := g.opts.CenterX - g.opts.RadiusChunks
	maxChunkX := g.opts.CenterX + g.opts.RadiusChunks
	minChunkZ := g.opts.CenterZ - g.opts.RadiusChunks
	maxChunkZ := g.opts.CenterZ + g.opts.RadiusChunks

	width := maxChunkX - minChunkX + 1
	height := maxChunkZ - minChunkZ + 1
	totalChunks := int64(width * height)

	g.stats = ProgressStats{
		TotalChunks:     totalChunks,
		CompletedChunks: 0,
		StartTime:       time.Now(),
	}

	// 3. Setup Channels
	resultQueueSize := g.opts.Workers * 64
	results := make(chan region.CompressedChunk, resultQueueSize)

	var writerWg sync.WaitGroup
	var workerWg sync.WaitGroup

	// Stage 2: Asynchronous Region Disk Writers
	ioWorkers := 4
	if ioWorkers > g.opts.Workers {
		ioWorkers = g.opts.Workers
	}
	if ioWorkers < 2 {
		ioWorkers = 2
	}

	for w := 0; w < ioWorkers; w++ {
		writerWg.Add(1)
		go func() {
			defer writerWg.Done()
			type regionBatch struct {
				rx, rz int
				chunks []region.CompressedChunk
			}
			pending := make(map[string]*regionBatch)
			flush := func(key string, batch *regionBatch) {
				reg, err := g.getOrCreateRegion(batch.rx, batch.rz)
				if err == nil {
					err = reg.WriteChunksCompressed(batch.chunks)
				}
				if err != nil {
					fmt.Printf("Warning: failed to flush %d chunks to region r.%d.%d.mca: %v\n", len(batch.chunks), batch.rx, batch.rz, err)
				}
				atomic.AddInt64(&g.stats.CompletedChunks, int64(len(batch.chunks)))
				delete(pending, key)
			}
			flushAll := func() {
				for key, batch := range pending {
					flush(key, batch)
				}
			}
			ticker := time.NewTicker(25 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case chunk, ok := <-results:
					if !ok {
						flushAll()
						return
					}
					rx, rz := region.ChunkCoordsToRegion(chunk.ChunkX, chunk.ChunkZ)
					key := fmt.Sprintf("%d,%d", rx, rz)
					batch := pending[key]
					if batch == nil {
						batch = &regionBatch{rx: rx, rz: rz, chunks: make([]region.CompressedChunk, 0, 32)}
						pending[key] = batch
					}
					batch.chunks = append(batch.chunks, chunk)
					if len(batch.chunks) >= 32 {
						flush(key, batch)
					}
				case <-ticker.C:
					flushAll()
				}
			}
		}()
	}

	// Progress ticker
	doneTicker := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if progressCallback != nil {
					progressCallback(&g.stats)
				}
			case <-doneTicker:
				if progressCallback != nil {
					progressCallback(&g.stats)
				}
				return
			}
		}
	}()

	// Stage 1: Pipeline Execution
	if g.isGPUActive && g.opts.TerrainType == "noise" {
		// GPU-Accelerated 3D Voxel Generator Path!
		// GPU generates batches of 3D chunks (all 24 sections, 6144 packed words per chunk)
		batchSize := g.opts.GPUChunkBatch
		gpuResults := make(chan opencl.GPUChunkResult, g.opts.Workers*32)

		// GPU Dispatcher Goroutine
		go func() {
			defer close(gpuResults)
			var currentBatch [][2]int

			dispatchBatch := func(b [][2]int) {
				if len(b) == 0 {
					return
				}
				res, err := g.clEngine.GenerateChunkVoxelsBatch(
					b,
					float32(g.opts.NoiseScale),
					float32(g.opts.SeaLevel),
					float32(g.opts.BaseHeight),
					float32(g.opts.HeightScale),
					g.opts.NoiseOctaves,
					float32(g.opts.NoisePersistence),
					float32(g.opts.NoiseLacunarity),
					float32(math.Max(float64(g.opts.SeaLevel), float64(g.opts.BaseHeight)+g.opts.HeightScale)),
					g.noise.Perm(),
				)
				if err != nil {
					fmt.Printf("Warning: GPU batch error: %v\n", err)
					return
				}
				for _, r := range res {
					gpuResults <- r
				}
			}

			for cz := minChunkZ; cz <= maxChunkZ; cz++ {
				for cx := minChunkX; cx <= maxChunkX; cx++ {
					currentBatch = append(currentBatch, [2]int{cx, cz})
					if len(currentBatch) == batchSize {
						dispatchBatch(currentBatch)
						currentBatch = currentBatch[:0]
					}
				}
			}
			if len(currentBatch) > 0 {
				dispatchBatch(currentBatch)
			}
		}()

		// CPU Workers consume GPU-generated 3D chunks, encode to BDF & compress with SIMD Zlib
		for i := 0; i < g.opts.Workers; i++ {
			workerWg.Add(1)
			go func() {
				defer workerWg.Done()
				for r := range gpuResults {
					zlibBytes, err := FastCompressGPUChunkPacked(r.ChunkX, r.ChunkZ, r.SectionTypes, r.SectionData)
					if err != nil {
						fmt.Printf("Warning: compression failed for (%d, %d): %v\n", r.ChunkX, r.ChunkZ, err)
						continue
					}

					results <- region.CompressedChunk{
						ChunkX: r.ChunkX,
						ChunkZ: r.ChunkZ,
						Data:   zlibBytes,
					}
				}
			}()
		}

	} else {
		// CPU Fallback Path
		type chunkTask struct {
			cx, cz int
		}
		tasks := make(chan chunkTask, g.opts.Workers*32)

		for i := 0; i < g.opts.Workers; i++ {
			workerWg.Add(1)
			go func() {
				defer workerWg.Done()
				for task := range tasks {
					var chunkTag bdf.Tag
					if g.opts.TerrainType == "flat" {
						chunk := GetPooledChunk(task.cx, task.cz, "minecraft:plains")
						g.generateFlatChunk(chunk)
						chunkTag = chunk.BuildChunkTag()
						PutPooledChunk(chunk)
					} else {
						chunk := GetPooledChunk(task.cx, task.cz, "minecraft:plains")
						g.generateNoiseChunk(chunk)
						chunkTag = chunk.BuildChunkTag()
						PutPooledChunk(chunk)
					}

					zlibBytes, err := region.CompressChunkToZlib(chunkTag)
					if err != nil {
						fmt.Printf("Warning: compression failed for (%d, %d): %v\n", task.cx, task.cz, err)
						continue
					}

					results <- region.CompressedChunk{
						ChunkX: task.cx,
						ChunkZ: task.cz,
						Data:   zlibBytes,
					}
				}
			}()
		}

		go func() {
			for cz := minChunkZ; cz <= maxChunkZ; cz++ {
				for cx := minChunkX; cx <= maxChunkX; cx++ {
					tasks <- chunkTask{cx: cx, cz: cz}
				}
			}
			close(tasks)
		}()
	}

	workerWg.Wait()
	close(results)

	writerWg.Wait()
	close(doneTicker)

	g.regMu.Lock()
	for _, reg := range g.regions {
		_ = reg.Close()
	}
	g.regions = make(map[string]*region.Region)
	g.regMu.Unlock()

	return nil
}

func (g *Generator) getOrCreateRegion(rx, rz int) (*region.Region, error) {
	key := fmt.Sprintf("%d,%d", rx, rz)
	g.regMu.Lock()
	defer g.regMu.Unlock()

	if reg, ok := g.regions[key]; ok {
		return reg, nil
	}

	reg, err := region.Create(g.regDir, rx, rz)
	if err != nil {
		return nil, err
	}
	g.regions[key] = reg
	return reg, nil
}

func (g *Generator) generateFlatChunk(chunk *ChunkData) {
	layers := g.opts.FlatLayers
	for y, block := range layers {
		worldY := MinY + y
		for lz := 0; lz < 16; lz++ {
			for lx := 0; lx < 16; lx++ {
				chunk.SetBlock(lx, worldY, lz, block)
			}
		}
	}
}

func (g *Generator) generateNoiseChunk(chunk *ChunkData) {
	seaLevel := g.opts.SeaLevel

	for lz := 0; lz < 16; lz++ {
		worldZ := chunk.ChunkZ*16 + lz
		for lx := 0; lx < 16; lx++ {
			worldX := chunk.ChunkX*16 + lx

			noiseVal := g.noise.OctaveNoise2D(float64(worldX)*g.opts.NoiseScale, float64(worldZ)*g.opts.NoiseScale, g.opts.NoiseOctaves, g.opts.NoisePersistence, g.opts.NoiseLacunarity)
			surfaceHeight := int(math.Round(float64(g.opts.BaseHeight) + noiseVal*g.opts.HeightScale))

			chunk.SetBlock(lx, MinY, lz, "minecraft:bedrock")

			for y := MinY + 1; y < -59; y++ {
				chunk.SetBlock(lx, y, lz, "minecraft:deepslate")
			}

			for y := -59; y < 0; y++ {
				chunk.SetBlock(lx, y, lz, "minecraft:deepslate")
			}

			for y := 0; y <= surfaceHeight-4 && y <= MaxY; y++ {
				chunk.SetBlock(lx, y, lz, "minecraft:stone")
			}

			if surfaceHeight >= seaLevel {
				for y := surfaceHeight - 3; y < surfaceHeight; y++ {
					if y >= 0 {
						chunk.SetBlock(lx, y, lz, "minecraft:dirt")
					}
				}
				chunk.SetBlock(lx, surfaceHeight, lz, "minecraft:grass_block")
			} else {
				for y := surfaceHeight - 3; y <= surfaceHeight; y++ {
					if y >= 0 {
						chunk.SetBlock(lx, y, lz, "minecraft:sand")
					}
				}
				for y := surfaceHeight + 1; y <= seaLevel; y++ {
					chunk.SetBlock(lx, y, lz, "minecraft:water")
				}
			}
		}
	}
}
