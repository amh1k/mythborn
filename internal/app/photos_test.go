package app

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestPhotoNormalizesOrientationAndDropsMetadata(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 30, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 30; x++ {
			c := color.RGBA{R: 255, A: 255}
			if x >= 15 {
				c = color.RGBA{B: 255, A: 255}
			}
			source.Set(x, y, c)
		}
	}
	var original bytes.Buffer
	if err := jpeg.Encode(&original, source, nil); err != nil {
		t.Fatal(err)
	}
	tiff := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 0x01, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	payload = append(payload, []byte("private GPS location")...)
	segment := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(payload)+2))
	segment = append(segment, payload...)
	data := append(append(append([]byte{}, original.Bytes()[:2]...), segment...), original.Bytes()[2:]...)
	processed, err := normalizePhoto(data)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(processed, []byte("Exif")) || bytes.Contains(processed, []byte("GPS")) {
		t.Fatal("metadata retained")
	}
	result, err := jpeg.Decode(bytes.NewReader(processed))
	if err != nil {
		t.Fatal(err)
	}
	if result.Bounds().Dx() != 10 || result.Bounds().Dy() != 30 {
		t.Fatalf("orientation not applied: %v", result.Bounds())
	}
	r, _, b, _ := result.At(5, 5).RGBA()
	if r <= b {
		t.Fatal("rotated image lost top red half")
	}
	r, _, b, _ = result.At(5, 25).RGBA()
	if b <= r {
		t.Fatal("rotated image lost bottom blue half")
	}
}

func TestPhotoRejectsPixelBombAndCorruptInput(t *testing.T) {
	var output bytes.Buffer
	_ = png.Encode(&output, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	data := append([]byte{}, output.Bytes()...)
	binary.BigEndian.PutUint32(data[16:20], 6000)
	binary.BigEndian.PutUint32(data[20:24], 6000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	for _, bad := range [][]byte{data, []byte("\xff\xd8\xffbroken"), output.Bytes()[:len(output.Bytes())/2]} {
		if _, err := normalizePhoto(bad); !errors.Is(err, ErrInvalid) {
			t.Fatalf("malformed/oversized photo accepted: %v", err)
		}
	}
}

func TestPhotoResizesAndHandlesAllOrientationTransforms(t *testing.T) {
	var data bytes.Buffer
	_ = png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 3000, 100)))
	processed, err := normalizePhoto(data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	c, err := jpeg.DecodeConfig(bytes.NewReader(processed))
	if err != nil || c.Width != 2048 || c.Height != 68 {
		t.Fatalf("incorrect resized dimensions: %+v %v", c, err)
	}
	source := image.NewRGBA(image.Rect(0, 0, 3, 2))
	source.Set(0, 0, color.RGBA{R: 255, A: 255})
	locations := [][2]int{{0, 0}, {2, 0}, {2, 1}, {0, 1}, {0, 0}, {1, 0}, {1, 2}, {0, 2}}
	for orientation := 1; orientation <= 8; orientation++ {
		out := orientImage(source, orientation)
		pos := locations[orientation-1]
		r, _, _, _ := out.At(pos[0], pos[1]).RGBA()
		if r != 65535 {
			t.Errorf("orientation %d incorrect", orientation)
		}
	}
}
