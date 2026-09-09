// Package artwork owns fresh definitions and canonical edition recipes.
package artwork

import (
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/circles"
	"github.com/jaminalder/go-graphics/internal/sketch/contour"
	"github.com/jaminalder/go-graphics/internal/sketch/drift"
	"github.com/jaminalder/go-graphics/internal/sketch/flame"
	"github.com/jaminalder/go-graphics/internal/sketch/foam"
	"github.com/jaminalder/go-graphics/internal/sketch/glaze"
	"github.com/jaminalder/go-graphics/internal/sketch/hatchbook"
	"github.com/jaminalder/go-graphics/internal/sketch/iris"
	"github.com/jaminalder/go-graphics/internal/sketch/pools"
	"github.com/jaminalder/go-graphics/internal/sketch/qql"
	"github.com/jaminalder/go-graphics/internal/sketch/riffle"
	"github.com/jaminalder/go-graphics/internal/sketch/rounds"
	"github.com/jaminalder/go-graphics/internal/sketch/scree"
	"github.com/jaminalder/go-graphics/internal/sketch/shallows"
	"github.com/jaminalder/go-graphics/internal/sketch/shoal"
	"github.com/jaminalder/go-graphics/internal/sketch/tapestry"
	"github.com/jaminalder/go-graphics/internal/sketch/warp"
)

// Registry constructs factory definitions for all local artworks. Publication is separate.
func Registry() *sketch.Registry {
	return sketch.NewFactories(
		func() sketch.Sketch { return circles.New() },
		func() sketch.Sketch { return contour.New() },
		func() sketch.Sketch { return drift.New() },
		func() sketch.Sketch { return flame.New() },
		func() sketch.Sketch { return foam.New() },
		func() sketch.Sketch { return glaze.New() },
		func() sketch.Sketch { return hatchbook.New() },
		func() sketch.Sketch { return iris.New() },
		func() sketch.Sketch { return pools.New() },
		func() sketch.Sketch { return qql.New() },
		func() sketch.Sketch { return riffle.New() },
		func() sketch.Sketch { return rounds.New() },
		func() sketch.Sketch { return scree.New() },
		func() sketch.Sketch { return shallows.New() },
		func() sketch.Sketch { return shoal.New() },
		func() sketch.Sketch { return tapestry.New() },
		func() sketch.Sketch { return warp.New() },
	)
}
