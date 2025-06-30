package app

import (
	"fmt"
	"image/color"
	"math"
	"runtime/debug"

	"github.com/hajimehoshi/ebiten/v2"
)

// OverlayState represents the visual state of a territory or route overlay.
type OverlayState int

const (
	OverlayNormal OverlayState = iota
	OverlayHovered
	OverlaySelected
	OverlayDimmed // For territories not belonging to the currently editing guild
)

// OverlayPolygon represents a polygon overlay (territory or route).
type OverlayPolygon struct {
	Points      [][2]float32 // Polygon vertices in map coordinates (screen space)
	Color       [3]float32   // RGB color (0..1)
	State       OverlayState // Normal, Hovered, Selected
	BlinkPhase  float32      // 0..1, for blinking animation (only for selected)
	BorderWidth float32      // Border width in pixels (0 = no border)
	BorderColor [3]float32   // RGB border color (0..1)
}

// TerritoryOverlayGPU handles GPU-accelerated overlay rendering.
type TerritoryOverlayGPU struct {
	shader     *ebiten.Shader
	whitePixel *ebiten.Image // 1x1 white pixel for solid color rendering

	// Reusable slices to minimize allocations
	fillVertices   []ebiten.Vertex
	fillIndices    []uint16
	borderVertices []ebiten.Vertex
	borderIndices  []uint16
}

// NewTerritoryOverlayGPU creates a new overlay renderer with the custom shader.
func NewTerritoryOverlayGPU() (*TerritoryOverlayGPU, error) {
	shader, err := ebiten.NewShader([]byte(territoryOverlayShaderSrc))
	if err != nil {
		return nil, err
	}

	// Create 1x1 white pixel image for solid color rendering
	whitePixel := ebiten.NewImage(1, 1)
	whitePixel.Fill(color.RGBA{255, 255, 255, 255})

	return &TerritoryOverlayGPU{
		shader:     shader,
		whitePixel: whitePixel,
	}, nil
}

// Draw overlays all polygons onto the dst image, blending with the map.
// polygons: all overlays to draw (territories, routes, etc)
// dst: the screen or offscreen buffer
// mapImg: the map image to blend with
// time: current time in seconds (for blinking)
func (r *TerritoryOverlayGPU) Draw(dst, mapImg *ebiten.Image, polygons []OverlayPolygon, time float32) {
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Printf("[PANIC] TerritoryOverlayGPU.Draw recovered: %v\n", rec)
			fmt.Printf("[PANIC] Stack trace:\n%s\n", debug.Stack())
		}
	}()
	if len(polygons) == 0 {
		return
	}

	// Batch all fills (reuse slices)
	r.fillVertices = r.fillVertices[:0]
	r.fillIndices = r.fillIndices[:0]
	vertexIndex := uint16(0)
	for _, poly := range polygons {
		if len(poly.Points) < 3 {
			continue
		}
		alpha := overlayAlphaForState(poly.State, poly.BlinkPhase)
		col := color.NRGBA{
			R: uint8(clamp01(poly.Color[0]) * 255),
			G: uint8(clamp01(poly.Color[1]) * 255),
			B: uint8(clamp01(poly.Color[2]) * 255),
			A: uint8(alpha * 255),
		}
		vs := make([]ebiten.Vertex, len(poly.Points))
		for i, pt := range poly.Points {
			vs[i].DstX = pt[0]
			vs[i].DstY = pt[1]
			vs[i].SrcX = 0
			vs[i].SrcY = 0
			vs[i].ColorR = float32(col.R) / 255
			vs[i].ColorG = float32(col.G) / 255
			vs[i].ColorB = float32(col.B) / 255
			vs[i].ColorA = float32(col.A) / 255
		}
		is := triangulatePolygon(len(poly.Points))
		for _, idx := range is {
			r.fillIndices = append(r.fillIndices, idx+vertexIndex)
		}
		r.fillVertices = append(r.fillVertices, vs...)
		vertexIndex += uint16(len(vs))
	}
	if len(r.fillVertices) > 0 && len(r.fillIndices) > 0 {
		opts := &ebiten.DrawTrianglesOptions{}
		opts.CompositeMode = ebiten.CompositeModeSourceOver
		dst.DrawTriangles(r.fillVertices, r.fillIndices, r.whitePixel, opts)
	}

	// Batch all borders (as thick quads, reuse slices)
	r.borderVertices = r.borderVertices[:0]
	r.borderIndices = r.borderIndices[:0]
	vertexIndex = 0
	for _, poly := range polygons {
		if poly.BorderWidth <= 0 || len(poly.Points) < 2 {
			continue
		}
		fillAlpha := overlayAlphaForState(poly.State, poly.BlinkPhase)
		borderAlpha := fillAlpha + 0.25
		if borderAlpha > 1.0 {
			borderAlpha = 1.0
		}
		borderCol := color.NRGBA{
			R: uint8(clamp01(poly.BorderColor[0]) * 255),
			G: uint8(clamp01(poly.BorderColor[1]) * 255),
			B: uint8(clamp01(poly.BorderColor[2]) * 255),
			A: uint8(borderAlpha * 255),
		}
		for i := 0; i < len(poly.Points); i++ {
			p1 := poly.Points[i]
			p2 := poly.Points[(i+1)%len(poly.Points)]
			dx := p2[0] - p1[0]
			dy := p2[1] - p1[1]
			length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if length < 0.001 {
				continue
			}
			nx := -dy / length * poly.BorderWidth * 0.5
			ny := dx / length * poly.BorderWidth * 0.5
			vs := []ebiten.Vertex{
				{DstX: p1[0] + nx, DstY: p1[1] + ny, SrcX: 0, SrcY: 0, ColorR: float32(borderCol.R) / 255, ColorG: float32(borderCol.G) / 255, ColorB: float32(borderCol.B) / 255, ColorA: float32(borderCol.A) / 255},
				{DstX: p1[0] - nx, DstY: p1[1] - ny, SrcX: 0, SrcY: 0, ColorR: float32(borderCol.R) / 255, ColorG: float32(borderCol.G) / 255, ColorB: float32(borderCol.B) / 255, ColorA: float32(borderCol.A) / 255},
				{DstX: p2[0] - nx, DstY: p2[1] - ny, SrcX: 0, SrcY: 0, ColorR: float32(borderCol.R) / 255, ColorG: float32(borderCol.G) / 255, ColorB: float32(borderCol.B) / 255, ColorA: float32(borderCol.A) / 255},
				{DstX: p2[0] + nx, DstY: p2[1] + ny, SrcX: 0, SrcY: 0, ColorR: float32(borderCol.R) / 255, ColorG: float32(borderCol.G) / 255, ColorB: float32(borderCol.B) / 255, ColorA: float32(borderCol.A) / 255},
			}
			is := []uint16{0, 1, 2, 0, 2, 3}
			for _, idx := range is {
				r.borderIndices = append(r.borderIndices, idx+vertexIndex)
			}
			r.borderVertices = append(r.borderVertices, vs...)
			vertexIndex += 4
		}
	}
	if len(r.borderVertices) > 0 && len(r.borderIndices) > 0 {
		opts := &ebiten.DrawTrianglesOptions{}
		opts.CompositeMode = ebiten.CompositeModeSourceOver
		dst.DrawTriangles(r.borderVertices, r.borderIndices, r.whitePixel, opts)
	}
}

func overlayAlphaForState(state OverlayState, blinkPhase float32) float32 {
	switch state {
	case OverlayNormal:
		return 0.33 // 33% opacity to match GIMP Normal blend mode preference
	case OverlayHovered:
		return 0.75 // Moderately increased visibility for hovered state
	case OverlaySelected:
		// Simple blink jump between two alpha values
		if math.Sin(float64(blinkPhase)*math.Pi*2) > 0 {
			return 0.85 // High alpha
		} else {
			return 0.55 // Low alpha
		}
	case OverlayDimmed:
		return 0.10 // Very low opacity for territories not being edited
	default:
		return 0.33
	}
}

func clamp01(f float32) float32 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// Returns indices for a fan triangulation (suitable for convex polygons)
func triangulatePolygon(n int) []uint16 {
	if n < 3 {
		return nil
	}
	is := make([]uint16, 0, (n-2)*3)
	for i := 1; i < n-1; i++ {
		is = append(is, 0, uint16(i), uint16(i+1))
	}
	return is
}

// Ebiten shader source for overlay blending (Ebiten Go shading language)
const territoryOverlayShaderSrc = `
package main

var Time float

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	mapColor := imageSrc0At(texCoord)
	overlay := imageSrc1At(texCoord)
	if overlay.a == 0.0 {
		return mapColor
	}
	// Blinking for selected (alpha in [0.54, 0.8])
	blink := 1.0
	if overlay.a > 0.54 && overlay.a < 0.8 {
		blink = 0.7 + 0.3 * sin(Time * 6.2831)
	}
	outAlpha := overlay.a * blink
	outColor := mix(mapColor.rgb, overlay.rgb, outAlpha)
	return vec4(outColor, 1.0)
}
`
