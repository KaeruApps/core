package identity

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"
)

// Simulates a portrait phone photo. The camera stores the frame landscape with
// a dark-to-bright gradient along the stored X axis and records orientation 6.
// Correcting that orientation turns the gradient vertical, so a correctly
// processed avatar is dark at the top and bright at the bottom. Left untouched
// it would stay horizontal, which is exactly the sideways avatar users saw.
func TestPhonePortraitPhotoEndsUpUpright(t *testing.T) {
	const width, height = 96, 48
	stored := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			level := uint8(20 + (x*215)/(width-1))
			stored.Set(x, y, color.NRGBA{R: level, G: level, B: level, A: 255})
		}
	}

	var body bytes.Buffer
	if err := jpeg.Encode(&body, stored, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}

	processed, _, err := processUserAvatar(withOrientation(t, body.Bytes(), orientationRotate90))
	if err != nil {
		t.Fatalf("processUserAvatar() error = %v", err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(processed))
	if err != nil {
		t.Fatal(err)
	}

	brightness := func(x, y int) int {
		r, g, b, _ := decoded.At(x, y).RGBA()
		return int((r + g + b) / 3 >> 8)
	}
	top, bottom := brightness(128, 8), brightness(128, storedAvatarSize-9)
	left, right := brightness(8, 128), brightness(storedAvatarSize-9, 128)
	t.Logf("top=%d bottom=%d left=%d right=%d", top, bottom, left, right)

	if bottom-top < 40 {
		t.Errorf("gradient did not become vertical: top=%d bottom=%d", top, bottom)
	}
	if abs(right-left) > 12 {
		t.Errorf("gradient is still horizontal, avatar is sideways: left=%d right=%d", left, right)
	}
}

// Confirms the same photo is stored sideways before correction, so the test
// above is measuring the fix rather than an image that was already upright.
func TestStoredPhonePhotoIsSidewaysBeforeCorrection(t *testing.T) {
	content, err := os.ReadFile("testdata/exif-orientation/o6.jpg")
	if err != nil {
		t.Fatal(err)
	}
	source, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if source.Bounds().Dx() <= source.Bounds().Dy() {
		t.Fatalf("sample should be stored landscape, got %v", source.Bounds().Size())
	}
	upright := applyOrientation(source, exifOrientation(content))
	if upright.Bounds().Dx() >= upright.Bounds().Dy() {
		t.Errorf("corrected image should be portrait, got %v", upright.Bounds().Size())
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
