package opencl

import (
	"testing"
)

func TestOpenCLInit(t *testing.T) {
	engine, err := InitGPU()
	if err != nil {
		t.Logf("OpenCL GPU not available on this machine (expected if no ICD driver installed): %v", err)
		return
	}
	defer engine.Close()

	info := engine.Info()
	t.Logf("Successfully initialized OpenCL GPU: %s (%s, %d compute units, %d MHz)",
		info.DeviceName, info.VendorName, info.ComputeUnits, info.ClockFrequency)

	// Test 3D chunk voxel generation
	var perm [512]int
	for i := 0; i < 512; i++ {
		perm[i] = i % 256
	}

	coords := [][2]int{
		{0, 0},
		{1, 0},
	}

	results, err := engine.GenerateChunkVoxelsBatch(coords, 0.005, 64.0, 68.0, 26.0, 4, 0.5, 2.0, 94.0, perm)
	if err != nil {
		t.Fatalf("GenerateChunkVoxelsBatch failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 chunk results, got %d", len(results))
	}
}
