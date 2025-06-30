package app

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// territoryBlendShaderSrc is the GLSL code for blending overlay and background
const territoryBlendShaderSrc = `
package main

// Uniforms for blend mode and color adjustments
var BlendMode float // 0=normal, 1=multiply, 2=screen, 3=overlay
var ColorTint vec3  // RGB color tint
var Opacity float   // Global opacity multiplier

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
    bg := imageSrc0At(texCoord)
    overlay := imageSrc1At(texCoord)
    
    // Early return for transparent pixels
    if overlay.a == 0.0 {
        return bg
    }
    
    // Apply color tint to overlay
    tintedOverlay := overlay.rgb * ColorTint
    
    // Apply global opacity
    finalAlpha := overlay.a * Opacity
    
    var result vec3
    
    // Different blend modes
    if BlendMode < 0.5 { // Normal blend
        result = mix(bg.rgb, tintedOverlay, finalAlpha)
    } else if BlendMode < 1.5 { // Multiply blend
        result = mix(bg.rgb, bg.rgb * tintedOverlay, finalAlpha)
    } else if BlendMode < 2.5 { // Screen blend
        result = mix(bg.rgb, 1.0 - (1.0 - bg.rgb) * (1.0 - tintedOverlay), finalAlpha)
    } else { // Overlay blend
        overlay_result := vec3(0)
        if bg.r < 0.5 {
            overlay_result.r = 2.0 * bg.r * tintedOverlay.r
        } else {
            overlay_result.r = 1.0 - 2.0 * (1.0 - bg.r) * (1.0 - tintedOverlay.r)
        }
        if bg.g < 0.5 {
            overlay_result.g = 2.0 * bg.g * tintedOverlay.g
        } else {
            overlay_result.g = 1.0 - 2.0 * (1.0 - bg.g) * (1.0 - tintedOverlay.g)
        }
        if bg.b < 0.5 {
            overlay_result.b = 2.0 * bg.b * tintedOverlay.b
        } else {
            overlay_result.b = 1.0 - 2.0 * (1.0 - bg.b) * (1.0 - tintedOverlay.b)
        }
        result = mix(bg.rgb, overlay_result, finalAlpha)
    }
    
    return vec4(result, bg.a)
}
`

var territoryBlendShader *ebiten.Shader

func init() {
	var err error
	territoryBlendShader, err = ebiten.NewShader([]byte(territoryBlendShaderSrc))
	if err != nil {
		panic("failed to compile territory blend shader: " + err.Error())
	}
}

// BlendMode constants
const (
	BlendModeNormal   = 0.0
	BlendModeMultiply = 1.0
	BlendModeScreen   = 2.0
	BlendModeOverlay  = 3.0
)

// DrawTerritoryOverlayWithShaderAdvanced draws with custom blend mode and color adjustments
func DrawTerritoryOverlayWithShaderAdvanced(dst *ebiten.Image, bg, overlay *ebiten.Image, blendMode float32, colorTint [3]float32, opacity float32) {
	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = bg
	op.Images[1] = overlay

	// Set shader uniforms
	op.Uniforms = make(map[string]interface{})
	op.Uniforms["BlendMode"] = blendMode
	op.Uniforms["ColorTint"] = colorTint
	op.Uniforms["Opacity"] = opacity

	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	dst.DrawRectShader(w, h, territoryBlendShader, op)
}

// Convenience function for simple blending (backward compatibility)
// Now improved with better default settings for visibility
func DrawTerritoryOverlayWithShader(dst *ebiten.Image, bg, overlay *ebiten.Image) {
	// Use overlay blend mode for better contrast against varying backgrounds
	// Apply slightly increased opacity (1.2) and color enhancement with tint
	DrawTerritoryOverlayWithShaderAdvanced(dst, bg, overlay, BlendModeOverlay, [3]float32{1.1, 1.1, 1.1}, 0.8)
}

const territoryColorShaderSrc = `
package main

var SelectedTerritoryColor vec4 // Color for selected territory
var BlinkPhase float           // 0.0 to 1.0 for blinking animation
var TimeMs float              // Current time in milliseconds

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
    bg := imageSrc0At(texCoord)
    overlay := imageSrc1At(texCoord)
    
    if overlay.a == 0.0 {
        return bg
    }
    
    finalColor := overlay.rgb
    finalAlpha := overlay.a
    
    // Check if this pixel is part of selected territory (you'd need to encode this info)
    // For example, using specific color values or additional texture channels
    
    // Apply blinking effect for selected territories
    if BlinkPhase > 0.0 {
        blinkIntensity := 0.5 + 0.5 * sin(TimeMs * 0.01) // Oscillate between 0.5 and 1.0
        finalAlpha *= blinkIntensity
        
        // Optional: Change color for selected territory
        finalColor = mix(finalColor, SelectedTerritoryColor.rgb, BlinkPhase * 0.3)
    }
    
    return vec4(mix(bg.rgb, finalColor, finalAlpha), bg.a)
}
`

// Direct shader rendering functions (more efficient, no temporary buffers)

// DrawTerritoryOverlayDirect draws overlay directly to destination using shader blending
// This function MUST NOT create temporary buffers to avoid memory issues
func DrawTerritoryOverlayDirect(dst *ebiten.Image, bg, overlay *ebiten.Image, blendMode float32, colorTint [3]float32, opacity float32) {
	// CRITICAL: Check if dst and bg are the same image
	if dst == bg {
		// This would cause "source images must be different from the receiver" error
		// We need to handle this case differently - use enhanced drawing for better visibility
		op := &ebiten.DrawImageOptions{}

		// Use higher opacity for better visibility
		op.ColorScale.ScaleAlpha(opacity * 1.2)

		// Apply color enhancement - slightly increased intensity for better visibility
		// Especially for red territories over blue background
		op.ColorScale.Scale(1.1, 1.1, 1.1, 1.0)

		dst.DrawImage(overlay, op)
		return
	}

	// Safe to use shader since dst != bg
	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = bg
	op.Images[1] = overlay

	// Set shader uniforms
	op.Uniforms = make(map[string]interface{})
	op.Uniforms["BlendMode"] = blendMode
	op.Uniforms["ColorTint"] = colorTint
	op.Uniforms["Opacity"] = opacity

	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	dst.DrawRectShader(w, h, territoryBlendShader, op)
}

// DrawCachedTerritoriesDirect renders cached territory and route buffers directly to destination
// MEMORY EFFICIENT VERSION - No temporary buffers created
func DrawCachedTerritoriesDirect(dst, background, territoryCache, routeCache *ebiten.Image, blendMode float32, colorTint [3]float32, opacity float32) {
	// First, blend territories with background if available
	if territoryCache != nil {
		// Direct blend - no temporary buffers
		DrawTerritoryOverlayDirect(dst, background, territoryCache, blendMode, colorTint, opacity*0.7)
	} else if background != nil {
		// If no territory cache, just copy background
		dst.Clear()
		dst.DrawImage(background, nil)
	}

	// Then, blend routes on top if available
	if routeCache != nil {
		// Direct route blending using simple alpha compositing to avoid memory issues
		op := &ebiten.DrawImageOptions{}
		op.ColorScale.ScaleAlpha(opacity)
		dst.DrawImage(routeCache, op)
	}
}
