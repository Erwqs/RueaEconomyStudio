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

	// Pre-allocated temporary slices to avoid allocations in hot path
	tempVertices []ebiten.Vertex
	tempIndices  []uint16
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
		shader:         shader,
		whitePixel:     whitePixel,
		fillVertices:   make([]ebiten.Vertex, 0, 1000), // Pre-allocate with reasonable capacity
		fillIndices:    make([]uint16, 0, 3000),        // Pre-allocate with reasonable capacity
		borderVertices: make([]ebiten.Vertex, 0, 500),  // Pre-allocate with reasonable capacity
		borderIndices:  make([]uint16, 0, 1500),        // Pre-allocate with reasonable capacity
		tempVertices:   make([]ebiten.Vertex, 0, 100),  // Pre-allocate temp slice
		tempIndices:    make([]uint16, 0, 300),         // Pre-allocate temp slice
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

		// Pre-compute color components to avoid repeated calculations
		colorR := clamp01(poly.Color[0])
		colorG := clamp01(poly.Color[1])
		colorB := clamp01(poly.Color[2])
		colorA := alpha

		// Ensure temp slice has enough capacity and reset length
		pointCount := len(poly.Points)
		if cap(r.tempVertices) < pointCount {
			r.tempVertices = make([]ebiten.Vertex, 0, pointCount*2)
		}
		r.tempVertices = r.tempVertices[:pointCount]

		// Build vertices in pre-allocated slice
		for i, pt := range poly.Points {
			r.tempVertices[i].DstX = pt[0]
			r.tempVertices[i].DstY = pt[1]
			r.tempVertices[i].SrcX = 0
			r.tempVertices[i].SrcY = 0
			r.tempVertices[i].ColorR = colorR
			r.tempVertices[i].ColorG = colorG
			r.tempVertices[i].ColorB = colorB
			r.tempVertices[i].ColorA = colorA
		}

		// Get triangulation indices
		triangleIndices := triangulatePolygon(pointCount)

		// Add indices with offset
		for _, idx := range triangleIndices {
			r.fillIndices = append(r.fillIndices, idx+vertexIndex)
		}

		// Add vertices
		r.fillVertices = append(r.fillVertices, r.tempVertices...)
		vertexIndex += uint16(pointCount)
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

		// Pre-compute border color components
		borderColorR := clamp01(poly.BorderColor[0])
		borderColorG := clamp01(poly.BorderColor[1])
		borderColorB := clamp01(poly.BorderColor[2])
		borderColorA := borderAlpha
		halfBorderWidth := poly.BorderWidth * 0.5

		for i := 0; i < len(poly.Points); i++ {
			p1 := poly.Points[i]
			p2 := poly.Points[(i+1)%len(poly.Points)]
			dx := p2[0] - p1[0]
			dy := p2[1] - p1[1]

			// Use fast inverse square root approximation for better performance
			lengthSq := dx*dx + dy*dy
			if lengthSq < 0.000001 { // length < 0.001
				continue
			}
			invLength := float32(1.0 / math.Sqrt(float64(lengthSq)))

			nx := -dy * invLength * halfBorderWidth
			ny := dx * invLength * halfBorderWidth

			// Ensure temp vertices slice has capacity for 4 vertices
			if cap(r.tempVertices) < 4 {
				r.tempVertices = make([]ebiten.Vertex, 4)
			}
			r.tempVertices = r.tempVertices[:4]

			// Build border quad vertices
			r.tempVertices[0] = ebiten.Vertex{DstX: p1[0] + nx, DstY: p1[1] + ny, SrcX: 0, SrcY: 0, ColorR: borderColorR, ColorG: borderColorG, ColorB: borderColorB, ColorA: borderColorA}
			r.tempVertices[1] = ebiten.Vertex{DstX: p1[0] - nx, DstY: p1[1] - ny, SrcX: 0, SrcY: 0, ColorR: borderColorR, ColorG: borderColorG, ColorB: borderColorB, ColorA: borderColorA}
			r.tempVertices[2] = ebiten.Vertex{DstX: p2[0] - nx, DstY: p2[1] - ny, SrcX: 0, SrcY: 0, ColorR: borderColorR, ColorG: borderColorG, ColorB: borderColorB, ColorA: borderColorA}
			r.tempVertices[3] = ebiten.Vertex{DstX: p2[0] + nx, DstY: p2[1] + ny, SrcX: 0, SrcY: 0, ColorR: borderColorR, ColorG: borderColorG, ColorB: borderColorB, ColorA: borderColorA}

			// Add quad indices (two triangles)
			r.borderIndices = append(r.borderIndices,
				vertexIndex, vertexIndex+1, vertexIndex+2,
				vertexIndex, vertexIndex+2, vertexIndex+3)

			r.borderVertices = append(r.borderVertices, r.tempVertices...)
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
