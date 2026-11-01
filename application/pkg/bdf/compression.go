package bdf

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// CompressPayload compresses data using the specified compression mode
func CompressPayload(data []byte, mode uint16) ([]byte, error) {
	switch mode {
	case CompressionNone:
		return data, nil

	case CompressionZstd:
		var buf bytes.Buffer
		enc, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.SpeedDefault))
		if err != nil {
			return nil, fmt.Errorf("zstd encoder error: %w", err)
		}
		if _, err := enc.Write(data); err != nil {
			_ = enc.Close()
			return nil, fmt.Errorf("zstd write error: %w", err)
		}
		if err := enc.Close(); err != nil {
			return nil, fmt.Errorf("zstd close error: %w", err)
		}
		return buf.Bytes(), nil

	case CompressionGzip:
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(data); err != nil {
			_ = gw.Close()
			return nil, fmt.Errorf("gzip write error: %w", err)
		}
		if err := gw.Close(); err != nil {
			return nil, fmt.Errorf("gzip close error: %w", err)
		}
		return buf.Bytes(), nil

	default:
		return nil, fmt.Errorf("unsupported compression mode: %d", mode)
	}
}

// DecompressPayload decompresses payload based on compression mode
func DecompressPayload(compressed []byte, mode uint16) ([]byte, error) {
	switch mode {
	case CompressionNone:
		return compressed, nil

	case CompressionZstd:
		dec, err := zstd.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return nil, fmt.Errorf("zstd reader error: %w", err)
		}
		defer dec.Close()
		decompressed, err := io.ReadAll(dec)
		if err != nil {
			return nil, fmt.Errorf("zstd decompress error: %w", err)
		}
		return decompressed, nil

	case CompressionGzip:
		gr, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return nil, fmt.Errorf("gzip reader error: %w", err)
		}
		defer gr.Close()
		decompressed, err := io.ReadAll(gr)
		if err != nil {
			return nil, fmt.Errorf("gzip decompress error: %w", err)
		}
		return decompressed, nil

	default:
		return nil, fmt.Errorf("unsupported compression mode: %d", mode)
	}
}
