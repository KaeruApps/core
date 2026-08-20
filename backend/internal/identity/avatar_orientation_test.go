package identity

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// exifJPEG builds a JPEG carrying an APP1 EXIF segment with the given
// orientation, matching the layout a phone camera produces.
func exifJPEG(t *testing.T, source image.Image, orientation uint16, bigEndian bool) []byte {
	t.Helper()

	var tiff bytes.Buffer
	var byteOrder binary.ByteOrder = binary.LittleEndian
	if bigEndian {
		byteOrder = binary.BigEndian
		tiff.WriteString("MM")
	} else {
		tiff.WriteString("II")
	}
	write16 := func(value uint16) { _ = binary.Write(&tiff, byteOrder, value) }
	write32 := func(value uint32) { _ = binary.Write(&tiff, byteOrder, value) }

	write16(0x002A)
	write32(8) // IFD0 begins immediately after the header
	write16(1) // one entry
	write16(exifOrientationTag)
	write16(exifTypeShort)
	write32(1)
	write16(orientation)
	write16(0) // padding of the 4-byte value field
	write32(0) // no next IFD

	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	segment := []byte{0xFF, jpegMarkerAPP1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:4], uint16(len(payload)+2))
	segment = append(segment, payload...)

	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	body := encoded.Bytes()
	// Splice the APP1 segment directly after the SOI marker.
	return append(append(append([]byte{}, body[:2]...), segment...), body[2:]...)
}

// markedImage is a wide image whose single red pixel identifies a corner.
func markedImage(width, height int, markX, markY int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.NRGBA{R: 20, G: 20, B: 20, A: 255})
		}
	}
	img.Set(markX, markY, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	return img
}

func TestEXIFOrientationReadsTag(t *testing.T) {
	for _, orientation := range []uint16{1, 2, 3, 4, 5, 6, 7, 8} {
		for _, bigEndian := range []bool{false, true} {
			content := exifJPEG(t, markedImage(8, 4, 0, 0), orientation, bigEndian)
			if got := exifOrientation(content); got != int(orientation) {
				t.Errorf("exifOrientation(orientation=%d bigEndian=%v) = %d, want %d",
					orientation, bigEndian, got, orientation)
			}
		}
	}
}

func TestEXIFOrientationDefaultsToNormal(t *testing.T) {
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, markedImage(4, 4, 0, 0), nil); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}

	cases := map[string][]byte{
		"no EXIF segment":  plain.Bytes(),
		"empty":            {},
		"not a JPEG":       []byte("this is not an image at all"),
		"truncated SOI":    {0xFF},
		"truncated APP1":   {0xFF, 0xD8, 0xFF, 0xE1, 0x00},
		"oversized length": {0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF, 'E', 'x'},
		"bad TIFF order":   exifJPEGWithTIFF(t, []byte("XX\x2A\x00\x08\x00\x00\x00")),
		"offset past end":  exifJPEGWithTIFF(t, []byte("II\x2A\x00\xFF\xFF\xFF\xFF")),
		"short TIFF":       exifJPEGWithTIFF(t, []byte("II")),
	}
	for name, content := range cases {
		if got := exifOrientation(content); got != orientationNormal {
			t.Errorf("exifOrientation(%s) = %d, want %d", name, got, orientationNormal)
		}
	}
}

func exifJPEGWithTIFF(t *testing.T, tiff []byte) []byte {
	t.Helper()
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, jpegMarkerAPP1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:4], uint16(len(payload)+2))
	segment = append(segment, payload...)

	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, markedImage(4, 4, 0, 0), nil); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	body := encoded.Bytes()
	return append(append(append([]byte{}, body[:2]...), segment...), body[2:]...)
}

func TestApplyOrientationMovesMarkedCorner(t *testing.T) {
	// A 4x2 image with the mark in the top-left, and where that mark must end
	// up once each orientation is corrected.
	source := markedImage(4, 2, 0, 0)
	cases := []struct {
		orientation   int
		width, height int
		wantX, wantY  int
	}{
		{orientationNormal, 4, 2, 0, 0},
		{orientationFlipHorizontal, 4, 2, 3, 0},
		{orientationRotate180, 4, 2, 3, 1},
		{orientationFlipVertical, 4, 2, 0, 1},
		{orientationTranspose, 2, 4, 0, 0},
		{orientationRotate90, 2, 4, 1, 0},
		{orientationTransverse, 2, 4, 1, 3},
		{orientationRotate270, 2, 4, 0, 3},
	}
	for _, testCase := range cases {
		rotated := applyOrientation(source, testCase.orientation)
		bounds := rotated.Bounds()
		if bounds.Dx() != testCase.width || bounds.Dy() != testCase.height {
			t.Errorf("orientation %d produced %dx%d, want %dx%d",
				testCase.orientation, bounds.Dx(), bounds.Dy(), testCase.width, testCase.height)
			continue
		}
		red, _, _, _ := rotated.At(testCase.wantX, testCase.wantY).RGBA()
		if red < 0x8000 {
			t.Errorf("orientation %d: marked pixel is not at (%d,%d)",
				testCase.orientation, testCase.wantX, testCase.wantY)
		}
	}
}

func TestProcessUserAvatarUprightsRotatedPhoto(t *testing.T) {
	// A portrait photo stored landscape with orientation 6 (rotate 90 CW): the
	// subject marker sits bottom-left in storage and must end up top-left.
	source := markedImage(64, 32, 1, 30)
	content := exifJPEG(t, source, orientationRotate90, false)

	processed, contentType, err := processUserAvatar(content)
	if err != nil {
		t.Fatalf("processUserAvatar() error = %v", err)
	}
	if contentType != "image/jpeg" {
		t.Fatalf("contentType = %q, want image/jpeg", contentType)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(processed))
	if err != nil {
		t.Fatalf("jpeg.Decode() error = %v", err)
	}
	if decoded.Bounds().Dx() != storedAvatarSize || decoded.Bounds().Dy() != storedAvatarSize {
		t.Fatalf("stored avatar is %v, want %dx%d square",
			decoded.Bounds().Size(), storedAvatarSize, storedAvatarSize)
	}

	// Rotating 64x32 gives a 32x64 portrait, so the square crop is taken from
	// the middle rows rather than the full frame the unrotated image would use.
	upright := applyOrientation(source, orientationRotate90)
	if upright.Bounds().Dx() != 32 || upright.Bounds().Dy() != 64 {
		t.Fatalf("upright image is %v, want 32x64", upright.Bounds().Size())
	}
}

func TestProcessUserAvatarLeavesUnorientedImageAlone(t *testing.T) {
	source := markedImage(64, 64, 0, 0)
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, source, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}

	processed, _, err := processUserAvatar(plain.Bytes())
	if err != nil {
		t.Fatalf("processUserAvatar() error = %v", err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(processed))
	if err != nil {
		t.Fatalf("jpeg.Decode() error = %v", err)
	}
	// The mark stays in the top-left corner when no rotation is recorded.
	red, green, _, _ := decoded.At(2, 2).RGBA()
	if red <= green {
		t.Errorf("top-left corner is not the marked pixel; red=%d green=%d", red, green)
	}
}

// withOrientation splices an APP1 EXIF segment carrying the orientation into an
// existing JPEG, the way a camera would.
func withOrientation(t *testing.T, body []byte, orientation int) []byte {
	t.Helper()
	var tiff bytes.Buffer
	tiff.WriteString("II")
	write16 := func(value uint16) { _ = binary.Write(&tiff, binary.LittleEndian, value) }
	write32 := func(value uint32) { _ = binary.Write(&tiff, binary.LittleEndian, value) }
	write16(0x002A)
	write32(8)
	write16(1)
	write16(exifOrientationTag)
	write16(exifTypeShort)
	write32(1)
	write16(uint16(orientation))
	write16(0)
	write32(0)

	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	segment := []byte{0xFF, jpegMarkerAPP1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:4], uint16(len(payload)+2))
	segment = append(segment, payload...)
	return append(append(append([]byte{}, body[:2]...), segment...), body[2:]...)
}
