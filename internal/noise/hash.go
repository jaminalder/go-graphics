package noise

// Hash01 is deterministic white noise: it maps (seed, ix, iy) to a value in
// [0, 1) with no spatial correlation. Used for grain/stipple lattices.
func Hash01(seed uint64, ix, iy int64) float64 {
	h := seed
	h ^= uint64(ix) * 0x9E3779B97F4A7C15
	h = mix(h)
	h ^= uint64(iy) * 0xC2B2AE3D27D4EB4F
	h = mix(h)
	return float64(h>>11) / (1 << 53)
}

// mix is the splitmix64 finalizer.
func mix(h uint64) uint64 {
	h ^= h >> 30
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 27
	h *= 0x94D049BB133111EB
	h ^= h >> 31
	return h
}
