package render

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type objFace struct {
	vIdx  [3]int
	vnIdx [3]int
	vtIdx [3]int
}

// LoadOBJ loads an OBJ file and returns a Mesh and texture ID.
// Falls back to NewBoxMesh if the file is missing.
func LoadOBJ(path string, fallbackMeshID uint32) (*Mesh, uint32) {
	mesh, texID, err := loadOBJFile(path)
	if err != nil {
		// fallback to box mesh based on mesh type
		mesh, texID = fallbackMesh(fallbackMeshID)
	}
	return mesh, texID
}

func fallbackMesh(meshID uint32) (*Mesh, uint32) {
	texID := whiteTexture()
	switch meshID {
	case MeshTree:
		return NewBoxMesh(2, 20, 2, [3]float32{0.1, 0.5, 0.1}), texID
	case MeshRock:
		return NewBoxMesh(3, 2, 3, [3]float32{0.5, 0.5, 0.5}), texID
	case MeshStump:
		return NewBoxMesh(1.5, 1.5, 1.5, [3]float32{0.4, 0.3, 0.2}), texID
	case MeshBuilding:
		return NewBoxMesh(20, 8, 20, [3]float32{0.8, 0.7, 0.6}), texID
	case MeshTower:
		return NewBoxMesh(1, 20, 1, [3]float32{0.6, 0.6, 0.6}), texID
	case MeshAgent:
		return NewBoxMesh(1, 2, 0.5, [3]float32{0.9, 0.2, 0.2}), texID
	case MeshLiftStation:
		return NewBoxMesh(8, 6, 8, [3]float32{0.5, 0.55, 0.6}), texID
	case MeshShed:
		return NewBoxMesh(16, 7, 12, [3]float32{0.65, 0.70, 0.78}), texID
	case MeshSnowcat:
		return NewBoxMesh(6, 3, 3, [3]float32{0.95, 0.45, 0.20}), texID
	}
	return NewBoxMesh(2, 2, 2, [3]float32{1, 1, 1}), texID
}

func loadOBJFile(path string) (*Mesh, uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var positions [][3]float32
	var colors [][3]float32 // parallel to positions; defaults to white if not in OBJ
	var normals [][3]float32
	var uvs [][2]float32
	var faces []objFace
	mtlFile := ""

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "v":
			if len(fields) >= 4 {
				x, _ := strconv.ParseFloat(fields[1], 32)
				y, _ := strconv.ParseFloat(fields[2], 32)
				z, _ := strconv.ParseFloat(fields[3], 32)
				positions = append(positions, [3]float32{float32(x), float32(y), float32(z)})
				// Wavefront extension: `v x y z r g b` carries per-vertex colour.
				// Falls back to white if the three colour components aren't there.
				col := [3]float32{1, 1, 1}
				if len(fields) >= 7 {
					r, _ := strconv.ParseFloat(fields[4], 32)
					g, _ := strconv.ParseFloat(fields[5], 32)
					b, _ := strconv.ParseFloat(fields[6], 32)
					col = [3]float32{float32(r), float32(g), float32(b)}
				}
				colors = append(colors, col)
			}
		case "vn":
			if len(fields) >= 4 {
				x, _ := strconv.ParseFloat(fields[1], 32)
				y, _ := strconv.ParseFloat(fields[2], 32)
				z, _ := strconv.ParseFloat(fields[3], 32)
				normals = append(normals, [3]float32{float32(x), float32(y), float32(z)})
			}
		case "vt":
			if len(fields) >= 3 {
				u, _ := strconv.ParseFloat(fields[1], 32)
				v, _ := strconv.ParseFloat(fields[2], 32)
				uvs = append(uvs, [2]float32{float32(u), float32(v)})
			}
		case "f":
			// Parse face — may be triangle or quad
			faceVerts := fields[1:]
			parsed := make([][3]int, len(faceVerts)) // [v, vt, vn]
			for i, fv := range faceVerts {
				parsed[i] = parseFaceVertex(fv)
			}
			// Triangulate: fan from first vertex
			for i := 1; i+1 < len(parsed); i++ {
				face := objFace{}
				verts := [3][3]int{parsed[0], parsed[i], parsed[i+1]}
				for j, v := range verts {
					face.vIdx[j] = v[0]
					face.vtIdx[j] = v[1]
					face.vnIdx[j] = v[2]
				}
				faces = append(faces, face)
			}
		case "mtllib":
			if len(fields) >= 2 {
				mtlFile = fields[1]
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	if len(faces) == 0 {
		return nil, 0, fmt.Errorf("no faces in OBJ %q", path)
	}

	// Build flat-shaded vertex buffer. Vertex layout: pos(3) + normal(3) +
	// uv(2) + color(3) = 11 floats. Color comes from the OBJ extension
	// `v x y z r g b`; defaults to white if not provided. The color slot
	// binds at GL location 8 (VertexColorLoc) so it doesn't collide with
	// the static batch's per-instance attributes at 3-7 or the dynamic
	// batch's at 2-4 — see mesh.go.
	const floatsPerVert = 11
	vertices := make([]float32, 0, len(faces)*3*floatsPerVert)
	indices := make([]uint32, 0, len(faces)*3)

	for fi, face := range faces {
		// compute face normal from positions
		var faceNormal [3]float32
		if len(normals) > 0 && face.vnIdx[0] > 0 && face.vnIdx[0]-1 < len(normals) {
			n := normals[face.vnIdx[0]-1]
			faceNormal = n
		} else {
			// compute from positions
			if len(positions) >= 3 {
				vi0 := clampIdx(face.vIdx[0]-1, len(positions))
				vi1 := clampIdx(face.vIdx[1]-1, len(positions))
				vi2 := clampIdx(face.vIdx[2]-1, len(positions))
				p0 := positions[vi0]
				p1 := positions[vi1]
				p2 := positions[vi2]
				faceNormal = computeNormal(p0, p1, p2)
			}
		}

		baseIdx := uint32(fi * 3)
		for j := 0; j < 3; j++ {
			vi := clampIdx(face.vIdx[j]-1, len(positions))
			pos := positions[vi]
			col := [3]float32{1, 1, 1}
			if vi < len(colors) {
				col = colors[vi]
			}

			// UV
			var uv [2]float32
			if face.vtIdx[j] > 0 && face.vtIdx[j]-1 < len(uvs) {
				uv = uvs[face.vtIdx[j]-1]
			}

			vertices = append(vertices,
				pos[0], pos[1], pos[2],
				faceNormal[0], faceNormal[1], faceNormal[2],
				uv[0], uv[1],
				col[0], col[1], col[2],
			)
			indices = append(indices, baseIdx+uint32(j))
		}
	}

	mesh := NewMesh(vertices, indices, []int{3, 3, 2, 3}, []uint32{0, 1, 2, VertexColorLoc})

	// Load texture from MTL if present
	texID := whiteTexture()
	if mtlFile != "" {
		dir := filepath.Dir(path)
		mtlPath := filepath.Join(dir, mtlFile)
		texPath := findTexturePath(mtlPath)
		if texPath != "" {
			id, err := LoadTexture(texPath)
			if err == nil {
				texID = id
			}
		}
	}

	return mesh, texID, nil
}

func parseFaceVertex(s string) [3]int {
	parts := strings.Split(s, "/")
	result := [3]int{0, 0, 0}
	for i, p := range parts {
		if i >= 3 {
			break
		}
		if p == "" {
			continue
		}
		v, _ := strconv.Atoi(p)
		result[i] = v
	}
	return result
}

func clampIdx(idx, length int) int {
	if idx < 0 {
		return 0
	}
	if idx >= length {
		return length - 1
	}
	return idx
}

func computeNormal(p0, p1, p2 [3]float32) [3]float32 {
	e1 := [3]float32{p1[0] - p0[0], p1[1] - p0[1], p1[2] - p0[2]}
	e2 := [3]float32{p2[0] - p0[0], p2[1] - p0[1], p2[2] - p0[2]}
	n := [3]float32{
		e1[1]*e2[2] - e1[2]*e2[1],
		e1[2]*e2[0] - e1[0]*e2[2],
		e1[0]*e2[1] - e1[1]*e2[0],
	}
	l := float32(1.0)
	mag := n[0]*n[0] + n[1]*n[1] + n[2]*n[2]
	if mag > 0 {
		l = 1.0 / sqrtf(mag)
	}
	return [3]float32{n[0] * l, n[1] * l, n[2] * l}
}

func sqrtf(x float32) float32 {
	if x <= 0 {
		return 0
	}
	// Newton-Raphson
	z := x
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

func findTexturePath(mtlPath string) string {
	f, err := os.Open(mtlPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	dir := filepath.Dir(mtlPath)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "map_Kd" {
			return filepath.Join(dir, fields[1])
		}
	}
	return ""
}
