package render

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

// Shader wraps an OpenGL shader program.
type Shader struct {
	id uint32
}

// LoadShader compiles a vertex + fragment shader program.
// sharedPaths are prepended (in order) to both vertex and fragment sources.
func LoadShader(vertPath, fragPath string, sharedPaths ...string) (*Shader, error) {
	// Load shared snippets
	var shared strings.Builder
	for _, sp := range sharedPaths {
		data, err := os.ReadFile(sp)
		if err != nil {
			return nil, fmt.Errorf("shader shared file %q: %w", sp, err)
		}
		shared.Write(data)
		shared.WriteString("\n")
	}
	sharedSrc := shared.String()

	vertSrc, err := os.ReadFile(vertPath)
	if err != nil {
		return nil, fmt.Errorf("vertex shader %q: %w", vertPath, err)
	}

	fragSrc, err := os.ReadFile(fragPath)
	if err != nil {
		return nil, fmt.Errorf("fragment shader %q: %w", fragPath, err)
	}

	vertID, err := compileShader(prependShared(string(vertSrc), sharedSrc), gl.VERTEX_SHADER)
	if err != nil {
		return nil, fmt.Errorf("vertex shader %q: %w", vertPath, err)
	}

	fragID, err := compileShader(prependShared(string(fragSrc), sharedSrc), gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vertID)
		return nil, fmt.Errorf("fragment shader %q: %w", fragPath, err)
	}

	prog := gl.CreateProgram()
	gl.AttachShader(prog, vertID)
	gl.AttachShader(prog, fragID)
	gl.LinkProgram(prog)

	gl.DeleteShader(vertID)
	gl.DeleteShader(fragID)

	var status int32
	gl.GetProgramiv(prog, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetProgramiv(prog, gl.INFO_LOG_LENGTH, &logLen)
		log := strings.Repeat("\x00", int(logLen+1))
		gl.GetProgramInfoLog(prog, logLen, nil, gl.Str(log))
		gl.DeleteProgram(prog)
		return nil, fmt.Errorf("shader link: %s", log)
	}

	return &Shader{id: prog}, nil
}

// prependShared inserts shared source after the #version directive.
func prependShared(src, shared string) string {
	if shared == "" {
		return src
	}
	lines := strings.SplitN(src, "\n", 2)
	if len(lines) < 2 {
		return shared + "\n" + src
	}
	// If first line is a #version directive, keep it first
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "#version") {
		return lines[0] + "\n" + shared + "\n" + lines[1]
	}
	return shared + "\n" + src
}

func compileShader(src string, shaderType uint32) (uint32, error) {
	id := gl.CreateShader(shaderType)
	csrc, free := gl.Strs(src + "\x00")
	gl.ShaderSource(id, 1, csrc, nil)
	free()
	gl.CompileShader(id)

	var status int32
	gl.GetShaderiv(id, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(id, gl.INFO_LOG_LENGTH, &logLen)
		log := strings.Repeat("\x00", int(logLen+1))
		gl.GetShaderInfoLog(id, logLen, nil, gl.Str(log))
		gl.DeleteShader(id)
		return 0, fmt.Errorf("compile error: %s", log)
	}
	return id, nil
}

// Use binds this shader program.
func (s *Shader) Use() {
	gl.UseProgram(s.id)
}

func (s *Shader) loc(name string) int32 {
	return gl.GetUniformLocation(s.id, gl.Str(name+"\x00"))
}

// SetMat4 sets a mat4 uniform.
func (s *Shader) SetMat4(name string, m mgl32.Mat4) {
	gl.UniformMatrix4fv(s.loc(name), 1, false, &m[0])
}

// SetVec3 sets a vec3 uniform.
func (s *Shader) SetVec3(name string, v mgl32.Vec3) {
	gl.Uniform3f(s.loc(name), v[0], v[1], v[2])
}

// SetVec4 sets a vec4 uniform.
func (s *Shader) SetVec4(name string, v mgl32.Vec4) {
	gl.Uniform4f(s.loc(name), v[0], v[1], v[2], v[3])
}

// SetInt sets an int uniform.
func (s *Shader) SetInt(name string, v int) {
	gl.Uniform1i(s.loc(name), int32(v))
}

// SetFloat sets a float uniform.
func (s *Shader) SetFloat(name string, v float32) {
	gl.Uniform1f(s.loc(name), v)
}

// SetVec2 sets a vec2 uniform.
func (s *Shader) SetVec2(name string, v mgl32.Vec2) {
	gl.Uniform2f(s.loc(name), v[0], v[1])
}
