package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

func loadCutoutShader() rl.Shader {
	vertex := "#version 330\n" +
		"in vec3 vertexPosition;\n" +
		"in vec2 vertexTexCoord;\n" +
		"in vec4 vertexColor;\n" +
		"out vec2 fragTexCoord;\n" +
		"out vec4 fragColor;\n" +
		"uniform mat4 mvp;\n" +
		"void main()\n" +
		"{\n" +
		"    fragTexCoord = vertexTexCoord;\n" +
		"    fragColor = vertexColor;\n" +
		"    gl_Position = mvp*vec4(vertexPosition, 1.0);\n" +
		"}\n"
	fragment := "#version 330\n" +
		"in vec2 fragTexCoord;\n" +
		"in vec4 fragColor;\n" +
		"out vec4 finalColor;\n" +
		"uniform sampler2D texture0;\n" +
		"uniform vec4 colDiffuse;\n" +
		"void main()\n" +
		"{\n" +
		"    vec4 texelColor = texture(texture0, fragTexCoord);\n" +
		"    vec4 color = texelColor*colDiffuse*fragColor;\n" +
		"    if (color.a < 0.5) discard;\n" +
		"    finalColor = color;\n" +
		"}\n"
	return rl.LoadShaderFromMemory(vertex, fragment)
}

func (a *RenderAssets) loadFogShader() rl.Shader {
	vertex := `#version 330
in vec3 vertexPosition;
in vec2 vertexTexCoord;
in vec4 vertexColor;
out vec2 fragTexCoord;
out vec4 fragColor;
out vec2 fragOffset;
uniform mat4 mvp;
uniform mat4 matModel;
uniform vec3 viewPos;
void main()
{
    fragTexCoord = vertexTexCoord;
    fragColor = vertexColor;
    vec4 worldPos = matModel * vec4(vertexPosition, 1.0);
    fragOffset = worldPos.xz - viewPos.xz;
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}`
	fragment := `#version 330
in vec2 fragTexCoord;
in vec4 fragColor;
in vec2 fragOffset;
out vec4 finalColor;
uniform sampler2D texture0;
uniform vec4 colDiffuse;
uniform vec4 fogColor;
uniform float fogStart;
uniform float fogEnd;

void main()
{
    vec4 texelColor = texture(texture0, fragTexCoord);
    vec4 color = texelColor * colDiffuse * fragColor;
    if (color.a < 0.5) discard;

    float fogAmount = smoothstep(fogStart, fogEnd, length(fragOffset));
    finalColor = vec4(mix(color.rgb, fogColor.rgb, fogAmount), color.a);
}`
	return rl.LoadShaderFromMemory(vertex, fragment)
}
