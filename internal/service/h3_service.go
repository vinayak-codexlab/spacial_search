package service

import (
	"github.com/uber/h3-go/v4"
)

// MaxSearchRing bounds GridDisk work and the MongoDB $in filter to 1,951 cells.
// This is deliberately a service limit, rather than merely a BSON-size limit:
// an internet-facing endpoint must not allow one request to trigger an
// arbitrarily expensive geographic scan.
const MaxSearchRing = 25

type H3Service struct{}

func NewH3Service() *H3Service {
	return &H3Service{}
}

// map resolutions based on the zoom level
func (s *H3Service) MapZoomResolution(zoom int) int {
	switch {
	case zoom <= 4:
		return 6
	case zoom <= 8:
		return 7
	case zoom <= 11:
		return 8
	case zoom <= 14:
		return 9
	case zoom <= 17:
		return 10
	default:
		return 11
	}
}
func (s *H3Service) GetTargetHexagons(lat, lng float64, resolution, kRingSize int) []string {
	if kRingSize < 0 || kRingSize > MaxSearchRing {
		return nil
	}
	latLng := h3.NewLatLng(lat, lng)
	originCell, err := h3.LatLngToCell(latLng, resolution)
	if err != nil {
		return nil
	}
	diskCells, err := originCell.GridDisk(kRingSize)
	if err != nil {
		return nil
	}
	targetHexes := make([]string, len(diskCells))
	for i, cell := range diskCells {
		targetHexes[i] = cell.String()
	}
	return targetHexes
}
