package identity

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"time"

	"golang.org/x/image/draw"
)

const (
	MaxUserAvatarUploadBytes = 5 * 1024 * 1024
	maxStoredAvatarBytes     = 1024 * 1024
	maxUserAvatarPixels      = 25_000_000
	storedAvatarSize         = 256
)

var (
	ErrUserAvatarNotFound = errors.New("user avatar not found")
	ErrUserAvatarInvalid  = errors.New("user avatar must be a valid JPG or PNG file no larger than 5 MB")
)

type UserAvatar struct {
	Content     []byte
	ContentType string
	UpdatedAt   time.Time
}

type UserAvatarStore interface {
	GetUserAvatar(context.Context, string) (UserAvatar, error)
	UpdateUserAvatar(context.Context, string, UserAvatar) error
}

type UserAvatarManager struct {
	store UserAvatarStore
	now   func() time.Time
}

func NewUserAvatarManager(store UserAvatarStore) *UserAvatarManager {
	return &UserAvatarManager{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (manager *UserAvatarManager) Get(ctx context.Context, userID string) (UserAvatar, error) {
	return manager.store.GetUserAvatar(ctx, userID)
}

func (manager *UserAvatarManager) Update(ctx context.Context, userID string, content []byte) (UserAvatar, error) {
	processed, contentType, err := processUserAvatar(content)
	if err != nil {
		return UserAvatar{}, ErrUserAvatarInvalid
	}
	avatar := UserAvatar{Content: processed, ContentType: contentType, UpdatedAt: manager.now()}
	if err := manager.store.UpdateUserAvatar(ctx, userID, avatar); err != nil {
		return UserAvatar{}, err
	}
	return avatar, nil
}

func processUserAvatar(content []byte) ([]byte, string, error) {
	if len(content) == 0 || len(content) > MaxUserAvatarUploadBytes {
		return nil, "", ErrUserAvatarInvalid
	}
	configuration, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || (format != "jpeg" && format != "png") {
		return nil, "", ErrUserAvatarInvalid
	}
	pixelCount := int64(configuration.Width) * int64(configuration.Height)
	if configuration.Width < 1 || configuration.Height < 1 || pixelCount > maxUserAvatarPixels {
		return nil, "", ErrUserAvatarInvalid
	}

	source, decodedFormat, err := image.Decode(bytes.NewReader(content))
	if err != nil || decodedFormat != format {
		return nil, "", ErrUserAvatarInvalid
	}
	// Phone cameras record the frame in sensor orientation and note the
	// required rotation in EXIF. Correct it before cropping so the crop is
	// taken from the upright image rather than the stored one.
	if format == "jpeg" {
		source = applyOrientation(source, exifOrientation(content))
	}
	bounds := source.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	crop := image.Rect(
		bounds.Min.X+(bounds.Dx()-side)/2,
		bounds.Min.Y+(bounds.Dy()-side)/2,
		bounds.Min.X+(bounds.Dx()-side)/2+side,
		bounds.Min.Y+(bounds.Dy()-side)/2+side,
	)
	destination := image.NewNRGBA(image.Rect(0, 0, storedAvatarSize, storedAvatarSize))
	draw.CatmullRom.Scale(destination, destination.Bounds(), source, crop, draw.Src, nil)

	var encoded bytes.Buffer
	contentType := "image/png"
	if format == "jpeg" {
		contentType = "image/jpeg"
		err = jpeg.Encode(&encoded, destination, &jpeg.Options{Quality: 85})
	} else {
		err = png.Encode(&encoded, destination)
	}
	if err != nil || encoded.Len() == 0 || encoded.Len() > maxStoredAvatarBytes {
		return nil, "", ErrUserAvatarInvalid
	}
	return encoded.Bytes(), contentType, nil
}
