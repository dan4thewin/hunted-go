#version 330

// Input fragment attributes (from vertex shader)
in vec2 fragTexCoord;
in vec4 fragColor;

// Input uniform values
uniform sampler2D texture0;
uniform float brightness;  // 0.0 = normal, 1.0 = full white

// Output fragment color
out vec4 finalColor;

void main()
{
    // Fetch texel color from texture sampler
    vec4 texelColor = texture(texture0, fragTexCoord) * fragColor;
    
    // Lerp toward white based on brightness value
    vec3 white = vec3(1.0, 1.0, 1.0);
    vec3 result = mix(texelColor.rgb, white, brightness);
    
    finalColor = vec4(result, texelColor.a);
}
