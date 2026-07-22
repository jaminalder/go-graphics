package noise

import "math"

// Worley salts for the feature-point jitter hashes.
const (
	worleySaltX = 0x63726b78 // "crkx"
	worleySaltY = 0x63726b79 // "crky"
)

// Worley evaluates cell noise at (x, y) in cell units (caller scales
// coordinates by the desired cell frequency): each unit cell holds one
// deterministic feature point, and the returned f1 ≤ f2 are the distances
// to the nearest and second-nearest points. The cell borders — where
// f2−f1 approaches 0 — form a crack network.
func Worley(seed uint64, x, y float64) (f1, f2 float64) {
	cx, cy := int64(math.Floor(x)), int64(math.Floor(y))
	f1, f2 = math.MaxFloat64, math.MaxFloat64
	for j := cy - 1; j <= cy+1; j++ {
		for i := cx - 1; i <= cx+1; i++ {
			px := float64(i) + Hash01(seed^worleySaltX, i, j)
			py := float64(j) + Hash01(seed^worleySaltY, i, j)
			dx, dy := px-x, py-y
			d := math.Sqrt(dx*dx + dy*dy)
			switch {
			case d < f1:
				f1, f2 = d, f1
			case d < f2:
				f2 = d
			}
		}
	}
	return f1, f2
}
