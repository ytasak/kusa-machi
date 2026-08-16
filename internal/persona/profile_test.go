package persona

import (
	"errors"
	"strings"
	"testing"

	"kusamachi/internal/apperr"
)

func ptr(s string) *string { return &s }

func TestProfileNormalisation(t *testing.T) {
	tests := []struct {
		name  string
		input ProfileInput
		want  Profile
	}{
		{
			name:  "absent fields stay unset",
			input: ProfileInput{},
			want:  Profile{},
		},
		{
			name:  "surrounding whitespace is trimmed",
			input: ProfileInput{Name: ptr("  さとし  ")},
			want:  Profile{Name: ptr("さとし")},
		},
		{
			name:  "full width space only becomes unset",
			input: ProfileInput{Name: ptr("　　")},
			want:  Profile{},
		},
		{
			name:  "empty string becomes unset",
			input: ProfileInput{Bio: ptr("")},
			want:  Profile{},
		},
		{
			name:  "trailing newline is trimmed rather than rejected",
			input: ProfileInput{Hobby: ptr("散歩\n")},
			want:  Profile{Hobby: ptr("散歩")},
		},
		{
			name:  "html is kept as plain text",
			input: ProfileInput{Bio: ptr("<script>alert(1)</script>")},
			want:  Profile{Bio: ptr("<script>alert(1)</script>")},
		},
		{
			name:  "markdown is not parsed, just stored",
			input: ProfileInput{Bio: ptr("**bold**")},
			want:  Profile{Bio: ptr("**bold**")},
		},
		{
			name:  "exact length limits are accepted",
			input: ProfileInput{Name: ptr(strings.Repeat("あ", MaxNameLen)), Hobby: ptr(strings.Repeat("い", MaxHobbyLen)), Bio: ptr(strings.Repeat("う", MaxBioLen))},
			want:  Profile{Name: ptr(strings.Repeat("あ", MaxNameLen)), Hobby: ptr(strings.Repeat("い", MaxHobbyLen)), Bio: ptr(strings.Repeat("う", MaxBioLen))},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.input.Validate()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertField(t, "name", got.Name, tc.want.Name)
			assertField(t, "hobby", got.Hobby, tc.want.Hobby)
			assertField(t, "bio", got.Bio, tc.want.Bio)
		})
	}
}

func TestProfileRejections(t *testing.T) {
	tests := []struct {
		name  string
		input ProfileInput
	}{
		{"newline inside name", ProfileInput{Name: ptr("さと\nし")}},
		{"carriage return inside bio", ProfileInput{Bio: ptr("あ\rい")}},
		{"name over 20 chars", ProfileInput{Name: ptr(strings.Repeat("あ", MaxNameLen+1))}},
		{"hobby over 30 chars", ProfileInput{Hobby: ptr(strings.Repeat("あ", MaxHobbyLen+1))}},
		{"bio over 60 chars", ProfileInput{Bio: ptr(strings.Repeat("あ", MaxBioLen+1))}},
		{"http url", ProfileInput{Bio: ptr("遊びに来て http://example.com")}},
		{"https url", ProfileInput{Hobby: ptr("HTTPS://EXAMPLE.COM")}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.input.Validate(); !isInvalidProfileInput(err) {
				t.Fatalf("expected InvalidProfileInput, got %v", err)
			}
		})
	}
}

func TestProfileLengthIsCountedInRunes(t *testing.T) {
	// 20 multi-byte characters are 60 bytes but still exactly at the limit.
	in := ProfileInput{Name: ptr(strings.Repeat("あ", 20))}
	if _, err := in.Validate(); err != nil {
		t.Fatalf("20 runes should be accepted: %v", err)
	}
}

func isInvalidProfileInput(err error) bool {
	var domainErr *apperr.Error
	return errors.As(err, &domainErr) && domainErr.Code == apperr.CodeInvalidProfileInput
}

func assertField(t *testing.T, field string, got, want *string) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Fatalf("%s: got %v, want %v", field, deref(got), deref(want))
	case *got != *want:
		t.Fatalf("%s: got %q, want %q", field, *got, *want)
	}
}

func deref(s *string) string {
	if s == nil {
		return "<unset>"
	}
	return *s
}
