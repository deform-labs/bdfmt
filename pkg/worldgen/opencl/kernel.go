package opencl

// KernelSource is the OpenCL C kernel for GPU voxel chunk generation
const KernelSource = `
int perm_hash(int x, int y, __constant const int* perm) {
    return perm[(perm[x & 255] + y) & 511];
}

float grad2D(int hash, float x, float y) {
    int h = hash & 7;
    float u = h < 4 ? x : y;
    float v = h < 4 ? y : x;
    return ((h & 1) ? -u : u) + ((h & 2) ? -v : v);
}

float fade(float t) {
    return t * t * t * (t * (t * 6.0f - 15.0f) + 10.0f);
}

float lerp_f(float t, float a, float b) {
    return a + t * (b - a);
}

// 4-octave Perlin noise evaluated on GPU
float sample_noise_2d(float worldX, float worldZ, float frequency, int octaves, float persistence, float lacunarity, __constant const int* perm) {
    float total = 0.0f;
    float freq = frequency;
    float amp = 1.0f;
    float maxAmp = 0.0f;

	for (int o = 0; o < octaves; o++) {
        float nx = worldX * freq;
        float nz = worldZ * freq;
        int xi = (int)floor(nx) & 255;
        int zi = (int)floor(nz) & 255;
        float xf = nx - floor(nx);
        float zf = nz - floor(nz);
        float u = fade(xf);
        float v = fade(zf);

        int aa = perm[(perm[xi] + zi) & 511];
        int ab = perm[(perm[xi] + zi + 1) & 511];
        int ba = perm[(perm[xi + 1] + zi) & 511];
        int bb = perm[(perm[xi + 1] + zi + 1) & 511];

        float x1 = lerp_f(u, grad2D(aa, xf, zf), grad2D(ba, xf - 1.0f, zf));
        float x2 = lerp_f(u, grad2D(ab, xf, zf - 1.0f), grad2D(bb, xf - 1.0f, zf - 1.0f));
        total += lerp_f(v, x1, x2) * amp;
        maxAmp += amp;
		amp *= persistence;
		freq *= lacunarity;
    }
    return total / maxAmp;
}

// Block palette IDs:
// 0: minecraft:air
// 1: minecraft:bedrock
// 2: minecraft:deepslate
// 3: minecraft:stone
// 4: minecraft:dirt
// 5: minecraft:grass_block
// 6: minecraft:sand
// 7: minecraft:water

// Section status types:
// 0: All Air
// 1: All Deepslate
// 2: All Stone
// 3: Mixed Palette (256 longs generated)

// First pass: calculate the 16x16 height map once for each chunk.
__kernel void generate_chunk_heights(
    int numChunks,
    __global const int2* chunkCoords,
    float frequency,
    float seaLevel,
    float baseHeight,
    float heightScale,
    int octaves,
    float persistence,
    float lacunarity,
    __constant const int* perm,
    __global int* surfaceHeights // [numChunks * 256], indexed Z then X
) {
    int localX = get_global_id(0);
    int localZ = get_global_id(1);
    int chunkId = get_global_id(2);
    if (chunkId >= numChunks || localX >= 16 || localZ >= 16) return;

    int2 coord = chunkCoords[chunkId];
    int worldX = coord.x * 16 + localX;
    int worldZ = coord.y * 16 + localZ;
    float noise = sample_noise_2d((float)worldX, (float)worldZ, frequency, octaves, persistence, lacunarity, perm);
    int index = chunkId * 256 + localZ * 16 + localX;
    surfaceHeights[index] = (int)round(baseHeight + noise * heightScale);
}

// Second pass: parallelize across each row, section, and chunk, using the cached height map.
__kernel void generate_chunk_voxels(
    int numChunks,
    float seaLevel,
    float maximumSurfaceHeight,
    __global const int* surfaceHeights,
    __global uchar* outSectionTypes,
    __global ulong* outSectionData
) {
    int lineIdx = get_global_id(0); // localY * 16 + localZ
    int secIdx = get_global_id(1);
    int chunkId = get_global_id(2);
    if (chunkId >= numChunks || secIdx >= 24 || lineIdx >= 256) return;

    int localY = lineIdx >> 4;
    int localZ = lineIdx & 15;
    int worldY = -64 + secIdx * 16 + localY;
    ulong packedLong = 0;
    for (int localX = 0; localX < 16; localX++) {
        int height = surfaceHeights[chunkId * 256 + localZ * 16 + localX];
        uint blockId = 0;
        if (worldY == -64) blockId = 1;
        else if (worldY < 0) blockId = 2;
        else if (worldY <= height - 4) blockId = 3;
        else if (worldY < height) blockId = 4;
        else if (worldY == height) blockId = height >= (int)seaLevel ? 5 : 6;
        else if (worldY <= (int)seaLevel) blockId = 7;
        packedLong |= ((ulong)(blockId & 0xF)) << (localX * 4);
    }

    size_t outOffset = ((size_t)chunkId * 24 + (size_t)secIdx) * 256 + (size_t)lineIdx;
    outSectionData[outOffset] = packedLong;

    if (lineIdx == 0) {
        int secBaseY = -64 + secIdx * 16;
        uchar secType = 3;
        if (secBaseY > (int)maximumSurfaceHeight) secType = 0;
        else if (secBaseY + 15 < -1 && secBaseY > -60) secType = 1;
        outSectionTypes[chunkId * 24 + secIdx] = secType;
    }
}
`
