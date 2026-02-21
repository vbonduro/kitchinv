package web

import (
	"testing"
)

func TestAllowedImageMIME(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		wantMIME     string
		wantDetected bool
	}{
		{
			name:         "JPEG",
			data:         []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10},
			wantMIME:     "image/jpeg",
			wantDetected: true,
		},
		{
			name:         "PNG",
			data:         []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00},
			wantMIME:     "image/png",
			wantDetected: true,
		},
		{
			name:         "GIF",
			data:         []byte("GIF89a"),
			wantMIME:     "image/gif",
			wantDetected: true,
		},
		{
			name:         "WebP",
			data:         append([]byte("RIFF\x00\x00\x00\x00WEBP"), make([]byte, 10)...),
			wantMIME:     "image/webp",
			wantDetected: true,
		},
		{
			name:         "RIFF but not WebP",
			data:         append([]byte("RIFF\x00\x00\x00\x00WAVE"), make([]byte, 10)...),
			wantMIME:     "",
			wantDetected: false,
		},
		{
			name:         "PDF disguised as image",
			data:         []byte("%PDF-1.4 malicious content"),
			wantMIME:     "",
			wantDetected: false,
		},
		{
			name:         "empty",
			data:         []byte{},
			wantMIME:     "",
			wantDetected: false,
		},
		{
			name:         "too short for WebP check",
			data:         []byte("RIFF"),
			wantMIME:     "",
			wantDetected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMIME, gotDetected := allowedImageMIME(tt.data)
			if gotDetected != tt.wantDetected {
				t.Errorf("allowedImageMIME() detected = %v, want %v", gotDetected, tt.wantDetected)
			}
			if gotMIME != tt.wantMIME {
				t.Errorf("allowedImageMIME() mimeType = %q, want %q", gotMIME, tt.wantMIME)
			}
		})
	}
}
