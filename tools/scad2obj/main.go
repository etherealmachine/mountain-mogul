// scad2obj converts a 3MF file (the format OpenSCAD emits with color()
// information preserved) into an OBJ file with per-vertex colours, ready
// for our renderer's loader.
//
// The 3MF container is a zip archive; the mesh and materials live in
// 3D/3dmodel.model as XML. Each triangle carries a `pid` (basematerials
// resource ID) and `p1` (material index), which we resolve into an RGB
// colour from the `displaycolor` hex on the corresponding <base> element.
//
// Coordinate-system bridge: OpenSCAD authors in Z-up (so models read
// naturally in the OpenSCAD preview); our renderer expects Y-up. The
// converter applies a -90° rotation around the X axis at export time:
//
//	(x, y, z)_scad  →  (x, z, -y)_game
//
// This preserves right-handedness, so face winding and lighting stay
// correct. We emit explicit per-face vertex normals (transformed the same
// way) so the OBJ loader doesn't have to recompute them from positions.
//
// Output OBJ format uses the Wavefront extension `v x y z r g b` to
// carry per-vertex colour. The renderer's OBJ loader reads this; other
// tools that don't recognise the extension ignore the trailing rgb.
//
// Model metadata: the .scad source can publish anchor points and other
// attributes via openscad's echo() function, e.g.
//
//	echo("MOGUL_META", "slot", 0, x, y, z);
//	echo("MOGUL_META", "footprint", halfX, halfY);
//
// When invoked with three arguments (input.3mf input.echo output.obj),
// scad2obj also reads the .echo file (whatever openscad wrote to stderr),
// extracts MOGUL_META lines, applies the same SCAD→game rotation to the
// coordinates where relevant, and emits them at the head of the OBJ as
// `# <kind> ...` comment lines. Downstream consumers (the game's OBJ
// loader) parse those comments and register the metadata against the
// mesh ID. See models-src/README.md for the full list of supported kinds.
package main

import (
	"archive/zip"
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type vec3 [3]float32

// scadToGame rotates a SCAD-space vector into game space.
func scadToGame(v vec3) vec3 {
	return vec3{v[0], v[2], -v[1]}
}

// 3MF schema — only the bits we care about. Namespaces are stripped by
// using `xml:",any"` selectors via local-name matching.

type model struct {
	XMLName   xml.Name  `xml:"model"`
	Resources resources `xml:"resources"`
}

type resources struct {
	BaseMaterials []basematerials `xml:"basematerials"`
	Objects       []object        `xml:"object"`
}

type basematerials struct {
	ID    int    `xml:"id,attr"`
	Bases []base `xml:"base"`
}

type base struct {
	Name         string `xml:"name,attr"`
	DisplayColor string `xml:"displaycolor,attr"`
}

type object struct {
	ID   int  `xml:"id,attr"`
	Mesh mesh `xml:"mesh"`
}

type mesh struct {
	Vertices  vertices  `xml:"vertices"`
	Triangles triangles `xml:"triangles"`
}

type vertices struct {
	V []vertex `xml:"vertex"`
}

type vertex struct {
	X float32 `xml:"x,attr"`
	Y float32 `xml:"y,attr"`
	Z float32 `xml:"z,attr"`
}

type triangles struct {
	T []triangle `xml:"triangle"`
}

type triangle struct {
	V1  int `xml:"v1,attr"`
	V2  int `xml:"v2,attr"`
	V3  int `xml:"v3,attr"`
	PID int `xml:"pid,attr"`
	P1  int `xml:"p1,attr"`
}

// parseHexColor decodes a 3MF `displaycolor` string like "#RRGGBBAA" into
// linear RGB floats in [0, 1]. The alpha component is ignored — the
// renderer treats per-vertex colour as opaque. Defaults to white on a
// parse failure rather than crashing the build over a malformed string.
func parseHexColor(s string) vec3 {
	s = strings.TrimPrefix(s, "#")
	if len(s) < 6 {
		return vec3{1, 1, 1}
	}
	parse := func(off int) float32 {
		v, err := strconv.ParseUint(s[off:off+2], 16, 8)
		if err != nil {
			return 1
		}
		return float32(v) / 255.0
	}
	return vec3{parse(0), parse(2), parse(4)}
}

// readModelXML opens the 3MF zip and returns the contents of
// 3D/3dmodel.model.
func readModelXML(path string) ([]byte, error) {
	rc, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open 3mf: %w", err)
	}
	defer rc.Close()
	for _, f := range rc.File {
		if f.Name == "3D/3dmodel.model" {
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			return io.ReadAll(r)
		}
	}
	return nil, fmt.Errorf("3D/3dmodel.model not found in %s", path)
}

func main() {
	var inPath, echoPath, outPath string
	switch len(os.Args) {
	case 3:
		inPath, outPath = os.Args[1], os.Args[2]
	case 4:
		inPath, echoPath, outPath = os.Args[1], os.Args[2], os.Args[3]
	default:
		fmt.Fprintln(os.Stderr, "usage: scad2obj <input.3mf> [input.echo] <output.obj>")
		os.Exit(1)
	}

	xmlData, err := readModelXML(inPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read 3mf:", err)
		os.Exit(1)
	}
	var m model
	if err := xml.Unmarshal(xmlData, &m); err != nil {
		fmt.Fprintln(os.Stderr, "parse model xml:", err)
		os.Exit(1)
	}

	// Resolve material colours. The "Default" entry from OpenSCAD is the
	// preview yellow used when no color() block applies — remap to white so
	// uncoloured SCAD ports cleanly from the old STL pipeline.
	matColors := map[int][]vec3{}
	for _, bm := range m.Resources.BaseMaterials {
		bases := make([]vec3, len(bm.Bases))
		for i, b := range bm.Bases {
			if b.Name == "Default" {
				bases[i] = vec3{1, 1, 1}
			} else {
				bases[i] = parseHexColor(b.DisplayColor)
			}
		}
		matColors[bm.ID] = bases
	}

	if len(m.Resources.Objects) == 0 {
		fmt.Fprintln(os.Stderr, "no objects in 3mf")
		os.Exit(1)
	}

	out, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create output:", err)
		os.Exit(1)
	}
	defer out.Close()
	w := bufio.NewWriter(out)
	defer w.Flush()

	fmt.Fprintf(w, "# generated by tools/scad2obj from %s\n", inPath)

	// Slots / footprint from openscad echo() — converted to game coords
	// here so the loader doesn't need to know about the SCAD axis
	// convention.
	if echoPath != "" {
		slots, err := readSlots(echoPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read echo:", err)
			os.Exit(1)
		}
		for _, sl := range slots {
			p := scadToGame(sl.pos)
			fmt.Fprintf(w, "# slot %d %g %g %g\n", sl.index, p[0], p[1], p[2])
		}
		if fp, ok, err := readFootprint(echoPath); err != nil {
			fmt.Fprintln(os.Stderr, "read footprint:", err)
			os.Exit(1)
		} else if ok {
			// SCAD declares (halfX, halfY) in its Z-up frame. The
			// Y-up game coords map SCAD Y → game Z, so halfY in
			// SCAD is halfZ in game coords. Always positive.
			fmt.Fprintf(w, "# footprint %g %g\n", fp.halfX, fp.halfY)
		}
		parts, err := readParts(echoPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read parts:", err)
			os.Exit(1)
		}
		for _, pd := range parts {
			p := scadToGame(pd.pos)
			fmt.Fprintf(w, "# part %s %s %g %g %g\n", pd.name, pd.axis, p[0], p[1], p[2])
		}
	}

	// One pass per <object>; vertex/normal indices are kept global across
	// objects (OBJ uses a single index space). For each triangle we
	// duplicate the three vertices so per-face flat normals work and
	// the per-vertex colour matches the triangle's material.
	var vBase int
	totalTris := 0
	for _, obj := range m.Resources.Objects {
		verts := obj.Mesh.Vertices.V
		for _, tri := range obj.Mesh.Triangles.T {
			if tri.V1 >= len(verts) || tri.V2 >= len(verts) || tri.V3 >= len(verts) {
				continue
			}
			p0 := scadToGame(vec3{verts[tri.V1].X, verts[tri.V1].Y, verts[tri.V1].Z})
			p1 := scadToGame(vec3{verts[tri.V2].X, verts[tri.V2].Y, verts[tri.V2].Z})
			p2 := scadToGame(vec3{verts[tri.V3].X, verts[tri.V3].Y, verts[tri.V3].Z})

			col := vec3{1, 1, 1}
			if cols, ok := matColors[tri.PID]; ok && tri.P1 < len(cols) {
				col = cols[tri.P1]
			}

			n := triNormal(p0, p1, p2)

			fmt.Fprintf(w, "v %g %g %g %g %g %g\n", p0[0], p0[1], p0[2], col[0], col[1], col[2])
			fmt.Fprintf(w, "v %g %g %g %g %g %g\n", p1[0], p1[1], p1[2], col[0], col[1], col[2])
			fmt.Fprintf(w, "v %g %g %g %g %g %g\n", p2[0], p2[1], p2[2], col[0], col[1], col[2])
			fmt.Fprintf(w, "vn %g %g %g\n", n[0], n[1], n[2])
			fmt.Fprintf(w, "f %d//%d %d//%d %d//%d\n",
				vBase+1, totalTris+1,
				vBase+2, totalTris+1,
				vBase+3, totalTris+1,
			)
			vBase += 3
			totalTris++
		}
	}

	if totalTris == 0 {
		fmt.Fprintln(os.Stderr, "no triangles produced from", inPath)
		os.Exit(1)
	}

	fmt.Printf("scad2obj: %d tris  %s → %s\n", totalTris, inPath, outPath)
}

// triNormal returns the unit normal of the triangle (p0, p1, p2) with
// standard right-handed winding. Used so the OBJ loader gets explicit
// per-face normals and doesn't have to recompute them.
func triNormal(p0, p1, p2 vec3) vec3 {
	e1 := vec3{p1[0] - p0[0], p1[1] - p0[1], p1[2] - p0[2]}
	e2 := vec3{p2[0] - p0[0], p2[1] - p0[1], p2[2] - p0[2]}
	n := vec3{
		e1[1]*e2[2] - e1[2]*e2[1],
		e1[2]*e2[0] - e1[0]*e2[2],
		e1[0]*e2[1] - e1[1]*e2[0],
	}
	l := n[0]*n[0] + n[1]*n[1] + n[2]*n[2]
	if l <= 0 {
		return vec3{0, 1, 0}
	}
	inv := 1.0 / sqrt32(l)
	return vec3{n[0] * inv, n[1] * inv, n[2] * inv}
}

func sqrt32(x float32) float32 {
	z := x
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

// slot is one passenger anchor read from a SCAD echo() line.
type slot struct {
	index int
	pos   vec3 // in SCAD coords; convert before writing to OBJ
}

// readSlots scans an openscad stderr capture for MOGUL_META slot
// declarations. The expected format is:
//
//	ECHO: "MOGUL_META", "slot", <index>, <x>, <y>, <z>
//
// Whitespace/quoting tolerant. Lines that don't match are ignored so
// other echoes (debugging output, warnings) don't break the build.
var slotLineRE = regexp.MustCompile(
	`ECHO:\s*"MOGUL_META"\s*,\s*"slot"\s*,\s*` +
		`(-?\d+)\s*,\s*(-?[\d.eE+-]+)\s*,\s*(-?[\d.eE+-]+)\s*,\s*(-?[\d.eE+-]+)`,
)

// footprint is the building's ground-plane half-extents declared by the
// .scad source. Used by the placement-effects pass to size the apron
// and the tree-clearance zone without hardcoding per-type dimensions.
type footprint struct {
	halfX, halfY float32 // SCAD coords; the converter maps Y → game Z
}

// readFootprint scans an openscad stderr capture for a MOGUL_META
// footprint declaration. Format:
//
//	ECHO: "MOGUL_META", "footprint", <halfX>, <halfY>
//
// Only the first matching line is returned (a model has one footprint).
// ok=false when no line is present so the OBJ loader can fall back to a
// box-mesh default.
var footprintLineRE = regexp.MustCompile(
	`ECHO:\s*"MOGUL_META"\s*,\s*"footprint"\s*,\s*` +
		`(-?[\d.eE+-]+)\s*,\s*(-?[\d.eE+-]+)`,
)

func readFootprint(path string) (footprint, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return footprint{}, false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := footprintLineRE.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		hx, _ := strconv.ParseFloat(m[1], 32)
		hy, _ := strconv.ParseFloat(m[2], 32)
		return footprint{halfX: float32(hx), halfY: float32(hy)}, true, nil
	}
	return footprint{}, false, scanner.Err()
}

// partDecl is an animated sub-part attachment declared by a SCAD body model.
// The renderer loads <name>.obj and positions one instance per body instance,
// spinning it around the declared game-space axis.
type partDecl struct {
	name string // OBJ base name, e.g. "helicopter_main_rotor"
	axis string // "spin_y" or "spin_z"
	pos  vec3   // pivot in SCAD space; converted to game space before writing
}

// readParts scans an openscad stderr capture for MOGUL_META part declarations:
//
//	ECHO: "MOGUL_META", "part", "<name>", "<axis>", <x>, <y>, <z>
var partLineRE = regexp.MustCompile(
	`ECHO:\s*"MOGUL_META"\s*,\s*"part"\s*,\s*` +
		`"([^"]+)"\s*,\s*"([^"]+)"\s*,\s*` +
		`(-?[\d.eE+-]+)\s*,\s*(-?[\d.eE+-]+)\s*,\s*(-?[\d.eE+-]+)`,
)

func readParts(path string) ([]partDecl, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var parts []partDecl
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := partLineRE.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		x, _ := strconv.ParseFloat(m[3], 32)
		y, _ := strconv.ParseFloat(m[4], 32)
		z, _ := strconv.ParseFloat(m[5], 32)
		parts = append(parts, partDecl{
			name: m[1],
			axis: m[2],
			pos:  vec3{float32(x), float32(y), float32(z)},
		})
	}
	return parts, scanner.Err()
}

func readSlots(path string) ([]slot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var slots []slot
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := slotLineRE.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		idx, _ := strconv.Atoi(m[1])
		x, _ := strconv.ParseFloat(m[2], 32)
		y, _ := strconv.ParseFloat(m[3], 32)
		z, _ := strconv.ParseFloat(m[4], 32)
		slots = append(slots, slot{
			index: idx,
			pos:   vec3{float32(x), float32(y), float32(z)},
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// Stable ordering by index so the OBJ output is reproducible regardless
	// of the echo order in the .scad source.
	sort.Slice(slots, func(i, j int) bool { return slots[i].index < slots[j].index })
	return slots, nil
}
