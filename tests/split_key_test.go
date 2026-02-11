package tests

import (
	"reflect"
	"testing"

	"github.com/brownhounds/nim"
)

func TestSplitKey(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		want    []string
		wantErr bool
	}{
		{
			name:    "single token",
			key:     "hello",
			want:    []string{"hello"},
			wantErr: false,
		},
		{
			name:    "two parts",
			key:     "hello::resource",
			want:    []string{"hello", "resource"},
			wantErr: false,
		},
		{
			name:    "four parts",
			key:     "hello::resource::whatever::2",
			want:    []string{"hello", "resource", "whatever", "2"},
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
			got, err := nim.SplitKey(tc.key)
			if (err != nil) != tc.wantErr {
				t.Fatalf("SplitKey(%q) error=%v wantErr=%v", tc.key, err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SplitKey(%q) got=%v want=%v", tc.key, got, tc.want)
			}
		})
	}
}
