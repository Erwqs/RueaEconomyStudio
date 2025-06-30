#version 330 core

in vec2 fragTexCoord;
out vec4 outColor;

uniform sampler2D mapTexture;
uniform vec3 guildColor;
uniform float overlayAlpha; // 0 = most transparent, 1 = least transparent

void main() {
    vec4 mapColor = texture(mapTexture, fragTexCoord);
    // blend: overlayAlpha
    vec3 blended = mix(mapColor.rgb, guildColor, overlayAlpha);
    // Output alpha is always 1.0
    outColor = vec4(blended, 1.0);
}
