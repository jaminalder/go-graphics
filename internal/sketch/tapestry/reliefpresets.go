package tapestry

// Named relief looks, chosen by visual comparison (tools/reliefexp renders
// them side by side). "baseline" is the default; "deep-carve" is the
// favored stronger-3D variant (user review 2026-07-22).

type reliefPreset struct {
	Name   string
	Params ReliefParams
}

func reliefPresets() []reliefPreset {
	base := DefaultReliefParams()
	mod := func(name string, f func(*ReliefParams)) reliefPreset {
		p := base
		f(&p)
		return reliefPreset{name, p}
	}
	return []reliefPreset{
		{"baseline", base},
		mod("low-sun-west", func(p *ReliefParams) {
			// Late-afternoon raking light from the left: long, dramatic relief.
			p.LightDir = [3]float64{-1, -0.15, 0.35}
			p.Ambient = 0.45
		}),
		mod("noon-soft", func(p *ReliefParams) {
			// High overhead light, almost shadowless; edges do the talking.
			p.LightDir = [3]float64{-0.1, -0.1, 1.5}
			p.Ambient = 0.75
			p.Spec = 0.05
		}),
		mod("southeast-light", func(p *ReliefParams) {
			// Light from bottom-right — inverts the perceived height.
			p.LightDir = [3]float64{0.7, 0.7, 0.6}
		}),
		mod("deep-carve", func(p *ReliefParams) {
			// Exaggerated terrain slope, darker shadows: strong 3D.
			p.Slope = 0.12
			p.Ambient = 0.50
		}),
		mod("gentle-emboss", func(p *ReliefParams) {
			// Barely-there relief: soft wax-seal emboss.
			p.Slope = 0.02
			p.Ambient = 0.70
			p.Shadow = 0.15
			p.Rim = 0.05
		}),
		mod("papercut-max", func(p *ReliefParams) {
			// Minimal hillshade, maximal band-edge shadows: layered paper.
			p.Slope = 0.015
			p.Ambient = 0.80
			p.EdgeWidth = 0.70
			p.Shadow = 0.50
			p.Rim = 0.18
			p.Spec = 0
		}),
		mod("smooth-marble", func(p *ReliefParams) {
			// Hillshade only, no engraved edges: polished sculpted stone.
			p.Slope = 0.08
			p.Ambient = 0.50
			p.Shadow = 0
			p.Rim = 0
		}),
		mod("glossy-enamel", func(p *ReliefParams) {
			// Broad wet gloss over the baseline carve.
			p.Spec = 0.35
			p.Shininess = 6
			p.Ambient = 0.65
		}),
		mod("hard-metallic", func(p *ReliefParams) {
			// Low raking light from top-right with a tight bright highlight.
			p.LightDir = [3]float64{0.8, -0.6, 0.4}
			p.Slope = 0.08
			p.Ambient = 0.45
			p.Spec = 0.50
			p.Shininess = 60
		}),
	}
}

// ReliefPreset returns the named relief look.
func ReliefPreset(name string) (ReliefParams, bool) {
	for _, p := range reliefPresets() {
		if p.Name == name {
			return p.Params, true
		}
	}
	return ReliefParams{}, false
}

// ReliefPresetNames lists the presets in presentation order.
func ReliefPresetNames() []string {
	ps := reliefPresets()
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	return names
}
