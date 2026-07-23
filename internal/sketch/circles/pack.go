package circles

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/geom"
)

// pack fills [0, aspect]×[0,1] with non-overlapping circles by rejection
// sampling into a geom.Index. The radius cap shrinks over the attempt
// budget (large anchors first, small fillers later); each attempt tries a
// fixed number of positions and is discarded if none fits. The fixed
// budget keeps the whole loop deterministic for a given RNG.
func (s *Sketch) pack(rng *rand.Rand, aspect float64) *geom.Index {
	idx := geom.NewIndex(aspect, 1, s.MaxR)
	for a := 0; a < s.Attempts; a++ {
		shrink := 1 - float64(a)/float64(s.Attempts)
		rCap := s.MinR + (s.MaxR-s.MinR)*shrink*shrink
		r := s.MinR + (rCap-s.MinR)*math.Pow(rng.Float64(), 1.5)

		for try := 0; try < s.PosTries; try++ {
			margin := r + s.Gap
			cx := margin + rng.Float64()*(aspect-2*margin)
			cy := margin + rng.Float64()*(1-2*margin)
			if idx.FitsWithGap(geom.Circle{X: cx, Y: cy, R: r}, s.Gap) {
				idx.Insert(geom.Circle{X: cx, Y: cy, R: r})
				break
			}
		}
	}
	return idx
}
