package service

import (
	"github.com/uber/h3-go/v4"
)

type H3Service struct{}

func NewH3Service() *H3Service {
	return &H3Service{}
}
func (s *H3Service) MapZoomResolution(zoom int) int {
	if zoom < 12 {
		return 7
	} else if zoom < 15 {
		return 8
	}
	return 9
}
func (s *H3Service) GetTargetHexagons(lat, lng float64, resolution, kRingSize int) []string {
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
