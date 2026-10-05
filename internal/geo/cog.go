package geo

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync"

	"golang.org/x/image/tiff/lzw"
)

// cogReader reads float32 elevation from a tiled GeoTIFF over HTTP range
// requests, fetching only the tiles a caller samples. Supports what USGS
// 3DEP 1 m DEMs use: one band of float32, 512² tiles, LZW or none, with
// or without the floating-point predictor, and a tiepoint plus pixel
// scale (no rotation).
type cogReader struct {
	url    string
	client *http.Client
	order  binary.ByteOrder

	width, height, tileW, tileH int
	predictor                   int
	compression                 int
	offsets, counts             []uint64

	originX, originY float64 // model coordinates of the top-left pixel corner
	scaleX, scaleY   float64 // metres per pixel
	noData           float64

	mu    sync.Mutex
	tiles map[int][]float32
}

const (
	tiffTagWidth       = 256
	tiffTagHeight      = 257
	tiffTagBits        = 258
	tiffTagCompression = 259
	tiffTagPredictor   = 317
	tiffTagTileW       = 322
	tiffTagTileH       = 323
	tiffTagTileOffsets = 324
	tiffTagTileCounts  = 325
	tiffTagSampleFmt   = 339
	tiffTagPixelScale  = 33550
	tiffTagTiepoint    = 33922
	tiffTagNoData      = 42113
)

func openCOG(ctx context.Context, client *http.Client, url string) (*cogReader, error) {
	r := &cogReader{url: url, client: client, tiles: map[int][]float32{}}
	head, err := r.fetch(ctx, 0, 1<<16)
	if err != nil {
		return nil, err
	}
	switch string(head[:2]) {
	case "II":
		r.order = binary.LittleEndian
	case "MM":
		r.order = binary.BigEndian
	default:
		return nil, fmt.Errorf("%s: not a TIFF", url)
	}
	if r.order.Uint16(head[2:]) != 42 {
		return nil, fmt.Errorf("%s: BigTIFF or unknown TIFF variant", url)
	}
	ifd := int(r.order.Uint32(head[4:]))
	if ifd+2 > len(head) {
		return nil, fmt.Errorf("%s: first IFD beyond header", url)
	}

	// value returns an entry's raw bytes, fetching them if they lie
	// beyond the header.
	value := func(typ, count uint32, field []byte) ([]byte, error) {
		size := int(count) * map[uint32]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 11: 4, 12: 8, 16: 8}[typ]
		if size <= 4 {
			return field[:size], nil
		}
		off := int(r.order.Uint32(field))
		if off+size <= len(head) {
			return head[off : off+size], nil
		}
		return r.fetch(ctx, int64(off), int64(size))
	}
	ints := func(typ uint32, b []byte) []uint64 {
		var out []uint64
		switch typ {
		case 3:
			for i := 0; i+2 <= len(b); i += 2 {
				out = append(out, uint64(r.order.Uint16(b[i:])))
			}
		case 4:
			for i := 0; i+4 <= len(b); i += 4 {
				out = append(out, uint64(r.order.Uint32(b[i:])))
			}
		case 16:
			for i := 0; i+8 <= len(b); i += 8 {
				out = append(out, r.order.Uint64(b[i:]))
			}
		}
		return out
	}
	doubles := func(b []byte) []float64 {
		var out []float64
		for i := 0; i+8 <= len(b); i += 8 {
			out = append(out, math.Float64frombits(r.order.Uint64(b[i:])))
		}
		return out
	}

	n := int(r.order.Uint16(head[ifd:]))
	var bits, sampleFmt uint64 = 32, 3
	var tie []float64
	r.noData = math.NaN()
	for i := 0; i < n; i++ {
		e := head[ifd+2+i*12 : ifd+14+i*12]
		tag := r.order.Uint16(e)
		typ := uint32(r.order.Uint16(e[2:]))
		count := r.order.Uint32(e[4:])
		b, err := value(typ, count, e[8:12])
		if err != nil {
			return nil, err
		}
		first := func() int {
			if v := ints(typ, b); len(v) > 0 {
				return int(v[0])
			}
			return 0
		}
		switch tag {
		case tiffTagWidth:
			r.width = first()
		case tiffTagHeight:
			r.height = first()
		case tiffTagBits:
			bits = uint64(first())
		case tiffTagCompression:
			r.compression = first()
		case tiffTagPredictor:
			r.predictor = first()
		case tiffTagTileW:
			r.tileW = first()
		case tiffTagTileH:
			r.tileH = first()
		case tiffTagTileOffsets:
			r.offsets = ints(typ, b)
		case tiffTagTileCounts:
			r.counts = ints(typ, b)
		case tiffTagSampleFmt:
			sampleFmt = uint64(first())
		case tiffTagPixelScale:
			if d := doubles(b); len(d) >= 2 {
				r.scaleX, r.scaleY = d[0], d[1]
			}
		case tiffTagTiepoint:
			tie = doubles(b)
		case tiffTagNoData:
			if v, err := strconv.ParseFloat(string(bytes.TrimRight(b, "\x00 ")), 64); err == nil {
				r.noData = v
			}
		}
	}
	if len(tie) < 6 {
		return nil, fmt.Errorf("%s: no tiepoint", url)
	}
	r.originX, r.originY = tie[3]-tie[0]*r.scaleX, tie[4]+tie[1]*r.scaleY
	if r.tileW == 0 || r.tileH == 0 || bits != 32 || sampleFmt != 3 {
		return nil, fmt.Errorf("%s: need tiled float32 (tile %dx%d, %d bits, format %d)", url, r.tileW, r.tileH, bits, sampleFmt)
	}
	if r.compression != 1 && r.compression != 5 {
		return nil, fmt.Errorf("%s: unsupported compression %d", url, r.compression)
	}
	if r.scaleX == 0 || r.scaleY == 0 {
		return nil, fmt.Errorf("%s: no pixel scale", url)
	}
	return r, nil
}

func (r *cogReader) fetch(ctx context.Context, off, n int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+n-1))
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %s", r.url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// tile returns tile index ti decoded to float32 (NaN for no data),
// fetching it on first use.
func (r *cogReader) tile(ctx context.Context, ti int) ([]float32, error) {
	r.mu.Lock()
	t, ok := r.tiles[ti]
	r.mu.Unlock()
	if ok {
		return t, nil
	}
	raw, err := r.fetch(ctx, int64(r.offsets[ti]), int64(r.counts[ti]))
	if err != nil {
		return nil, err
	}
	if r.compression == 5 {
		zr := lzw.NewReader(bytes.NewReader(raw), lzw.MSB, 8)
		raw, err = io.ReadAll(zr)
		zr.Close()
		if err != nil {
			return nil, fmt.Errorf("%s tile %d: %w", r.url, ti, err)
		}
	}
	px := r.tileW * r.tileH
	if len(raw) < px*4 {
		return nil, fmt.Errorf("%s tile %d: %d bytes, want %d", r.url, ti, len(raw), px*4)
	}
	t = make([]float32, px)
	switch r.predictor {
	case 3:
		// Floating-point predictor: each row is byte-differenced, with
		// the bytes of every sample split into planes, most significant
		// first.
		row := r.tileW * 4
		for y := 0; y < r.tileH; y++ {
			b := raw[y*row : (y+1)*row]
			for i := 1; i < row; i++ {
				b[i] += b[i-1]
			}
			for x := 0; x < r.tileW; x++ {
				u := uint32(b[x])<<24 | uint32(b[r.tileW+x])<<16 | uint32(b[2*r.tileW+x])<<8 | uint32(b[3*r.tileW+x])
				t[y*r.tileW+x] = math.Float32frombits(u)
			}
		}
	case 0, 1:
		for i := range t {
			t[i] = math.Float32frombits(r.order.Uint32(raw[i*4:]))
		}
	default:
		return nil, fmt.Errorf("%s: unsupported predictor %d", r.url, r.predictor)
	}
	for i, v := range t {
		if float64(v) == r.noData || v < -1000 || v > 10000 {
			t[i] = float32(math.NaN())
		}
	}
	r.mu.Lock()
	r.tiles[ti] = t
	r.mu.Unlock()
	return t, nil
}

// pixel is the value at pixel (px, py), NaN outside the raster or where
// there's no data.
func (r *cogReader) pixel(ctx context.Context, px, py int) (float32, error) {
	if px < 0 || py < 0 || px >= r.width || py >= r.height {
		return float32(math.NaN()), nil
	}
	across := (r.width + r.tileW - 1) / r.tileW
	t, err := r.tile(ctx, (py/r.tileH)*across+px/r.tileW)
	if err != nil {
		return 0, err
	}
	return t[(py%r.tileH)*r.tileW+px%r.tileW], nil
}

// sample is the bilinear value at model coordinates (x, y), NaN if any
// of the four pixels is missing.
func (r *cogReader) sample(ctx context.Context, x, y float64) (float32, error) {
	// Pixel centres sit half a pixel in from the top-left corner.
	fx := (x-r.originX)/r.scaleX - 0.5
	fy := (r.originY-y)/r.scaleY - 0.5
	x0, y0 := int(math.Floor(fx)), int(math.Floor(fy))
	tx, ty := float32(fx-float64(x0)), float32(fy-float64(y0))
	var v [4]float32
	for k, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		p, err := r.pixel(ctx, x0+d[0], y0+d[1])
		if err != nil {
			return 0, err
		}
		v[k] = p
	}
	top := v[0]*(1-tx) + v[1]*tx
	bot := v[2]*(1-tx) + v[3]*tx
	return top*(1-ty) + bot*ty, nil
}

// contains reports whether model point (x, y) lies inside the raster.
func (r *cogReader) contains(x, y float64) bool {
	return x >= r.originX && x < r.originX+float64(r.width)*r.scaleX &&
		y <= r.originY && y > r.originY-float64(r.height)*r.scaleY
}
