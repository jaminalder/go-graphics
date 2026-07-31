// Package mathx holds the small numeric helpers shared across the art
// packages: clamping, interval remapping, smoothstep. Stdlib-only leaf.
package mathx

// Clamp01 limits v to [0, 1].
func Clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Remap maps x from [lo, hi] to [0, 1], unclamped — callers that need
// clamping compose with Clamp01 or rely on their gradient's own clamp.
func Remap(x, lo, hi float64) float64 {
	return (x - lo) / (hi - lo)
}

// Smoothstep is the standard cubic step: 0 for x ≤ lo, 1 for x ≥ hi,
// smooth in between.
func Smoothstep(lo, hi, x float64) float64 {
	t := Clamp01((x - lo) / (hi - lo))
	return t * t * (3 - 2*t)
}

// Rescale maps value from [oldMin, oldMax] to [newMin, newMax], clamping
// to the source interval first so callers can pass out-of-range values
// safely. Unlike Remap it does not assume a unit target, which is how
// parameter tables are usually written: "a density of 0.1..1 becomes a
// variance of 0.5..1.2".
func Rescale(value, oldMin, oldMax, newMin, newMax float64) float64 {
	lo, hi := min(oldMin, oldMax), max(oldMin, oldMax)
	v := min(max(value, lo), hi)
	return newMin + (v-oldMin)*(newMax-newMin)/(oldMax-oldMin)
}
