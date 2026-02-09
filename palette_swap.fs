#version 330

// Input fragment attributes (from vertex shader)
in vec2 fragTexCoord;
in vec4 fragColor;

// Input uniform values
uniform sampler2D texture0;
uniform float paletteIndex;  // 0.0, 1.0, or 2.0 for different waif colors

// PICO-8 palette colors (16 colors)
const vec3 pico8[16] = vec3[16](
    vec3(0.0, 0.0, 0.0),           // 0: black
    vec3(0.114, 0.169, 0.325),     // 1: dark blue
    vec3(0.494, 0.145, 0.325),     // 2: dark purple
    vec3(0.0, 0.529, 0.318),       // 3: dark green
    vec3(0.671, 0.322, 0.212),     // 4: brown
    vec3(0.373, 0.341, 0.310),     // 5: dark gray
    vec3(0.761, 0.765, 0.780),     // 6: light gray
    vec3(1.0, 0.945, 0.910),       // 7: white
    vec3(1.0, 0.0, 0.302),         // 8: red
    vec3(1.0, 0.639, 0.0),         // 9: orange
    vec3(1.0, 0.925, 0.153),       // 10: yellow
    vec3(0.0, 0.894, 0.212),       // 11: green
    vec3(0.161, 0.678, 1.0),       // 12: blue
    vec3(0.514, 0.463, 0.612),     // 13: indigo
    vec3(1.0, 0.467, 0.659),       // 14: pink
    vec3(1.0, 0.800, 0.667)        // 15: peach
);

// Palette remaps from hunted.lua recolor table
// recolor[0] = { 1,14,3,4,5,6,7,8,4,10,11,12,13,14,15,0 }
// recolor[1] = { 1,12,3,4,5,6,7,8,13,10,11,12,13,14,4,0 }
// recolor[2] = { 1,3,3,4,5,6,7,8,5,10,11,12,13,14,9,0 }
const int remap0[16] = int[16](0, 1, 14, 3, 4, 5, 6, 7, 8, 4, 10, 11, 12, 13, 14, 15);
const int remap1[16] = int[16](0, 1, 12, 3, 4, 5, 6, 7, 8, 13, 10, 11, 12, 13, 14, 4);
const int remap2[16] = int[16](0, 1, 3, 3, 4, 5, 6, 7, 8, 5, 10, 11, 12, 13, 14, 9);

// Output fragment color
out vec4 finalColor;

// Find closest PICO-8 color index
int findClosestColor(vec3 color) {
    float minDist = 999.0;
    int closest = 0;
    for (int i = 0; i < 16; i++) {
        vec3 diff = color - pico8[i];
        float dist = dot(diff, diff);
        if (dist < minDist) {
            minDist = dist;
            closest = i;
        }
    }
    return closest;
}

void main()
{
    // Fetch texel color from texture sampler
    vec4 texelColor = texture(texture0, fragTexCoord) * fragColor;
    
    // If mostly transparent, keep as is
    if (texelColor.a < 0.1) {
        finalColor = texelColor;
        return;
    }
    
    // Find which PICO-8 color this is
    int colorIndex = findClosestColor(texelColor.rgb);
    
    // Get remapped color index based on palette
    int newIndex = colorIndex;
    int palIdx = int(paletteIndex);
    if (palIdx == 0) {
        newIndex = remap0[colorIndex];
    } else if (palIdx == 1) {
        newIndex = remap1[colorIndex];
    } else {
        newIndex = remap2[colorIndex];
    }
    
    // Output the remapped color
    finalColor = vec4(pico8[newIndex], texelColor.a);
}
