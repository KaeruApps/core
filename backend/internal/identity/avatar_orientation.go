package identity

import (
	"encoding/binary"
	"image"
)

// Orientation values defined by EXIF 2.3 tag 0x0112. Cameras store the frame in
// sensor orientation and record how a viewer should turn it; Go's image/jpeg
// decoder returns the stored pixels and ignores the tag, so an uploaded phone
// photo arrives rotated unless it is corrected here.
const (
	orientationNormal         = 1
	orientationFlipHorizontal = 2
	orientationRotate180      = 3
	orientationFlipVertical   = 4
	orientationTranspose      = 5
	orientationRotate90       = 6
	orientationTransverse     = 7
	orientationRotate270      = 8
)

const (
	jpegMarkerPrefix = 0xFF
	jpegMarkerAPP1   = 0xE1
	jpegMarkerSOI    = 0xD8
	jpegMarkerSOS    = 0xDA
	jpegMarkerEOI    = 0xD9

	exifOrientationTag = 0x0112
	exifTypeShort      = 3
)

// exifOrientation reports the EXIF orientation recorded in a JPEG, returning
// orientationNormal when the file carries no usable orientation. It reads only
// the orientation tag of the first IFD and treats every malformed or
// unexpected structure as "no orientation" rather than probing further.
func exifOrientation(content []byte) int {
	segment, found := findEXIFSegment(content)
	if !found {
		return orientationNormal
	}
	return orientationFromTIFF(segment)
}

// findEXIFSegment walks the JPEG marker segments and returns the TIFF payload
// of the first APP1 "Exif" segment.
func findEXIFSegment(content []byte) ([]byte, bool) {
	if len(content) < 4 || content[0] != jpegMarkerPrefix || content[1] != jpegMarkerSOI {
		return nil, false
	}

	position := 2
	for position+3 < len(content) {
		if content[position] != jpegMarkerPrefix {
			return nil, false
		}
		marker := content[position+1]
		// Padding bytes may repeat before the marker identifier.
		if marker == jpegMarkerPrefix {
			position++
			continue
		}
		// Entropy-coded image data starts here; no metadata follows.
		if marker == jpegMarkerSOS || marker == jpegMarkerEOI {
			return nil, false
		}
		length := int(binary.BigEndian.Uint16(content[position+2 : position+4]))
		if length < 2 || position+2+length > len(content) {
			return nil, false
		}
		payload := content[position+4 : position+2+length]
		if marker == jpegMarkerAPP1 {
			const header = "Exif\x00\x00"
			if len(payload) > len(header) && string(payload[:len(header)]) == header {
				return payload[len(header):], true
			}
		}
		position += 2 + length
	}

	return nil, false
}

// orientationFromTIFF reads tag 0x0112 out of the first IFD of a TIFF header.
func orientationFromTIFF(tiff []byte) int {
	if len(tiff) < 8 {
		return orientationNormal
	}

	var byteOrder binary.ByteOrder
	switch {
	case tiff[0] == 'I' && tiff[1] == 'I':
		byteOrder = binary.LittleEndian
	case tiff[0] == 'M' && tiff[1] == 'M':
		byteOrder = binary.BigEndian
	default:
		return orientationNormal
	}
	if byteOrder.Uint16(tiff[2:4]) != 0x002A {
		return orientationNormal
	}

	directoryOffset := int(byteOrder.Uint32(tiff[4:8]))
	if directoryOffset < 8 || directoryOffset+2 > len(tiff) {
		return orientationNormal
	}
	entryCount := int(byteOrder.Uint16(tiff[directoryOffset : directoryOffset+2]))
	entries := tiff[directoryOffset+2:]
	const entrySize = 12
	if entryCount < 1 || entryCount*entrySize > len(entries) {
		return orientationNormal
	}

	for index := range entryCount {
		entry := entries[index*entrySize : (index+1)*entrySize]
		if byteOrder.Uint16(entry[0:2]) != exifOrientationTag {
			continue
		}
		if byteOrder.Uint16(entry[2:4]) != exifTypeShort || byteOrder.Uint32(entry[4:8]) != 1 {
			return orientationNormal
		}
		// A SHORT fits inline, occupying the first two bytes of the value field.
		orientation := int(byteOrder.Uint16(entry[8:10]))
		if orientation < orientationNormal || orientation > orientationRotate270 {
			return orientationNormal
		}
		return orientation
	}

	return orientationNormal
}

// applyOrientation returns the image turned so that it matches what the camera
// intended, leaving the image untouched for the identity orientation.
func applyOrientation(source image.Image, orientation int) image.Image {
	if orientation == orientationNormal {
		return source
	}

	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	swapsAxes := orientation == orientationTranspose ||
		orientation == orientationRotate90 ||
		orientation == orientationTransverse ||
		orientation == orientationRotate270
	destinationWidth, destinationHeight := width, height
	if swapsAxes {
		destinationWidth, destinationHeight = height, width
	}

	destination := image.NewNRGBA(image.Rect(0, 0, destinationWidth, destinationHeight))
	for y := range height {
		for x := range width {
			var destinationX, destinationY int
			switch orientation {
			case orientationFlipHorizontal:
				destinationX, destinationY = width-1-x, y
			case orientationRotate180:
				destinationX, destinationY = width-1-x, height-1-y
			case orientationFlipVertical:
				destinationX, destinationY = x, height-1-y
			case orientationTranspose:
				destinationX, destinationY = y, x
			case orientationRotate90:
				destinationX, destinationY = height-1-y, x
			case orientationTransverse:
				destinationX, destinationY = height-1-y, width-1-x
			case orientationRotate270:
				destinationX, destinationY = y, width-1-x
			default:
				destinationX, destinationY = x, y
			}
			destination.Set(destinationX, destinationY, source.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}

	return destination
}
