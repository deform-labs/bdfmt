package worldgen

import (
	"math"
)

// PerlinNoise provides deterministic 2D and 3D Perlin gradient noise
type PerlinNoise struct {
	perm [512]int
}

// Perm returns the 512-element permutation table
func (p *PerlinNoise) Perm() [512]int {
	return p.perm
}

// NewPerlinNoise creates a seeded Perlin noise generator
func NewPerlinNoise(seed int64) *PerlinNoise {
	p := &PerlinNoise{}
	var permutation [256]int
	for i := 0; i < 256; i++ {
		permutation[i] = i
	}

	// Pseudo-random shuffle with seed
	s := uint64(seed)
	for i := 255; i > 0; i-- {
		s = s*6364136223846793005 + 1442695040888963407
		j := int((s >> 32) % uint64(i+1))
		permutation[i], permutation[j] = permutation[j], permutation[i]
	}

	for i := 0; i < 256; i++ {
		p.perm[i] = permutation[i]
		p.perm[i+256] = permutation[i]
	}
	return p
}

func fade(t float64) float64 {
	return t * t * t * (t*(t*6-15) + 10)
}

func lerp(t, a, b float64) float64 {
	return a + t*(b-a)
}

func grad2D(hash int, x, y float64) float64 {
	h := hash & 7
	u := x
	if h >= 4 {
		u = y
	}
	v := y
	if h >= 4 {
		v = x
	}
	if (h & 1) != 0 {
		u = -u
	}
	if (h & 2) != 0 {
		v = -v
	}
	return u + v
}

// Noise2D samples 2D Perlin noise at (x, y), returning value in range [-1, 1]
func (p *PerlinNoise) Noise2D(x, y float64) float64 {
	xi := int(math.Floor(x)) & 255
	yi := int(math.Floor(y)) & 255

	xf := x - math.Floor(x)
	yf := y - math.Floor(y)

	u := fade(xf)
	v := fade(yf)

	aa := p.perm[p.perm[xi]+yi]
	ab := p.perm[p.perm[xi]+yi+1]
	ba := p.perm[p.perm[xi+1]+yi]
	bb := p.perm[p.perm[xi+1]+yi+1]

	x1 := lerp(u, grad2D(aa, xf, yf), grad2D(ba, xf-1, yf))
	x2 := lerp(u, grad2D(ab, xf, yf-1), grad2D(bb, xf-1, yf-1))

	return lerp(v, x1, x2)
}

// OctaveNoise2D computes multi-octave Fractal Brownian Motion (FBM)
func (p *PerlinNoise) OctaveNoise2D(x, y float64, octaves int, persistence, lacunarity float64) float64 {
	total := 0.0
	frequency := 1.0
	amplitude := 1.0
	maxValue := 0.0

	for i := 0; i < octaves; i++ {
		total += p.Noise2D(x*frequency, y*frequency) * amplitude
		maxValue += amplitude
		amplitude *= persistence
		frequency *= lacunarity
	}

	return total / maxValue
}
