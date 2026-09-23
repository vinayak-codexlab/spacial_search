package service

import "testing"

func TestMapZoomResolution(t *testing.T) {
	s := NewH3Service()
	for _, tt := range []struct{ zoom, resolution int }{
		{0, 6}, {1, 6}, {4, 6}, {5, 7}, {8, 7},
		{9, 8}, {11, 8}, {12, 9}, {14, 9},
		{15, 10}, {17, 10}, {18, 11}, {22, 11},
	} {
		if got := s.MapZoomResolution(tt.zoom); got != tt.resolution {
			t.Errorf("zoom %d: got resolution %d, want %d", tt.zoom, got, tt.resolution)
		}
	}
}
