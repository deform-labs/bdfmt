package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
	"github.com/twlve-secs/bdfmt/pkg/compiler"
	"github.com/twlve-secs/bdfmt/pkg/nbt"
)

func printUsage() {
	fmt.Println(`
		BDF (Binary Data Format) - 64-bit Optimized NBT Alternative
		Usage:
		  bdf compile <input.snbt|input.json> <output.bdf> [options]
		  bdf decompile <input.bdf> [output.snbt|output.json] [options]
		  bdf convert <input.nbt|input.dat> <output.bdf> [options]
		  bdf export-nbt <input.bdf> <output.nbt> [options]
		  bdf inspect <input.bdf>
		  bdf bench <input.nbt|input.dat>

		Options:
		  --compress    Compression mode for BDF: "none" (default, zero-copy), "zstd", "gzip"
		  --format      Decompilation text format: "snbt" (default) or "json"
		  --indent      Pretty-print formatted output (default: true)
		  --nbt-compress Compression for exported NBT: "gzip" (default), "zlib", "none"
  `)
}

func parseCompression(s string) uint16 {
	switch strings.ToLower(s) {
	case "zstd":
		return bdf.CompressionZstd
	case "gzip":
		return bdf.CompressionGzip
	case "none":
		return bdf.CompressionNone
	default:
		return bdf.CompressionNone
	}
}

func parseNBTCompression(s string) nbt.Compression {
	switch strings.ToLower(s) {
	case "gzip":
		return nbt.CompressionGzip
	case "zlib":
		return nbt.CompressionZlib
	case "none":
		return nbt.CompressionNone
	default:
		return nbt.CompressionGzip
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "compile":
		fs := flag.NewFlagSet("compile", flag.ExitOnError)
		compressFlag := fs.String("compress", "none", "Compression: none, zstd, gzip")
		formatFlag := fs.String("format", "snbt", "Input format: snbt, json")
		_ = fs.Parse(os.Args[2:])
		args := fs.Args()
		if len(args) < 2 {
			fmt.Println("Error: compile requires <input> and <output>")
			printUsage()
			os.Exit(1)
		}
		opts := compiler.Options{
			Compression: parseCompression(*compressFlag),
			Format:      *formatFlag,
		}
		if err := compiler.CompileFile(args[0], args[1], opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully compiled %s -> %s (compression: %s)\n", args[0], args[1], *compressFlag)

	case "decompile":
		fs := flag.NewFlagSet("decompile", flag.ExitOnError)
		formatFlag := fs.String("format", "snbt", "Output format: snbt, json")
		indentFlag := fs.Bool("indent", true, "Pretty print output")
		_ = fs.Parse(os.Args[2:])
		args := fs.Args()
		if len(args) < 1 {
			fmt.Println("Error: decompile requires <input.bdf>")
			printUsage()
			os.Exit(1)
		}
		outputFile := ""
		if len(args) >= 2 {
			outputFile = args[1]
		}
		opts := compiler.Options{
			Format: *formatFlag,
			Indent: *indentFlag,
		}
		out, err := compiler.DecompileFile(args[0], outputFile, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if outputFile == "" {
			fmt.Print(out)
		} else {
			fmt.Printf("Successfully decompiled %s -> %s\n", args[0], outputFile)
		}

	case "convert":
		fs := flag.NewFlagSet("convert", flag.ExitOnError)
		compressFlag := fs.String("compress", "none", "Compression: none, zstd, gzip")
		_ = fs.Parse(os.Args[2:])
		args := fs.Args()
		if len(args) < 2 {
			fmt.Println("Error: convert requires <input.nbt> and <output.bdf>")
			printUsage()
			os.Exit(1)
		}
		opts := compiler.Options{
			Compression: parseCompression(*compressFlag),
		}
		if err := compiler.ConvertNBTToBDF(args[0], args[1], opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully converted NBT %s -> BDF %s\n", args[0], args[1])

	case "export-nbt":
		fs := flag.NewFlagSet("export-nbt", flag.ExitOnError)
		nbtCompressFlag := fs.String("nbt-compress", "gzip", "NBT compression: gzip, zlib, none")
		_ = fs.Parse(os.Args[2:])
		args := fs.Args()
		if len(args) < 2 {
			fmt.Println("Error: export-nbt requires <input.bdf> and <output.nbt>")
			printUsage()
			os.Exit(1)
		}
		nbtComp := parseNBTCompression(*nbtCompressFlag)
		if err := compiler.ConvertBDFToNBT(args[0], args[1], nbtComp); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully exported BDF %s -> NBT %s\n", args[0], args[1])

	case "inspect":
		if len(os.Args) < 3 {
			fmt.Println("Error: inspect requires <input.bdf>")
			os.Exit(1)
		}
		info, err := compiler.Inspect(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("========== BDF File Inspection ==========")
		fmt.Printf("File:               %s\n", os.Args[2])
		fmt.Printf("File Size:          %d bytes\n", info.FileSize)
		fmt.Printf("Version:            %d\n", info.Version)
		fmt.Printf("Compression:        %s\n", info.Compression)
		fmt.Printf("Root Offset:        0x%X (8-byte aligned: %v)\n", info.RootOffset, info.RootOffset%8 == 0)
		fmt.Printf("String Table:       Offset 0x%X, Size %d bytes\n", info.StringTableOffset, info.StringTableSize)
		fmt.Printf("Uncompressed Size:  %d bytes\n", info.UncompressedSize)
		fmt.Printf("Total Tags:         %d\n", info.TotalTags)
		fmt.Printf("Root Keys (%d):      %s\n", len(info.RootKeys), strings.Join(info.RootKeys, ", "))
		fmt.Println("=========================================")

	case "bench":
		if len(os.Args) < 3 {
			fmt.Println("Error: bench requires <input.nbt|input.dat>")
			os.Exit(1)
		}
		runBenchmark(os.Args[2])

	default:
		fmt.Printf("Unknown command: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func runBenchmark(nbtPath string) {
	fmt.Printf("Running performance benchmark on: %s\n", nbtPath)
	rawNBT, err := os.ReadFile(nbtPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		return
	}

	// 1. Benchmark NBT Read
	start := time.Now()
	iterations := 100
	var rootTag bdf.Tag
	for i := 0; i < iterations; i++ {
		_, rootTag, err = nbt.Read(rawNBT)
		if err != nil {
			fmt.Fprintf(os.Stderr, "NBT read error: %v\n", err)
			return
		}
	}
	nbtReadTime := time.Since(start) / time.Duration(iterations)

	// 2. Benchmark BDF Write (Uncompressed 64-bit zero-copy)
	start = time.Now()
	var bdfRaw []byte
	for i := 0; i < iterations; i++ {
		bdfRaw, err = bdf.Write(rootTag, bdf.WriterOptions{Compression: bdf.CompressionNone})
		if err != nil {
			fmt.Fprintf(os.Stderr, "BDF write error: %v\n", err)
			return
		}
	}
	bdfWriteTime := time.Since(start) / time.Duration(iterations)

	// 3. Benchmark BDF Read (Full decode)
	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, err = bdf.Read(bdfRaw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "BDF read error: %v\n", err)
			return
		}
	}
	bdfReadTime := time.Since(start) / time.Duration(iterations)

	// 4. Benchmark BDF Zero-Copy View (Instant random access!)
	start = time.Now()
	viewIterations := 10000
	for i := 0; i < viewIterations; i++ {
		view, err := bdf.NewView(bdfRaw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "BDF view error: %v\n", err)
			return
		}
		rootComp, err := view.RootCompound()
		if err == nil && rootComp.Len() > 0 {
			keys := rootComp.Keys()
			if len(keys) > 0 {
				_ = keys[0]
			}
		}
	}
	bdfViewTime := time.Since(start) / time.Duration(viewIterations)

	// 5. Compare sizes with Zstandard
	bdfZstd, _ := bdf.Write(rootTag, bdf.WriterOptions{Compression: bdf.CompressionZstd})

	fmt.Println("\n------------------- Benchmark Results -------------------")
	fmt.Printf("Input NBT File Size:         %d bytes\n", len(rawNBT))
	fmt.Printf("BDF (Zero-Copy) File Size:   %d bytes\n", len(bdfRaw))
	fmt.Printf("BDF (Zstd) File Size:        %d bytes\n", len(bdfZstd))
	fmt.Printf("NBT Parse Time (avg):        %v\n", nbtReadTime)
	fmt.Printf("BDF Write Time (avg):        %v\n", bdfWriteTime)
	fmt.Printf("BDF Full Parse Time (avg):   %v\n", bdfReadTime)
	fmt.Printf("BDF Zero-Copy Lookup (avg):  %v (instant random access)\n", bdfViewTime)
	fmt.Println("---------------------------------------------------------")
}
