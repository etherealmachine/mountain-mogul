package render

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mountain-mogul/internal/world"
)

type objFace struct {
	vIdx  [3]int
	vnIdx [3]int
	vtIdx [3]int
}

// LoadOBJ loads an OBJ file and returns a Mesh and texture ID. Falls
// back to a magenta marker cube if the file is missing — see fallbackMesh.
func LoadOBJ(path string) (*Mesh, uint32) {
	mesh, texID, err := loadOBJFile(path)
	if err != nil {
		mesh, texID = fallbackMesh()
	}
	return mesh, texID
}

// LoadOBJSlots reads `# slot N x y z` comment lines from an OBJ produced
// by tools/scad2obj. Coordinates are mesh-local game-space (the converter
// has already applied the SCAD Z-up → game Y-up rotation). Returns nil
// if the file is missing or has no slot lines — callers handle the
// no-slots case via a plain fallback.
func LoadOBJSlots(path string) []world.MeshSlot {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var slots []world.MeshSlot
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "# slot ") {
			// Only the leading `# slot ...` comment block is interesting;
			// once we hit a vertex line, no slot lines can follow.
			if strings.HasPrefix(line, "v ") || strings.HasPrefix(line, "f ") {
				break
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 6 {
			continue
		}
		idx, err1 := strconv.Atoi(fields[2])
		x, err2 := strconv.ParseFloat(fields[3], 32)
		y, err3 := strconv.ParseFloat(fields[4], 32)
		z, err4 := strconv.ParseFloat(fields[5], 32)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}
		slots = append(slots, world.MeshSlot{
			Index: idx,
			Pos:   [3]float32{float32(x), float32(y), float32(z)},
		})
	}
	return slots
}

// LoadOBJFootprint reads a `# footprint halfX halfZ` comment line from an
// OBJ produced by tools/scad2obj. Both values are in metres in mesh-local
// game coords (the converter has already applied the SCAD Z-up → game
// Y-up rotation). Returns ok=false when the file is missing or has no
// footprint line — callers fall back to a box-bbox default.
func LoadOBJFootprint(path string) (world.MeshFootprint, bool) {
	f, err := os.Open(path)
	if err != nil {
		return world.MeshFootprint{}, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# footprint ") {
			fields := strings.Fields(line)
			if len(fields) != 4 {
				continue
			}
			hx, err1 := strconv.ParseFloat(fields[2], 32)
			hz, err2 := strconv.ParseFloat(fields[3], 32)
			if err1 != nil || err2 != nil {
				continue
			}
			return world.MeshFootprint{HalfX: float32(hx), HalfZ: float32(hz)}, true
		}
		// Footprint sits in the leading comment block; once vertices /
		// faces appear, no footprint line can follow.
		if strings.HasPrefix(line, "v ") || strings.HasPrefix(line, "f ") {
			break
		}
	}
	return world.MeshFootprint{}, false
}

// fallbackMesh returns a 2 m marker cube used when an OBJ fails to load
// — typically because `make models` hasn't run for a new SCAD file.
// Per-mesh sizes were a development convenience but the working build has
// all OBJs in place, so this only appears as a diagnostic. If you see
// this cube, rebuild the models.
func fallbackMesh() (*Mesh, uint32) {
	return NewBoxMesh(2, 2, 2), whiteTexture()
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
