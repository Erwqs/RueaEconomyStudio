package app

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// territoryEnhancedShaderSrc is the GLSL code for blending territory overlay with separate border and fill handling
const territoryEnhancedShaderSrc = `
package main

// Uniforms for blend mode and color adjustments
var BorderThreshold float // Threshold to determine what is a border (higher values = thicker borders)
var BorderBrightness float // Brightness multiplier for borders
var FillOpacity float // Opacity for the fill (0.0 - 1.0)
var BorderOpacity float // Opacity for the border (0.0 - 1.0)
var BlinkFactor float // Blink animation factor (0.0-1.0) for selected territories
var HoverFactor float // Hover effect factor (0.0-1.0) for highlighting territories under cursor

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
    bg := imageSrc0At(texCoord)
    overlay := imageSrc1At(texCoord)
    
    // Early return for transparent pixels
    if overlay.a == 0.0 {
        return bg
    }
    
    // Detect if this is a selected territory by checking for strong green encoding
    // Selected territories now have green = 1.0 to preserve original colors better
    isSelected := overlay.g > 0.95
    
    // Detect if this is a hovered territory by checking for maximum alpha
    // Hovered territories now have alpha = 1.0 (255)
    isHovered := overlay.a > 0.95
    
    // Base color and alpha settings
    finalColor := overlay.rgb
    finalAlpha := overlay.a * FillOpacity // Use FillOpacity uniform instead of hardcoded value
    
    // HOVER EFFECT - Simple two-state system
    if HoverFactor > 0.0 {
        if isHovered {
            // This is the HOVERED territory - make it less visible (darker/dimmer)
            finalAlpha = finalAlpha * 0.3 // Make hovered territory much less visible
            finalColor = finalColor * 0.5 // Darken hovered territory significantly
            
            // Apply desaturation to make it look distinct
            luminance := 0.299 * finalColor.r + 0.587 * finalColor.g + 0.114 * finalColor.b
            finalColor = mix(vec3(luminance), finalColor, 0.6) // More grayish
        }
        // No else clause - non-hovered territories keep their normal appearance
    }
    
    // Apply selection blinking effect - make selected territories MORE visible
    if isSelected {
        // Base increase in opacity for selected territories
        finalAlpha = finalAlpha * 2.0
        
        // Apply blinking effect based on BlinkFactor (0.0 to 1.0)
        // Make it pulse between 1x and 2x brightness
        blinkBrightness := 1.0 + BlinkFactor
        
        // Apply brightness while preserving hue better
        luminance2 := 0.299 * finalColor.r + 0.587 * finalColor.g + 0.114 * finalColor.b
        enhancedColor := mix(vec3(luminance2), finalColor, 1.2) // Enhance saturation slightly
        finalColor = enhancedColor * blinkBrightness
        
        // Also pulse the opacity
        blinkOpacity := 1.0 + (0.5 * BlinkFactor)
        finalAlpha = finalAlpha * blinkOpacity
    }
    
    // Improve contrast against blue ocean backgrounds
    adaptiveAlpha := finalAlpha
    
    // Check if background is predominantly blue (like ocean)
    if bg.b > 0.6 && bg.b > (bg.r*1.5) && bg.b > (bg.g*1.5) {
        // Increase opacity specifically when overlaying on blue backgrounds
        adaptiveAlpha = finalAlpha * 1.3 // 30% more opacity on blue backgrounds
        
        // Make colors more saturated on blue backgrounds for better contrast
        finalColor = finalColor * 1.1 // Slightly increase color brightness on blue backgrounds
    }
    
    // Ensure alpha stays within valid range
    adaptiveAlpha = min(adaptiveAlpha, 0.95) // Cap at 95% opacity to avoid overwhelming the base map
    
    // Final blend with background using adaptive alpha
    return vec4(mix(bg.rgb, finalColor, adaptiveAlpha), bg.a)
}
`

var territoryEnhancedShader *ebiten.Shader

func init() {
	var err error
	territoryEnhancedShader, err = ebiten.NewShader([]byte(territoryEnhancedShaderSrc))
	if err != nil {
		panic("failed to compile territory enhanced shader: " + err.Error())
	}
}

// DrawTerritoryWithEnhancedShader draws territories with separate control over fill and border
func DrawTerritoryWithEnhancedShader(dst *ebiten.Image, bg, overlay *ebiten.Image, borderThreshold, borderBrightness, fillOpacity, borderOpacity, blinkFactor, hoverFactor float32) {
	// Handle same image case differently
	if dst == bg {
		op := &ebiten.DrawImageOptions{}
		// Apply hover effect to simple draw as well
		opacity := borderOpacity
		if hoverFactor > 0.0 {
			opacity *= (1.0 + (2.0 * hoverFactor)) // Match shader hover effect
		}
		op.ColorScale.ScaleAlpha(opacity)
		dst.DrawImage(overlay, op)
		return
	}

	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = bg
	op.Images[1] = overlay

	// Set shader uniforms
	op.Uniforms = make(map[string]interface{})
	op.Uniforms["BorderThreshold"] = borderThreshold
	op.Uniforms["BorderBrightness"] = borderBrightness
	op.Uniforms["FillOpacity"] = fillOpacity
	op.Uniforms["BorderOpacity"] = borderOpacity
	op.Uniforms["BlinkFactor"] = blinkFactor
	op.Uniforms["HoverFactor"] = hoverFactor

	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	dst.DrawRectShader(w, h, territoryEnhancedShader, op)
}
