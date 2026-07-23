package render

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"os"
)

// Meta is embedded into written image files: physical resolution for print
// handoff, and provenance (recipe + software revision) so an artwork stays
// self-describing after any rename.
type Meta struct {
	DPI      int    // physical resolution; 0 = omit
	Software string // e.g. "staticart abc123"; "" = omit
	Comment  string // full render recipe; "" = omit
}

// WritePNGMeta encodes img as PNG with sRGB/gAMA tags and Meta embedded
// as pHYs + tEXt chunks.
func WritePNGMeta(path string, img image.Image, m Meta) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return fmt.Errorf("render: encoding %s: %w", path, err)
	}
	out, err := splicePNGChunks(buf.Bytes(), m)
	if err != nil {
		return fmt.Errorf("render: %s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}

// WriteJPEGMeta encodes img as JPEG at JPEGQuality with a JFIF density
// header (DPI) and the recipe as a COM segment.
func WriteJPEGMeta(path string, img image.Image, m Meta) error {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		return fmt.Errorf("render: encoding %s: %w", path, err)
	}
	out, err := spliceJPEGSegments(buf.Bytes(), m)
	if err != nil {
		return fmt.Errorf("render: %s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}

// pngChunk assembles one chunk: length, type, data, CRC.
func pngChunk(typ string, data []byte) []byte {
	out := make([]byte, 0, 12+len(data))
	out = binary.BigEndian.AppendUint32(out, uint32(len(data))) //nolint:gosec // chunk data is small
	out = append(out, typ...)
	out = append(out, data...)
	crc := crc32.NewIEEE()
	crc.Write([]byte(typ))
	crc.Write(data)
	return binary.BigEndian.AppendUint32(out, crc.Sum32())
}

// splicePNGChunks inserts metadata chunks right after IHDR (signature 8
// bytes + 25-byte IHDR chunk = offset 33).
func splicePNGChunks(p []byte, m Meta) ([]byte, error) {
	const ihdrEnd = 33
	if len(p) < ihdrEnd || string(p[12:16]) != "IHDR" {
		return nil, fmt.Errorf("unexpected PNG layout")
	}
	var chunks []byte
	// sRGB (perceptual intent) with the spec-recommended gAMA fallback.
	chunks = append(chunks, pngChunk("sRGB", []byte{0})...)
	chunks = append(chunks, pngChunk("gAMA", binary.BigEndian.AppendUint32(nil, 45455))...)
	if m.DPI > 0 {
		ppm := uint32(float64(m.DPI)/0.0254 + 0.5) // pixels per meter
		data := binary.BigEndian.AppendUint32(nil, ppm)
		data = binary.BigEndian.AppendUint32(data, ppm)
		data = append(data, 1) // unit: meter
		chunks = append(chunks, pngChunk("pHYs", data)...)
	}
	if m.Software != "" {
		chunks = append(chunks, pngChunk("tEXt", append([]byte("Software\x00"), m.Software...))...)
	}
	if m.Comment != "" {
		chunks = append(chunks, pngChunk("tEXt", append([]byte("Comment\x00"), m.Comment...))...)
	}
	out := make([]byte, 0, len(p)+len(chunks))
	out = append(out, p[:ihdrEnd]...)
	out = append(out, chunks...)
	return append(out, p[ihdrEnd:]...), nil
}

// spliceJPEGSegments inserts a JFIF APP0 (with DPI density) and a COM
// segment right after SOI. Go's encoder writes no APP0 of its own.
func spliceJPEGSegments(p []byte, m Meta) ([]byte, error) {
	if len(p) < 2 || p[0] != 0xFF || p[1] != 0xD8 {
		return nil, fmt.Errorf("unexpected JPEG layout")
	}
	var segs []byte
	if m.DPI > 0 {
		app0 := []byte{0xFF, 0xE0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 2, 1 /* units: dpi */, 0, 0, 0, 0, 0, 0}
		binary.BigEndian.PutUint16(app0[12:], uint16(m.DPI)) //nolint:gosec // DPI is small
		binary.BigEndian.PutUint16(app0[14:], uint16(m.DPI)) //nolint:gosec // DPI is small
		segs = append(segs, app0...)
	}
	if comment := m.Comment; comment != "" {
		if m.Software != "" {
			comment = m.Software + " | " + comment
		}
		if len(comment) > 65000 {
			comment = comment[:65000]
		}
		com := []byte{0xFF, 0xFE, 0, 0}
		binary.BigEndian.PutUint16(com[2:], uint16(len(comment)+2)) //nolint:gosec // capped above
		segs = append(segs, append(com, comment...)...)
	}
	out := make([]byte, 0, len(p)+len(segs))
	out = append(out, p[:2]...)
	out = append(out, segs...)
	return append(out, p[2:]...), nil
}
