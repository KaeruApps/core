package identity

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

type stubUserAvatarStore struct {
	avatar UserAvatar
}

func (store *stubUserAvatarStore) GetUserAvatar(_ context.Context, _ string) (UserAvatar, error) {
	return store.avatar, nil
}

func (store *stubUserAvatarStore) UpdateUserAvatar(_ context.Context, _ string, avatar UserAvatar) error {
	store.avatar = avatar
	return nil
}

func TestUserAvatarManagerAcceptsPNG(t *testing.T) {
	store := &stubUserAvatarStore{}
	manager := NewUserAvatarManager(store)
	source := image.NewNRGBA(image.Rect(0, 0, 400, 200))
	source.Set(200, 100, color.NRGBA{R: 50, G: 150, B: 80, A: 255})
	var upload bytes.Buffer
	if err := png.Encode(&upload, source); err != nil {
		t.Fatalf("encode test image: %v", err)
	}

	avatar, err := manager.Update(context.Background(), "user-id", upload.Bytes())
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if avatar.ContentType != "image/png" {
		t.Fatalf("ContentType = %q, want image/png", avatar.ContentType)
	}
	stored, _, err := image.Decode(bytes.NewReader(avatar.Content))
	if err != nil {
		t.Fatalf("decode stored avatar: %v", err)
	}
	if stored.Bounds().Dx() != storedAvatarSize || stored.Bounds().Dy() != storedAvatarSize {
		t.Fatalf("stored avatar size = %v, want 256x256", stored.Bounds())
	}
}

func TestUserAvatarManagerRejectsUnsupportedContent(t *testing.T) {
	manager := NewUserAvatarManager(&stubUserAvatarStore{})
	if _, err := manager.Update(context.Background(), "user-id", []byte("not an image")); !errors.Is(err, ErrUserAvatarInvalid) {
		t.Fatalf("Update() error = %v, want ErrUserAvatarInvalid", err)
	}
}
