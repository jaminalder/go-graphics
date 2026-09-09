package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/png"
	"strings"

	"github.com/jaminalder/go-graphics/internal/artwork"
	"github.com/jaminalder/go-graphics/internal/publish"
)

type example struct {
	File       string          `json:"file"`
	Recipe     json.RawMessage `json:"recipe"`
	SHA256     string          `json:"sha256"`
	Width      int             `json:"width"`
	Height     int             `json:"height"`
	Alt        string          `json:"alt"`
	Provenance string          `json:"provenance"`
}

func catalogueAssets() (map[string]string, error) {
	raw, e := files.ReadFile("assets/catalogue.json")
	if e != nil {
		return nil, e
	}
	var examples []example
	if e := artwork.StrictJSON(raw, &examples); e != nil {
		return nil, e
	}
	known := map[string]bool{}
	for _, example := range examples {
		if known[example.File] || strings.Contains(example.File, "/") {
			return nil, errors.New("invalid catalogue file")
		}
		known[example.File] = true
		b, e := files.ReadFile("assets/" + example.File)
		if e != nil {
			return nil, e
		}
		h := sha256.Sum256(b)
		if hex.EncodeToString(h[:]) != example.SHA256 {
			return nil, errors.New("catalogue image digest mismatch")
		}
		cfg, e := png.DecodeConfig(bytes.NewReader(b))
		if e != nil || cfg.Width != example.Width || cfg.Height != example.Height {
			return nil, errors.New("catalogue dimensions mismatch")
		}
		if _, e := publish.Validate(example.Recipe); e != nil {
			return nil, e
		}
		if example.Alt == "" || example.Provenance == "" {
			return nil, errors.New("catalogue provenance missing")
		}
	}
	for _, entry := range publish.All() {
		if !known[entry.ID+"-hero.png"] {
			return nil, errors.New("missing hero")
		}
		for _, c := range entry.Styles {
			if !known[entry.ID+"-"+c.ID+".png"] {
				return nil, errors.New("missing style example")
			}
		}
		for _, c := range entry.Colours {
			if !known[entry.ID+"-"+c.ID+".png"] {
				return nil, errors.New("missing colour example")
			}
		}
	}
	entries, e := files.ReadDir("assets")
	if e != nil {
		return nil, e
	}
	hashes := map[string]string{}
	for _, entry := range entries {
		b, e := files.ReadFile("assets/" + entry.Name())
		if e != nil {
			return nil, e
		}
		h := sha256.Sum256(b)
		hashes[entry.Name()] = hex.EncodeToString(h[:])[:12]
	}
	return hashes, nil
}
