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

func TestSearchRingBounds(t *testing.T) {
	s := NewH3Service()
	for _, ring := range []int{-1, MaxSearchRing + 1, 2147483647} {
		if cells := s.GetTargetHexagons(28, 77, 9, ring); cells != nil {
			t.Fatalf("ring %d should be rejected", ring)
		}
	}
	cells := s.GetTargetHexagons(28, 77, 9, MaxSearchRing)
	if want := 1 + 3*MaxSearchRing*(MaxSearchRing+1); len(cells) != want {
		t.Fatalf("maximum ring produced %d cells, want %d", len(cells), want)
	}
}
