package registry

import "testing"

func TestResolvedNativeAppsURL(t *testing.T) {
	service := Service{PublicURL: "https://upload.example.com"}
	if got := service.ResolvedNativeAppsURL(); got != service.PublicURL {
		t.Fatalf("expected public URL fallback, got %q", got)
	}

	service.NativeAppsURL = "https://native-upload.example.com"
	if got := service.ResolvedNativeAppsURL(); got != service.NativeAppsURL {
		t.Fatalf("expected native apps URL, got %q", got)
	}
}

func TestValidateAccessURLs(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		nativeURL string
		wantErr   bool
	}{
		{name: "public only", publicURL: "https://upload.example.com"},
		{name: "public and native", publicURL: "https://upload.example.com", nativeURL: "https://native-upload.example.com"},
		{name: "missing public", wantErr: true},
		{name: "invalid native", publicURL: "https://upload.example.com", nativeURL: "not-a-url", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAccessURLs(test.publicURL, test.nativeURL)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateAccessURLs() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
