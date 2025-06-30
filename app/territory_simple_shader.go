package app

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// territorySimpleShaderSrc is a clean, simple GLSL shader for territory rendering with hover effects
const territorySimpleShaderSrc = `
package main

// Shader uniforms
var FillOpacity float    // Base fill opacity (0.0 - 1.0)
var BlinkFactor float    // Blink animation factor for selected territories (0.0 - 1.0)
var HoverFactor float    // Hover effect factor (0.0 - 1.0)

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
    // Get background and overlay colors
    background := imageSrc0At(texCoord)
    overlay := imageSrc1At(texCoord)
    
    // Skip transparent pixels
    if overlay.a == 0.0 {
        return background
    }
    
    // Detect territory states from overlay encoding
    isSelected := overlay.g > 0.95  // Selected territories have high green component
    isHovered := overlay.a > 0.95   // Hovered territories have maximum alpha
    
    // Start with base territory color and opacity
    territoryColor := overlay.rgb
    territoryAlpha := FillOpacity
    
    // Apply hover effect when HoverFactor is active
    if HoverFactor > 0.0 {
        if isHovered {
            // Hovered territory: make it darker and less visible
            territoryAlpha = territoryAlpha * 0.2  // Much less visible
            territoryColor = territoryColor * 0.4  // Darker
            
            // Desaturate to make it look distinct
            gray := 0.299*territoryColor.r + 0.587*territoryColor.g + 0.114*territoryColor.b
            territoryColor = mix(vec3(gray), territoryColor, 0.5)  // 50% desaturated
        } else {
            // Non-hovered territories: keep normal visibility
            territoryAlpha = territoryAlpha * 1.0  // Normal visibility
            territoryColor = territoryColor * 1.0  // Normal color
        }
    }
    
    // Apply selection blinking effect
    if isSelected {
        // Increase base visibility for selected territories
        territoryAlpha = territoryAlpha * 2.0
        
        // Apply pulsing effect
        pulse := 1.0 + (0.5 * BlinkFactor)  // Pulse between 1.0 and 1.5
        territoryColor = territoryColor * pulse
        territoryAlpha = territoryAlpha * pulse
    }
    
    // Ensure alpha stays in valid range
    territoryAlpha = clamp(territoryAlpha, 0.0, 1.0)
    
    // Blend territory with background
    finalColor := mix(background.rgb, territoryColor, territoryAlpha)
    
    return vec4(finalColor, background.a)
}
`

var territorySimpleShader *ebiten.Shader

func init() {
	var err error
	territorySimpleShader, err = ebiten.NewShader([]byte(territorySimpleShaderSrc))
	if err != nil {
		panic("failed to compile territory simple shader: " + err.Error())
	}
}

// DrawTerritoryWithSimpleShader draws territories using the new simple shader
func DrawTerritoryWithSimpleShader(dst *ebiten.Image, bg, overlay *ebiten.Image, fillOpacity, blinkFactor, hoverFactor float32) {
	// Handle same image case
	if dst == bg {
		op := &ebiten.DrawImageOptions{}
		op.ColorScale.ScaleAlpha(fillOpacity)
		dst.DrawImage(overlay, op)
		return
	}

	// Use shader for proper blending
	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = bg
	op.Images[1] = overlay

	// Set shader uniforms
	op.Uniforms = make(map[string]interface{})
	op.Uniforms["FillOpacity"] = fillOpacity
	op.Uniforms["BlinkFactor"] = blinkFactor
	op.Uniforms["HoverFactor"] = hoverFactor

	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	dst.DrawRectShader(w, h, territorySimpleShader, op)
}
