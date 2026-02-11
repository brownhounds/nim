package tests

import (
	"testing"

	"github.com/brownhounds/nim"
)

func TestValidateKey(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "empty",
			key:     "",
			wantErr: true,
		},
		{
			name:    "single token",
			key:     "hello",
			wantErr: false,
		},
		{
			name:    "two parts",
			key:     "hello::resource",
			wantErr: false,
		},
		{
			name:    "four parts",
			key:     "hello::resource::whatever::2",
			wantErr: false,
		},
		{
			name:    "leading separator",
			key:     "::resource",
			wantErr: true,
		},
		{
			name:    "trailing separator",
			key:     "resource::",
			wantErr: true,
		},
		{
			name:    "double separator",
			key:     "hello::::resource",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := nim.ValidateKey(tc.key)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateKey(%q) error=%v wantErr=%v", tc.key, err, tc.wantErr)
			}
		})
	}
}
