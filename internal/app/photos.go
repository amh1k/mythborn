package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const maxPhotoPixels = 24_000_000
const normalizedPhotoEdge = 2048

var photoProcessingSlots = make(chan struct{}, 1)

func normalizePhotoWithContext(ctx context.Context, data []byte) ([]byte, error) {
	select {
	case photoProcessingSlots <- struct{}{}:
		defer func() { <-photoProcessingSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return normalizePhoto(data)
}

// Only the normalized JPEG is retained. Re-encoding drops EXIF, GPS, comments,
// and other metadata; orientation is applied to pixels before resizing.
func normalizePhoto(data []byte) ([]byte, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return nil, fmt.Errorf("%w: choose a valid JPEG, PNG, or WebP photo", ErrInvalid)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 16000 || config.Height > 16000 || int64(config.Width)*int64(config.Height) > maxPhotoPixels {
		return nil, fmt.Errorf("%w: photo exceeds the 24 megapixel processing limit", ErrInvalid)
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: this photo is damaged; export it again and retry", ErrInvalid)
	}
	source = orientImage(source, photoOrientation(data, format))
	bounds := source.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if max(w, h) > normalizedPhotoEdge {
		if w >= h {
			h = max(1, h*normalizedPhotoEdge/w)
			w = normalizedPhotoEdge
		} else {
			w = max(1, w*normalizedPhotoEdge/h)
			h = normalizedPhotoEdge
		}
	}
	result := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(result, result.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	xdraw.BiLinear.Scale(result, result.Bounds(), source, bounds, draw.Over, nil)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, result, &jpeg.Options{Quality: 88}); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func photoOrientation(data []byte, format string) int {
	if format == "jpeg" {
		for pos := 2; pos+4 <= len(data); {
			if data[pos] != 0xff {
				break
			}
			marker := data[pos+1]
			if marker == 0xda || marker == 0xd9 {
				break
			}
			if marker == 0xff {
				pos++
				continue
			}
			if marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
				pos += 2
				continue
			}
			length := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
			if length < 2 || length > len(data)-pos-2 {
				break
			}
			if marker == 0xe1 && bytes.HasPrefix(data[pos+4:pos+2+length], []byte("Exif\x00\x00")) {
				return tiffOrientation(data[pos+10 : pos+2+length])
			}
			pos += 2 + length
		}
	} else if format == "webp" {
		for pos := 12; pos+8 <= len(data); {
			length := uint64(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
			if length > uint64(len(data)-pos-8) {
				break
			}
			if string(data[pos:pos+4]) == "EXIF" {
				return tiffOrientation(bytes.TrimPrefix(data[pos+8:pos+8+int(length)], []byte("Exif\x00\x00")))
			}
			pos += 8 + int(length) + (int(length) & 1)
		}
	} else if format == "png" {
		for pos := 8; pos+12 <= len(data); {
			length := uint64(binary.BigEndian.Uint32(data[pos : pos+4]))
			if length > uint64(len(data)-pos-12) {
				break
			}
			if string(data[pos+4:pos+8]) == "eXIf" {
				return tiffOrientation(data[pos+8 : pos+8+int(length)])
			}
			pos += 12 + int(length)
		}
	}
	return 1
}

func tiffOrientation(data []byte) int {
	if len(data) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(data[2:4]) != 42 {
		return 1
	}
	offset := uint64(order.Uint32(data[4:8]))
	if offset+2 > uint64(len(data)) {
		return 1
	}
	count := int(order.Uint16(data[offset : offset+2]))
	for i, pos := 0, int(offset)+2; i < count && pos+12 <= len(data); i, pos = i+1, pos+12 {
		if order.Uint16(data[pos:pos+2]) == 0x0112 && order.Uint16(data[pos+2:pos+4]) == 3 && order.Uint32(data[pos+4:pos+8]) == 1 {
			value := int(order.Uint16(data[pos+8 : pos+10]))
			if value >= 1 && value <= 8 {
				return value
			}
		}
	}
	return 1
}

func orientImage(source image.Image, orientation int) image.Image {
	if orientation == 1 {
		return source
	}
	b := source.Bounds()
	w, h := b.Dx(), b.Dy()
	ow, oh := w, h
	if orientation >= 5 {
		ow, oh = h, w
	}
	output := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ox, oy := x, y
			switch orientation {
			case 2:
				ox = w - 1 - x
			case 3:
				ox, oy = w-1-x, h-1-y
			case 4:
				oy = h - 1 - y
			case 5:
				ox, oy = y, x
			case 6:
				ox, oy = h-1-y, x
			case 7:
				ox, oy = h-1-y, w-1-x
			case 8:
				ox, oy = y, w-1-x
			}
			output.Set(ox, oy, source.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return output
}
