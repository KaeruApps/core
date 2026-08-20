package identity

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// Checks the orientation parser against files written by exiftool, which uses
// big-endian byte order and a multi-entry IFD0 that the in-memory test files
// do not exercise.
func TestEXIFOrientationMatchesExiftoolSamples(t *testing.T) {
	for expected := orientationNormal; expected <= orientationRotate270; expected++ {
		name := fmt.Sprintf("o%d.jpg", expected)
		content, err := os.ReadFile(filepath.Join("testdata", "exif-orientation", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if got := exifOrientation(content); got != expected {
			t.Errorf("exifOrientation(%s) = %d, want %d", name, got, expected)
		}

		processed, contentType, err := processUserAvatar(content)
		if err != nil {
			t.Fatalf("processUserAvatar(%s) error = %v", name, err)
		}
		if contentType != "image/jpeg" {
			t.Errorf("processUserAvatar(%s) contentType = %q, want image/jpeg", name, contentType)
		}
		decoded, err := jpeg.Decode(bytes.NewReader(processed))
		if err != nil {
			t.Fatalf("decode processed %s: %v", name, err)
		}
		if decoded.Bounds().Dx() != storedAvatarSize || decoded.Bounds().Dy() != storedAvatarSize {
			t.Errorf("processUserAvatar(%s) produced %v, want %dx%d",
				name, decoded.Bounds().Size(), storedAvatarSize, storedAvatarSize)
		}
	}
}
