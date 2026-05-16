package geo

import (
	"image"
	"image/draw"

	sm "github.com/flopp/go-staticmaps"
	"github.com/golang/geo/s2"
)

// PreviewResult holds the rendered map image and the actual lat/lon bounds shown.
type PreviewResult struct {
	Image                          *image.RGBA
	MinLat, MaxLat, MinLon, MaxLon float64
}

// esriImagery returns an ESRI World Imagery tile provider. Satellite/aerial
// photography with clear terrain relief — useful for scouting ski terrain.
// ESRI uses {z}/{y}/{x} tile addressing (y and x are swapped vs. standard).
func esriImagery() *sm.TileProvider {
	return &sm.TileProvider{
		Name:        "ESRI World Imagery",
		Attribution: "Tiles © Esri — Source: Esri, Maxar, GeoEye, Earthstar Geographics, CNES/Airbus DS, USDA, USGS, AeroGRID, IGN, and the GIS User Community",
		TileSize:    256,
		URLPattern:  "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/%[2]d/%[4]d/%[3]d",
		Shards:      []string{},
	}
}

// RenderPreviewAt downloads CartoDB Voyager tiles centred on the given point
// at the given zoom level and renders to an image of w×h pixels.
func RenderPreviewAt(centerLat, centerLon float64, zoom, w, h int) (PreviewResult, error) {
	ctx := sm.NewContext()
	ctx.SetTileProvider(esriImagery())
	ctx.SetSize(w, h)
	ctx.SetMaxZoom(18)
	ctx.SetCenter(s2.LatLngFromDegrees(centerLat, centerLon))
	ctx.SetZoom(zoom)

	img, actualBounds, err := ctx.RenderWithBounds()
	if err != nil {
		return PreviewResult{}, err
	}

	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)

	return PreviewResult{
		Image:  rgba,
		MinLat: actualBounds.Lo().Lat.Degrees(),
		MaxLat: actualBounds.Hi().Lat.Degrees(),
		MinLon: actualBounds.Lo().Lng.Degrees(),
		MaxLon: actualBounds.Hi().Lng.Degrees(),
	}, nil
}

// RenderPreview downloads CartoDB Voyager tiles and renders a map image for
// the bounding box. w and h set the output image dimensions in pixels.
func RenderPreview(minLat, maxLat, minLon, maxLon float64, w, h int) (PreviewResult, error) {
	ctx := sm.NewContext()
	ctx.SetTileProvider(esriImagery())
	ctx.SetSize(w, h)
	// Tile server only serves zoom 0–19; the go-staticmaps default cap is 30,
	// which causes HTTP 400 errors for small bounding boxes.
	ctx.SetMaxZoom(18)

	sw := s2.LatLngFromDegrees(minLat, minLon)
	ne := s2.LatLngFromDegrees(maxLat, maxLon)
	bbox := s2.RectFromLatLng(sw).AddPoint(ne)
	ctx.SetBoundingBox(bbox)

	img, actualBounds, err := ctx.RenderWithBounds()
	if err != nil {
		return PreviewResult{}, err
	}

	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)

	return PreviewResult{
		Image:  rgba,
		MinLat: actualBounds.Lo().Lat.Degrees(),
		MaxLat: actualBounds.Hi().Lat.Degrees(),
		MinLon: actualBounds.Lo().Lng.Degrees(),
		MaxLon: actualBounds.Hi().Lng.Degrees(),
	}, nil
}
