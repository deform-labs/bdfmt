package region

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/klauspost/compress/zlib"
	"github.com/twlve-secs/bdfmt/pkg/bdf"
	"github.com/twlve-secs/bdfmt/pkg/nbt"
)

const (
	SectorSize          = 4096
	ChunksPerRegionSide = 32
	ChunksPerRegion     = ChunksPerRegionSide * ChunksPerRegionSide // 1024
	HeaderSectors       = 2                                         // 8192 bytes (4KB locations + 4KB timestamps)
	CompressionZlib     = 2
)

// CompressedChunk carries pre-computed and compressed chunk payload
type CompressedChunk struct {
	ChunkX int
	ChunkZ int
	Data   []byte
}

var zlibPool = sync.Pool{
	New: func() interface{} {
		var buf bytes.Buffer
		w, _ := zlib.NewWriterLevel(&buf, zlib.BestSpeed)
		return &pooledCompressor{
			buf: &buf,
			zw:  w,
		}
	},
}

type pooledCompressor struct {
	buf *bytes.Buffer
	zw  *zlib.Writer
}

// CompressChunkToZlib serializes BDF chunk tag to NBT and compresses with fast SIMD Zlib
func CompressChunkToZlib(chunkTag bdf.Tag) ([]byte, error) {
	nbtBytes, err := nbt.Write("", chunkTag, nbt.CompressionNone)
	if err != nil {
		return nil, fmt.Errorf("failed to encode chunk to NBT: %w", err)
	}

	pc := zlibPool.Get().(*pooledCompressor)
	defer zlibPool.Put(pc)

	pc.buf.Reset()
	pc.zw.Reset(pc.buf)

	if _, err := pc.zw.Write(nbtBytes); err != nil {
		_ = pc.zw.Close()
		return nil, err
	}
	if err := pc.zw.Close(); err != nil {
		return nil, err
	}

	res := make([]byte, pc.buf.Len())
	copy(res, pc.buf.Bytes())
	return res, nil
}

// Region represents an Anvil .mca region file
type Region struct {
	mu         sync.Mutex
	path       string
	regionX    int
	regionZ    int
	locations  [ChunksPerRegion]uint32 // (offset << 8) | sectorCount
	timestamps [ChunksPerRegion]uint32
	sectors    []bool // Sector allocation table (free/used)
	file       *os.File
}

// ChunkCoordsToRegion converts chunk (x, z) to region (rx, rz)
func ChunkCoordsToRegion(chunkX, chunkZ int) (int, int) {
	rx := chunkX >> 5
	rz := chunkZ >> 5
	return rx, rz
}

// ChunkIndexInRegion returns 0..1023 index for a chunk inside its region
func ChunkIndexInRegion(chunkX, chunkZ int) int {
	cx := chunkX & 31
	cz := chunkZ & 31
	return cx + cz*32
}

// RegionFileName returns the standard Anvil filename: r.X.Z.mca
func RegionFileName(rx, rz int) string {
	return fmt.Sprintf("r.%d.%d.mca", rx, rz)
}

// Create creates or opens a region file for writing
func Create(dir string, rx, rz int) (*Region, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	path := filepath.Join(dir, RegionFileName(rx, rz))
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open region file: %w", err)
	}

	r := &Region{
		path:    path,
		regionX: rx,
		regionZ: rz,
		file:    f,
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	if info.Size() < SectorSize*HeaderSectors {
		header := make([]byte, SectorSize*HeaderSectors)
		if _, err := f.WriteAt(header, 0); err != nil {
			_ = f.Close()
			return nil, err
		}
		r.sectors = make([]bool, HeaderSectors)
		r.sectors[0] = true
		r.sectors[1] = true
	} else {
		header := make([]byte, SectorSize*HeaderSectors)
		if _, err := f.ReadAt(header, 0); err != nil {
			_ = f.Close()
			return nil, err
		}
		for i := 0; i < ChunksPerRegion; i++ {
			r.locations[i] = binary.BigEndian.Uint32(header[i*4 : (i+1)*4])
			r.timestamps[i] = binary.BigEndian.Uint32(header[SectorSize+i*4 : SectorSize+(i+1)*4])
		}

		totalSectors := int((info.Size() + SectorSize - 1) / SectorSize)
		if totalSectors < HeaderSectors {
			totalSectors = HeaderSectors
		}
		r.sectors = make([]bool, totalSectors)
		r.sectors[0] = true
		r.sectors[1] = true

		for _, loc := range r.locations {
			offset := int(loc >> 8)
			count := int(loc & 0xFF)
			for s := 0; s < count && offset+s < len(r.sectors); s++ {
				r.sectors[offset+s] = true
			}
		}
	}

	return r, nil
}

// Close closes the region file
func (r *Region) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}

// WriteChunkBDF serializes and writes a BDF tag
func (r *Region) WriteChunkBDF(chunkX, chunkZ int, chunkTag bdf.Tag) error {
	zlibPayload, err := CompressChunkToZlib(chunkTag)
	if err != nil {
		return err
	}
	return r.WriteChunkCompressed(chunkX, chunkZ, zlibPayload)
}

// WriteChunkCompressed writes pre-compressed Zlib chunk payload to region
func (r *Region) WriteChunkCompressed(chunkX, chunkZ int, zlibData []byte) error {
	return r.WriteChunksCompressed([]CompressedChunk{{ChunkX: chunkX, ChunkZ: chunkZ, Data: zlibData}})
}

// WriteChunksCompressed writes a same-region batch while holding the region
// lock once. Adjacent sector and header ranges are coalesced into fewer writes.
func (r *Region) WriteChunksCompressed(chunks []CompressedChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	type sectorWrite struct {
		offset int
		data   []byte
	}
	writes := make([]sectorWrite, 0, len(chunks))
	indices := make([]int, 0, len(chunks))
	for _, chunk := range chunks {
		idx := ChunkIndexInRegion(chunk.ChunkX, chunk.ChunkZ)
		totalBytes := 4 + 1 + len(chunk.Data)
		sectorsNeeded := (totalBytes + SectorSize - 1) / SectorSize
		if sectorsNeeded >= 256 {
			return fmt.Errorf("chunk (%d, %d) exceeds max Anvil sector count: %d bytes", chunk.ChunkX, chunk.ChunkZ, totalBytes)
		}
		oldLoc := r.locations[idx]
		if oldLoc != 0 {
			oldOffset, oldCount := int(oldLoc>>8), int(oldLoc&0xFF)
			for s := 0; s < oldCount && oldOffset+s < len(r.sectors); s++ {
				r.sectors[oldOffset+s] = false
			}
		}
		sectorOffset := r.allocateSectors(sectorsNeeded)
		buf := make([]byte, sectorsNeeded*SectorSize)
		binary.BigEndian.PutUint32(buf[:4], uint32(len(chunk.Data)+1))
		buf[4] = CompressionZlib
		copy(buf[5:], chunk.Data)
		writes = append(writes, sectorWrite{offset: sectorOffset, data: buf})
		r.locations[idx] = uint32((sectorOffset << 8) | sectorsNeeded)
		r.timestamps[idx] = uint32(time.Now().Unix())
		indices = append(indices, idx)
	}

	// Sector allocation generally yields consecutive offsets for a fresh world.
	// Merge each consecutive run into one positional write.
	sort.Slice(writes, func(i, j int) bool { return writes[i].offset < writes[j].offset })
	var runOffset int
	var run []byte
	flushRun := func() error {
		if len(run) == 0 {
			return nil
		}
		if _, err := r.file.WriteAt(run, int64(runOffset*SectorSize)); err != nil {
			return fmt.Errorf("failed to write chunk sectors: %w", err)
		}
		run = nil
		return nil
	}
	for _, write := range writes {
		if len(run) == 0 {
			runOffset = write.offset
		} else if write.offset*SectorSize != runOffset*SectorSize+len(run) {
			if err := flushRun(); err != nil {
				return err
			}
			runOffset = write.offset
		}
		run = append(run, write.data...)
	}
	if err := flushRun(); err != nil {
		return err
	}

	// Coalesce adjacent location and timestamp entries in the two header sectors.
	sort.Ints(indices)
	for _, timestamp := range []bool{false, true} {
		for i := 0; i < len(indices); {
			start := i
			for i+1 < len(indices) && indices[i+1] == indices[i]+1 {
				i++
			}
			values := make([]byte, (i-start+1)*4)
			for j := start; j <= i; j++ {
				idx := indices[j]
				value := r.locations[idx]
				if timestamp {
					value = r.timestamps[idx]
				}
				binary.BigEndian.PutUint32(values[(j-start)*4:], value)
			}
			headerOffset := int64(indices[start] * 4)
			if timestamp {
				headerOffset += SectorSize
			}
			if _, err := r.file.WriteAt(values, headerOffset); err != nil {
				return fmt.Errorf("failed to update region header: %w", err)
			}
			i++
		}
	}
	return nil
}

func (r *Region) allocateSectors(count int) int {
	freeStreak := 0
	startSector := -1

	for i := HeaderSectors; i < len(r.sectors); i++ {
		if !r.sectors[i] {
			if freeStreak == 0 {
				startSector = i
			}
			freeStreak++
			if freeStreak == count {
				for s := 0; s < count; s++ {
					r.sectors[startSector+s] = true
				}
				return startSector
			}
		} else {
			freeStreak = 0
		}
	}

	startSector = len(r.sectors)
	for s := 0; s < count; s++ {
		r.sectors = append(r.sectors, true)
	}
	return startSector
}

// ReadChunkBDF reads and deserializes a chunk as a BDF Tag
func (r *Region) ReadChunkBDF(chunkX, chunkZ int) (bdf.Tag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	idx := ChunkIndexInRegion(chunkX, chunkZ)
	loc := r.locations[idx]
	if loc == 0 {
		return nil, nil
	}

	offset := int64(loc>>8) * SectorSize
	count := int(loc & 0xFF)

	sectorBuf := make([]byte, count*SectorSize)
	if _, err := r.file.ReadAt(sectorBuf, offset); err != nil {
		return nil, err
	}

	dataLength := binary.BigEndian.Uint32(sectorBuf[0:4])
	if dataLength < 1 {
		return nil, fmt.Errorf("invalid chunk data length: %d", dataLength)
	}
	compression := sectorBuf[4]
	if compression != CompressionZlib {
		return nil, fmt.Errorf("unsupported chunk compression: %d", compression)
	}

	zr, err := zlib.NewReader(bytes.NewReader(sectorBuf[5 : 5+dataLength-1]))
	if err != nil {
		return nil, err
	}
	decompressed, err := io.ReadAll(zr)
	_ = zr.Close()
	if err != nil {
		return nil, err
	}

	_, tag, err := nbt.Read(decompressed)
	return tag, err
}
