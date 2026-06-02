package sim

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	"mountain-mogul/internal/world"
)

// Row is a flat string→scalar map produced by reflecting one entity.
type Row = map[string]any

// TableSet maps table names to their row slices.
type TableSet map[string][]Row

// BuildTableSet constructs all queryable tables from the current sim/world state.
// Tables: cells, guests, cats, buildings, lifts, patrollers.
func BuildTableSet(w *world.World, s *Simulation) TableSet {
	const cellSize = float32(5.0)
	ts := TableSet{}
	ts["cells"] = buildCellRows(w.Terrain, s)
	ts["guests"] = buildEntityRows(w.OnMountain, func(g *world.Guest) Row {
		row := reflectRow(reflect.ValueOf(*g), "")
		row["activity"] = world.Activity(w, g)
		// followed: 1 for the camera-followed guest, 0 otherwise.
		if g.ID == w.FocusedGuestID {
			row["followed"] = float64(1)
		} else {
			row["followed"] = float64(0)
		}
		// Terrain columns at the guest's XZ position.
		if w.Terrain != nil {
			px, pz := g.Pos[0], g.Pos[2]
			cx := int(px / cellSize)
			cz := int(pz / cellSize)
			row["cell_x"] = float64(cx)
			row["cell_z"] = float64(cz)
			row["surface_elev"] = float64(w.Terrain.InterpolatedSurfaceElevationAt(px, pz))
			row["ground_elev"] = float64(w.Terrain.InterpolatedGroundElevationAt(px, pz))
			row["snow_depth"] = float64(w.Terrain.InterpolatedSurfaceElevationAt(px, pz) - w.Terrain.InterpolatedGroundElevationAt(px, pz))
		}
		return row
	})
	ts["cats"] = buildEntityRows(w.Snowcats, func(c *world.Snowcat) Row {
		return reflectRow(reflect.ValueOf(*c), "")
	})
	ts["buildings"] = buildEntityRows(w.Buildings, func(b *world.Building) Row {
		return reflectRow(reflect.ValueOf(*b), "")
	})
	ts["lifts"] = buildEntityRows(w.Lifts, func(l *world.Lift) Row {
		return reflectRow(reflect.ValueOf(*l), "")
	})
	ts["patrollers"] = buildEntityRows(w.Patrollers, func(p *world.Patroller) Row {
		return reflectRow(reflect.ValueOf(*p), "")
	})
	return ts
}

// RunQuery executes one or more ";" separated SQL queries against ts and
// returns formatted output. WHERE expressions use Go syntax (&&, ||, ==).
// AND/OR are accepted as aliases.
func RunQuery(ts TableSet, queries string) (string, error) {
	var buf bytes.Buffer
	for _, q := range strings.Split(queries, ";") {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		result, err := runOneQuery(ts, q)
		if err != nil {
			fmt.Fprintf(&buf, "--- %s\nERROR: %v\n\n", q, err)
			continue
		}
		fmt.Fprintf(&buf, "--- %s\n%s\n", q, result)
	}
	return buf.String(), nil
}

// =============================================================================
// Row building
// =============================================================================

func buildCellRows(t *world.Terrain, s *Simulation) []Row {
	rows := make([]Row, 0, t.Width*t.Height)
	for x := range t.Cells {
		for z := range t.Cells[x] {
			c := &t.Cells[x][z]
			row := reflectRow(reflect.ValueOf(*c), "")
			row["x"] = float64(x)
			row["z"] = float64(z)
			row["instability_score"] = float64(c.InstabilityScore())
			row["surface_elev"] = float64(c.SurfaceElevation())
			row["snow_depth"] = float64(c.VisibleSnowDepth())
			// Friendly alias: top_swe == top_accumulation
			if v, ok := row["top_accumulation"]; ok {
				row["top_swe"] = v
			}
			rows = append(rows, row)
		}
	}
	return rows
}

func buildEntityRows[T any](items []T, build func(T) Row) []Row {
	rows := make([]Row, len(items))
	for i, item := range items {
		rows[i] = build(item)
	}
	return rows
}

// reflectRow walks v (must be a struct) and returns a flat map.
// Nested structs are flattened with an underscore prefix.
// Arrays ([2]float32, [3]float32, [2]int) are flattened to col_x/col_y/col_z.
// Slices, maps, channels, funcs, and external-package time.Time are skipped.
func reflectRow(v reflect.Value, prefix string) Row {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return Row{}
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return Row{}
	}
	row := Row{}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fv := v.Field(i)
		if !f.IsExported() {
			continue
		}
		colName := prefix + toSnake(f.Name)
		flattenValue(colName, fv, row)
	}
	return row
}

func flattenValue(name string, v reflect.Value, row Row) {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		row[name] = v.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		row[name] = float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		row[name] = float64(v.Uint())
	case reflect.Bool:
		if v.Bool() {
			row[name] = float64(1)
		} else {
			row[name] = float64(0)
		}
	case reflect.String:
		row[name] = v.String()
	case reflect.Struct:
		// Skip time.Time and other external types we can't meaningfully flatten
		if v.Type().PkgPath() == "time" {
			return
		}
		sub := reflectRow(v, name+"_")
		for k, val := range sub {
			row[k] = val
		}
	case reflect.Array:
		flattenArray(name, v, row)
	// Slices, maps, funcs, chans, ptrs-to-non-struct: skip
	}
}

func flattenArray(name string, v reflect.Value, row Row) {
	n := v.Len()
	if n == 0 {
		return
	}
	elem := v.Type().Elem()
	// Choose suffix style based on length and element type
	var suffixes []string
	switch {
	case n == 2 && (elem.Kind() == reflect.Float32 || elem.Kind() == reflect.Float64):
		suffixes = []string{"x", "z"} // Vec2 = horizontal plane
	case n == 3 && (elem.Kind() == reflect.Float32 || elem.Kind() == reflect.Float64):
		suffixes = []string{"x", "y", "z"} // Vec3
	case n <= 4:
		suffixes = []string{"0", "1", "2", "3"}[:n]
	default:
		return // skip long arrays
	}
	for i := 0; i < n; i++ {
		flattenValue(name+"_"+suffixes[i], v.Index(i), row)
	}
}

// toSnake converts CamelCase to snake_case.
func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(rune(s[i-1])) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// =============================================================================
// SQL execution
// =============================================================================

type parsedSQL struct {
	cols    []string // nil = SELECT *
	table   string
	where   string // raw expr; empty = no filter
	orderBy string // column name; empty = no sort
	asc     bool
	limit   int // -1 = no limit
}

func runOneQuery(ts TableSet, q string) (string, error) {
	sq, err := parseSQL(q)
	if err != nil {
		return "", err
	}
	rows, ok := ts[sq.table]
	if !ok {
		names := make([]string, 0, len(ts))
		for k := range ts {
			names = append(names, k)
		}
		sort.Strings(names)
		return "", fmt.Errorf("unknown table %q; available: %s", sq.table, strings.Join(names, ", "))
	}

	// Filter
	if sq.where != "" {
		rows, err = filterRows(rows, sq.where)
		if err != nil {
			return "", fmt.Errorf("WHERE: %w", err)
		}
	}

	// Detect aggregates
	if len(sq.cols) > 0 && isAggregateQuery(sq.cols) {
		return evalAggregateQuery(rows, sq.cols)
	}

	// Sort
	if sq.orderBy != "" {
		sortRows(rows, sq.orderBy, sq.asc)
	}

	// Limit
	if sq.limit >= 0 && len(rows) > sq.limit {
		rows = rows[:sq.limit]
	}

	// Project
	return projectRows(rows, sq.cols)
}

// parseSQL parses a SELECT … FROM … [WHERE …] [ORDER BY … [DESC]] [LIMIT n].
func parseSQL(raw string) (parsedSQL, error) {
	sq := parsedSQL{asc: true, limit: -1}

	up := strings.ToUpper(raw)

	mustHave := func(kw string) int {
		return strings.Index(up, kw)
	}

	selectIdx := mustHave("SELECT ")
	if selectIdx < 0 {
		return sq, fmt.Errorf("expected SELECT")
	}
	fromIdx := strings.Index(up, " FROM ")
	if fromIdx < 0 {
		return sq, fmt.Errorf("expected FROM")
	}
	whereIdx := strings.Index(up, " WHERE ")
	orderIdx := strings.Index(up, " ORDER BY ")
	limitIdx := strings.Index(up, " LIMIT ")

	// Columns — between SELECT and FROM
	colsRaw := strings.TrimSpace(raw[selectIdx+7 : fromIdx])
	if colsRaw == "*" {
		sq.cols = nil
	} else {
		for _, c := range strings.Split(colsRaw, ",") {
			sq.cols = append(sq.cols, strings.TrimSpace(c))
		}
	}

	// Table — between FROM and next keyword
	tableEnd := len(raw)
	for _, pos := range []int{whereIdx, orderIdx, limitIdx} {
		if pos > fromIdx && pos < tableEnd {
			tableEnd = pos
		}
	}
	sq.table = strings.ToLower(strings.TrimSpace(raw[fromIdx+6 : tableEnd]))

	// WHERE expression
	if whereIdx >= 0 {
		exprEnd := len(raw)
		for _, pos := range []int{orderIdx, limitIdx} {
			if pos > whereIdx && pos < exprEnd {
				exprEnd = pos
			}
		}
		sq.where = strings.TrimSpace(raw[whereIdx+7 : exprEnd])
	}

	// ORDER BY
	if orderIdx >= 0 {
		orderEnd := len(raw)
		if limitIdx > orderIdx && limitIdx < orderEnd {
			orderEnd = limitIdx
		}
		orderStr := strings.TrimSpace(raw[orderIdx+10 : orderEnd])
		upOrder := strings.ToUpper(orderStr)
		if strings.HasSuffix(upOrder, " DESC") {
			sq.orderBy = strings.ToLower(strings.TrimSpace(orderStr[:len(orderStr)-5]))
			sq.asc = false
		} else if strings.HasSuffix(upOrder, " ASC") {
			sq.orderBy = strings.ToLower(strings.TrimSpace(orderStr[:len(orderStr)-4]))
		} else {
			sq.orderBy = strings.ToLower(orderStr)
		}
	}

	// LIMIT
	if limitIdx >= 0 {
		limitStr := strings.TrimSpace(raw[limitIdx+7:])
		if f := strings.Fields(limitStr); len(f) > 0 {
			n, err := strconv.Atoi(f[0])
			if err != nil {
				return sq, fmt.Errorf("invalid LIMIT value %q", f[0])
			}
			sq.limit = n
		}
	}

	return sq, nil
}

// =============================================================================
// WHERE filtering
// =============================================================================

// filterRows returns the subset of rows for which the Go-syntax expression
// evaluates to true. AND/OR are accepted as aliases for &&/||.
func filterRows(rows []Row, expr string) ([]Row, error) {
	expr = normalizeExpr(expr)
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", expr, err)
	}
	var out []Row
	for _, row := range rows {
		v, err := evalExpr(node, row)
		if err != nil {
			return nil, err
		}
		if toBool(v) {
			out = append(out, row)
		}
	}
	return out, nil
}

// normalizeExpr translates SQL-ish expression syntax into valid Go syntax.
func normalizeExpr(s string) string {
	// AND/OR → &&/||
	for _, pair := range [][2]string{
		{" AND ", " && "}, {" and ", " && "},
		{" OR ", " || "}, {" or ", " || "},
	} {
		s = strings.ReplaceAll(s, pair[0], pair[1])
	}
	// Single = → ==  (skip !=, <=, >=, ==)
	out := make([]byte, 0, len(s)+4)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '=' && (i == 0 || (s[i-1] != '!' && s[i-1] != '<' && s[i-1] != '>' && s[i-1] != '=')) {
			if i+1 < len(s) && s[i+1] != '=' {
				out = append(out, '=', '=')
				continue
			}
		}
		out = append(out, c)
	}
	return string(out)
}

func evalExpr(node ast.Expr, row Row) (any, error) {
	switch e := node.(type) {
	case *ast.Ident:
		key := strings.ToLower(e.Name)
		if key == "true" {
			return float64(1), nil
		}
		if key == "false" {
			return float64(0), nil
		}
		val, ok := row[key]
		if !ok {
			return nil, fmt.Errorf("unknown column %q", key)
		}
		return val, nil

	case *ast.SelectorExpr: // top.accumulation → top_accumulation
		xIdent, ok := e.X.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("unsupported selector expression")
		}
		key := strings.ToLower(xIdent.Name) + "_" + strings.ToLower(e.Sel.Name)
		val, ok := row[key]
		if !ok {
			return nil, fmt.Errorf("unknown column %q", key)
		}
		return val, nil

	case *ast.BasicLit:
		switch e.Kind.String() {
		case "INT":
			n, _ := strconv.ParseInt(e.Value, 10, 64)
			return float64(n), nil
		case "FLOAT":
			f, _ := strconv.ParseFloat(e.Value, 64)
			return f, nil
		case "STRING":
			s, _ := strconv.Unquote(e.Value)
			return s, nil
		}
		return nil, fmt.Errorf("unsupported literal %s", e.Value)

	case *ast.UnaryExpr:
		x, err := evalExpr(e.X, row)
		if err != nil {
			return nil, err
		}
		switch e.Op.String() {
		case "!":
			if toBool(x) {
				return float64(0), nil
			}
			return float64(1), nil
		case "-":
			return -toNum(x), nil
		}
		return nil, fmt.Errorf("unsupported unary op %s", e.Op)

	case *ast.BinaryExpr:
		left, err := evalExpr(e.X, row)
		if err != nil {
			return nil, err
		}
		right, err := evalExpr(e.Y, row)
		if err != nil {
			return nil, err
		}
		return evalBinary(e.Op.String(), left, right)

	case *ast.ParenExpr:
		return evalExpr(e.X, row)
	}
	return nil, fmt.Errorf("unsupported expression type %T", node)
}

func evalBinary(op string, left, right any) (any, error) {
	// String comparison
	ls, lok := left.(string)
	rs, rok := right.(string)
	if lok && rok {
		switch op {
		case "==":
			return boolToFloat(ls == rs), nil
		case "!=":
			return boolToFloat(ls != rs), nil
		case "<":
			return boolToFloat(ls < rs), nil
		case ">":
			return boolToFloat(ls > rs), nil
		}
		return nil, fmt.Errorf("unsupported string op %s", op)
	}
	// Numeric
	l, r := toNum(left), toNum(right)
	switch op {
	case "+":
		return l + r, nil
	case "-":
		return l - r, nil
	case "*":
		return l * r, nil
	case "/":
		if r == 0 {
			return math.NaN(), nil
		}
		return l / r, nil
	case "==":
		return boolToFloat(l == r), nil
	case "!=":
		return boolToFloat(l != r), nil
	case "<":
		return boolToFloat(l < r), nil
	case "<=":
		return boolToFloat(l <= r), nil
	case ">":
		return boolToFloat(l > r), nil
	case ">=":
		return boolToFloat(l >= r), nil
	case "&&":
		return boolToFloat(toBool(left) && toBool(right)), nil
	case "||":
		return boolToFloat(toBool(left) || toBool(right)), nil
	}
	return nil, fmt.Errorf("unsupported operator %s", op)
}

// =============================================================================
// Aggregates
// =============================================================================

func isAggregateQuery(cols []string) bool {
	for _, c := range cols {
		cu := strings.ToUpper(c)
		for _, fn := range []string{"COUNT(", "AVG(", "SUM(", "MIN(", "MAX("} {
			if strings.Contains(cu, fn) {
				return true
			}
		}
	}
	return false
}

func evalAggregateQuery(rows []Row, cols []string) (string, error) {
	headers := make([]string, len(cols))
	vals := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = c
		v, err := evalAggregate(rows, c)
		if err != nil {
			return "", err
		}
		vals[i] = fmtNum(v)
	}
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Join(vals, "\t"))
	w.Flush()
	return buf.String(), nil
}

func evalAggregate(rows []Row, col string) (float64, error) {
	upper := strings.ToUpper(strings.TrimSpace(col))
	for _, fn := range []string{"COUNT", "AVG", "SUM", "MIN", "MAX"} {
		if !strings.HasPrefix(upper, fn+"(") {
			continue
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(upper, fn+"("), ")")
		inner = strings.ToLower(strings.TrimSpace(inner))
		switch fn {
		case "COUNT":
			if inner == "*" {
				return float64(len(rows)), nil
			}
			n := 0
			for _, row := range rows {
				if _, ok := row[inner]; ok {
					n++
				}
			}
			return float64(n), nil
		case "AVG":
			sum, n := 0.0, 0
			for _, row := range rows {
				if v, ok := row[inner]; ok {
					sum += toNum(v)
					n++
				}
			}
			if n == 0 {
				return 0, nil
			}
			return sum / float64(n), nil
		case "SUM":
			sum := 0.0
			for _, row := range rows {
				if v, ok := row[inner]; ok {
					sum += toNum(v)
				}
			}
			return sum, nil
		case "MIN":
			min := math.Inf(1)
			for _, row := range rows {
				if v, ok := row[inner]; ok {
					if f := toNum(v); f < min {
						min = f
					}
				}
			}
			if math.IsInf(min, 1) {
				return 0, nil
			}
			return min, nil
		case "MAX":
			max := math.Inf(-1)
			for _, row := range rows {
				if v, ok := row[inner]; ok {
					if f := toNum(v); f > max {
						max = f
					}
				}
			}
			if math.IsInf(max, -1) {
				return 0, nil
			}
			return max, nil
		}
	}
	return 0, fmt.Errorf("unrecognised aggregate %q", col)
}

// =============================================================================
// Projection and output
// =============================================================================

func sortRows(rows []Row, col string, asc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		a := toNum(rows[i][col])
		b := toNum(rows[j][col])
		if asc {
			return a < b
		}
		return a > b
	})
}

// projectRows returns a tabwriter-formatted table. cols nil = all columns.
func projectRows(rows []Row, cols []string) (string, error) {
	if len(rows) == 0 {
		return "(0 rows)\n", nil
	}

	// Determine columns
	headers := cols
	if headers == nil {
		// All columns from the first row, sorted
		seen := map[string]bool{}
		for _, row := range rows {
			for k := range row {
				seen[k] = true
			}
		}
		headers = make([]string, 0, len(seen))
		for k := range seen {
			headers = append(headers, k)
		}
		sort.Strings(headers)
	}

	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range rows {
		parts := make([]string, len(headers))
		for i, h := range headers {
			parts[i] = fmtVal(row[strings.ToLower(h)])
		}
		fmt.Fprintln(tw, strings.Join(parts, "\t"))
	}
	tw.Flush()
	fmt.Fprintf(&buf, "(%d rows)\n", len(rows))
	return buf.String(), nil
}

// =============================================================================
// Helpers
// =============================================================================

func toNum(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	}
	return 0
}

func toBool(v any) bool {
	switch x := v.(type) {
	case float64:
		return x != 0 && !math.IsNaN(x)
	case bool:
		return x
	case string:
		return x != ""
	}
	return false
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func fmtNum(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "+Inf"
	}
	if math.IsInf(f, -1) {
		return "-Inf"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e9 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', 4, 64)
}

func fmtVal(v any) string {
	if v == nil {
		return "NULL"
	}
	switch x := v.(type) {
	case float64:
		return fmtNum(x)
	case string:
		return x
	}
	return fmt.Sprint(v)
}
