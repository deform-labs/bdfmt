# BDF (Binary Data Format)

A high-performance **64-bit optimized binary data format** designed to replace Minecraft NBT files (`.dat`, `.nbt`, and chunk region data). 

Includes:
1. **Core BDF CLI (`bdf`)**: Compiler, decompiler (supporting **SNBT** and **JSON**), bidirectional NBT converter, zero-copy random access view, and benchmarking suite.
2. **World Pregenerator (`bdfgen`)**: Command-based Minecraft world pregenerator targeting modern Minecraft format with **Full GPU 3D Voxel Generation (OpenCL)**.

---

## Why BDF instead of NBT?

Standard Minecraft NBT was created in 2010 for 32-bit Big-Endian systems. On modern 64-bit Little-Endian CPUs (x86_64, aarch64), NBT incurs major performance penalties:

| Feature | Standard NBT | BDF (This Project) |
|---|---|---|
| **Byte Order** | Big-Endian (requires `BSWAP` on every word) | **Native Little-Endian** (zero byte swapping) |
| **Alignment** | Unaligned, byte-packed | **Strict 8-byte (64-bit) Word Alignment** |
| **Compound Keys** | Repeated UTF-8 strings inline | **Global String Interning Pool** (deduplicated) |
| **Compound Lookups** | Sequential scan $O(N)$ | **Binary Search $O(\log N)$ or Direct Index** |
| **Inlined Primitives** | No (requires pointer jumps & allocs) | **Inlined in 16-byte descriptors** |
| **Zero-Copy Traversal** | No (must decode all tags into heap) | **Yes** (inspect subfields with 0 allocations) |
| **Compression** | Gzip / Zlib | **None (mmap zero-copy), Zstandard, or Gzip** |

---

## Installation & Build

Build both applications with `./scripts/build.sh`. It writes the binaries to
`build/bdf` and `build/bdfgen`.

To run `bdfgen` and `bdf` by name from any directory, source the environment
script from the project root:

```bash
source scripts/env.sh
bdfgen --help
```

To load it automatically in future Bash sessions, add this line to `~/.bashrc`
(adjust the project path if needed):

```bash
source /path/to/BDF-format/scripts/env.sh
```

The script sets `BDFGEN_HOME` to the project's `build/` directory and adds that
directory to `PATH` once.

Run test suite:
```bash
go test -v ./...
```

---

## World Pregenerator CLI (`bdfgen`)

`bdfgen` pregenerates Minecraft worlds by offloading **full 3D voxel chunk generation directly to the GPU (OpenCL)** alongside an asynchronous multi-core CPU compression and disk pipeline.

### GPU + CPU Co-Processing Pipeline
- **GPU (OpenCL 3D Voxel Kernel)**:
  - Dispatches parallel 3D workgroups across all GPU execution units.
  - Generates the 3D block states for all 24 sections ($Y = -64$ to $320$, $16 \times 16 \times 384 = 98,304$ blocks per chunk).
  - Directly generates the 4-bit bitpacked 64-bit integer arrays (`TAG_Long_Array`) in GPU registers, bypassing CPU block loops completely!
- **CPU (Asynchronous Worker Pool)**:
  - Takes GPU-generated voxel sections, wraps into BDF chunk tags, and runs SIMD Zlib compression across all CPU cores.
- **Asynchronous Disk I/O**:
  - Non-blocking channel streams chunks directly into Anvil `.mca` region files.

### Usage
```bash
# Show commands and help
./bdfgen --help
./bdfgen start --help
./bdfgen devices

# Create preferences, then pregenerate a world
./bdfgen init
./bdfgen start --out=./my_world --radius=16 --type=noise --seed=1337

# On Intel Iris / Broadwell GPUs with Mesa Rusticl:
RUSTICL_ENABLE=iris ./bdfgen start --out=./my_world --radius=16 --type=noise

# Toggle GPU acceleration explicitly
./bdfgen start --out=./my_world --radius=16 --gpu=true
./bdfgen start --out=./my_world --radius=16 --gpu=false

# Choose an OpenCL GPU by part of its listed name
./bdfgen devices
./bdfgen start --gpu-device="Iris Pro"

# Rusticl may need the device enabled in the process environment
RUSTICL_ENABLE=iris ./bdfgen start --gpu-device="Iris Pro"

# Fast flat testing chunks
./bdfgen start --out=./flat_world --radius=16 --type=flat

# Install and enable a local datapack zip or directory in the new world
./bdfgen start --out=./my_world --datapack "/path/to/My Datapack.zip"
# Repeat --datapack for multiple packs, or drop paths after the command options

# Regenerate only level.dat in an existing world; region files are untouched
./bdfgen leveldat --out=/path/to/saves/test --seed=9876 --name=test
```

The `leveldat` subcommand replaces only `<world directory>/level.dat`. Set
`--seed` to the existing world's seed. Optional flags include `--spawn-x`,
`--spawn-y`, `--spawn-z`, and `--game-mode` (0 survival, 1 creative, 2
adventure, 3 spectator).

### TOML preferences

Run `./bdfgen init` to create `bdf-pregen.toml` in the current
directory. Use `--out=/path/to/file.toml` to choose another path. Existing
files are never overwritten. Edit the generated file to set your usual world,
generation, and level settings. The program loads `bdf-pregen.toml` from the
current directory automatically; `--config=/path/to/file.toml` selects another
file. Command-line flags override the TOML values. Level preferences include
enabled/disabled datapack IDs, difficulty, spawn, game mode, hardcore, and
world-generation switches. Set `generation.gpu_device` to a device-name
substring, or pass `--gpu-device`; use `bdfgen devices` to see available
OpenCL devices. Leave it blank to select the first available OpenCL GPU.
`generation.gpu_batch_size` or `--gpu-batch-size` controls chunks per GPU
dispatch; try 64, 128, or 256 for your device. The same file is used by the
`leveldat` subcommand.

Set `[world].seed` to pick the deterministic seed and tune the built-in noise
terrain under `[generation]`: `base_height` sets the average surface,
`sea_level` sets the water line, `noise_scale` controls feature width,
`height_scale` controls vertical relief, and `noise_octaves`, `persistence`,
and `lacunarity` control terrain detail. The same terrain values can be
overridden with `bdfgen start` flags such as `--height-scale=60` or
`--noise-scale=0.003`. These settings apply to BDFGen's built-in noise terrain;
they do not execute datapack world-generation rules.

For a broader, taller landscape, try:

```toml
[generation]
base_height = 80
sea_level = 64
noise_scale = 0.003
height_scale = 65.0
noise_octaves = 5
persistence = 0.55
lacunarity = 2.0
```

Add local datapacks with repeatable `--datapack PATH` options or the
`[datapacks].paths` TOML list. BDFGen copies valid packs into the world's
`datapacks/` folder and enables them in `level.dat`. Its built-in terrain
generator does not execute Minecraft datapack worldgen rules, so those rules
apply to chunks Minecraft generates later, not the chunks BDFGen already wrote.

---

## Core CLI Usage (`bdf`)

```bash
# Compile SNBT or JSON to BDF
./bdf compile player.snbt player.bdf
./bdf compile player.snbt player.bdf --compress=zstd

# Decompile BDF to SNBT or JSON
./bdf decompile player.bdf player.snbt
./bdf decompile player.bdf player.json --format=json

# Convert existing Minecraft .dat/.nbt files to BDF
./bdf convert player.dat player.bdf

# Export BDF back to standard Minecraft NBT (.dat)
./bdf export-nbt player.bdf player.dat --nbt-compress=gzip

# Inspect BDF alignment and metadata
./bdf inspect player.bdf

# Run performance benchmarks
./bdf bench player.dat
```

---

## Technical Specification & Minecraft Modding

See [docs/SPECIFICATION.md](docs/SPECIFICATION.md) for full binary layout diagrams, alignment constraints, and the Java Foreign Function & Memory API implementation guide for Minecraft mods.
