package nodes

import (
	"bytes"
	"testing"
)

func Test_encodeNodeLocations(t *testing.T) {
	buf := encodeNodeLocations([]nodeLocation{
		{Lon: -180, Lat: -90},
		{Lon: 0, Lat: 0},
		{Lon: 90, Lat: 45},
		{Lon: -90, Lat: 60},
	})
	expected := []byte{
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x80, 0x00, 0x80,
		0x00, 0xC0, 0x00, 0xC0,
		0x00, 0x40, 0x55, 0xD5,
	}
	if !bytes.Equal(buf, expected) {
		t.Errorf("expected % x, got % x", expected, buf)
	}

	// an empty group must be saved as an empty value, and nil would become NULL
	if buf := encodeNodeLocations(nil); buf == nil || len(buf) != 0 {
		t.Errorf("expected empty non-nil buffer, got %#v", buf)
	}
}
