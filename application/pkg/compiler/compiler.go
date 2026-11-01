package compiler

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
	"github.com/twlve-secs/bdfmt/pkg/jsonbdf"
	"github.com/twlve-secs/bdfmt/pkg/nbt"
	"github.com/twlve-secs/bdfmt/pkg/snbt"
)

// Options contains settings for compilation/decompilation
type Options struct {
	Compression uint16 // bdf.CompressionNone, bdf.CompressionZstd, bdf.CompressionGzip
	Format      string // "snbt" or "json"
	Indent      bool
}

// CompileFile compiles an input text file (SNBT or JSON) into a BDF binary file
func CompileFile(inputFile, outputFile string, opts Options) error {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	var root bdf.Tag
	ext := strings.ToLower(filepath.Ext(inputFile))
	if ext == ".json" || opts.Format == "json" {
		root, err = jsonbdf.FromJSON(data)
	} else {
		root, err = snbt.Parse(string(data))
	}
	if err != nil {
		return fmt.Errorf("failed to parse input text: %w", err)
	}

	bdfBytes, err := bdf.Write(root, bdf.WriterOptions{
		Compression: opts.Compression,
	})
	if err != nil {
		return fmt.Errorf("failed to compile to BDF: %w", err)
	}

	if err := os.WriteFile(outputFile, bdfBytes, 0644); err != nil {
		return fmt.Errorf("failed to write output BDF: %w", err)
	}

	return nil
}

// DecompileFile decompiles a BDF binary file into human-readable text (SNBT or JSON)
func DecompileFile(inputFile, outputFile string, opts Options) (string, error) {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return "", fmt.Errorf("failed to read BDF file: %w", err)
	}

	root, err := bdf.Read(data)
	if err != nil {
		return "", fmt.Errorf("failed to decode BDF file: %w", err)
	}

	var textOutput string
	if opts.Format == "json" {
		jsonBytes, err := jsonbdf.ToJSON(root, opts.Indent)
		if err != nil {
			return "", fmt.Errorf("failed to encode to JSON: %w", err)
		}
		textOutput = string(jsonBytes)
	} else {
		textOutput = snbt.Print(root, opts.Indent)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(textOutput), 0644); err != nil {
			return "", fmt.Errorf("failed to write decompiled file: %w", err)
		}
	}

	return textOutput, nil
}

// ConvertNBTToBDF converts an existing Minecraft NBT file (.dat, .nbt) to BDF
func ConvertNBTToBDF(nbtFile, bdfFile string, opts Options) error {
	data, err := os.ReadFile(nbtFile)
	if err != nil {
		return fmt.Errorf("failed to read NBT file: %w", err)
	}

	_, rootTag, err := nbt.Read(data)
	if err != nil {
		return fmt.Errorf("failed to parse NBT: %w", err)
	}

	bdfBytes, err := bdf.Write(rootTag, bdf.WriterOptions{
		Compression: opts.Compression,
	})
	if err != nil {
		return fmt.Errorf("failed to encode BDF: %w", err)
	}

	if err := os.WriteFile(bdfFile, bdfBytes, 0644); err != nil {
		return fmt.Errorf("failed to write BDF output: %w", err)
	}

	return nil
}

// ConvertBDFToNBT converts a BDF file back to standard Minecraft NBT format (.dat, .nbt)
func ConvertBDFToNBT(bdfFile, nbtFile string, nbtCompression nbt.Compression) error {
	data, err := os.ReadFile(bdfFile)
	if err != nil {
		return fmt.Errorf("failed to read BDF file: %w", err)
	}

	rootTag, err := bdf.Read(data)
	if err != nil {
		return fmt.Errorf("failed to decode BDF: %w", err)
	}

	nbtBytes, err := nbt.Write("", rootTag, nbtCompression)
	if err != nil {
		return fmt.Errorf("failed to encode NBT: %w", err)
	}

	if err := os.WriteFile(nbtFile, nbtBytes, 0644); err != nil {
		return fmt.Errorf("failed to write NBT file: %w", err)
	}

	return nil
}

// InspectInfo holds diagnostic metadata about a BDF file
type InspectInfo struct {
	Version           uint16
	Flags             uint16
	Compression       string
	RootOffset        uint64
	StringTableOffset uint64
	StringTableSize   uint64
	UncompressedSize  uint64
	TotalTags         uint64
	FileSize          int
	RootKeys          []string
}

// Inspect parses and returns metadata and alignment statistics of a BDF file
func Inspect(bdfFile string) (*InspectInfo, error) {
	data, err := os.ReadFile(bdfFile)
	if err != nil {
		return nil, err
	}

	if len(data) < bdf.HeaderSize {
		return nil, fmt.Errorf("file is smaller than 64-byte BDF header")
	}

	version := binary.LittleEndian.Uint16(data[4:6])
	flags := binary.LittleEndian.Uint16(data[6:8])
	rootOff := binary.LittleEndian.Uint64(data[8:16])
	strOff := binary.LittleEndian.Uint64(data[16:24])
	strSize := binary.LittleEndian.Uint64(data[24:32])
	uncompSize := binary.LittleEndian.Uint64(data[32:40])
	totalTags := binary.LittleEndian.Uint64(data[40:48])

	compMode := (flags & bdf.FlagCompressionMask) >> 1
	var compStr string
	switch compMode {
	case bdf.CompressionNone:
		compStr = "None (Raw Zero-Copy / mmap ready)"
	case bdf.CompressionZstd:
		compStr = "Zstandard"
	case bdf.CompressionGzip:
		compStr = "Gzip"
	default:
		compStr = fmt.Sprintf("Unknown (%d)", compMode)
	}

	view, err := bdf.NewView(data)
	var rootKeys []string
	if err == nil {
		if rootComp, err := view.RootCompound(); err == nil {
			rootKeys = rootComp.Keys()
		}
	}

	return &InspectInfo{
		Version:           version,
		Flags:             flags,
		Compression:       compStr,
		RootOffset:        rootOff,
		StringTableOffset: strOff,
		StringTableSize:   strSize,
		UncompressedSize:  uncompSize,
		TotalTags:         totalTags,
		FileSize:          len(data),
		RootKeys:          rootKeys,
	}, nil
}
