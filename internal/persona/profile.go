package persona

import (
	"strings"
	"unicode/utf8"

	"kusamachi/internal/apperr"
)

// Character limits for the user-editable "B" attributes, counted in runes.
const (
	MaxNameLen  = 20
	MaxHobbyLen = 30
	MaxBioLen   = 60
)

// Profile holds the validated, normalised "B" attributes.
// A nil field means the value is unset and must be omitted from the card.
type Profile struct {
	Name  *string
	Hobby *string
	Bio   *string
}

// ProfileInput is the raw PATCH payload. A missing or null field clears the
// value: the edit screen always submits all three fields together, so the
// update is a full replacement of the B attributes.
type ProfileInput struct {
	Name  *string `json:"name"`
	Hobby *string `json:"hobby"`
	Bio   *string `json:"bio"`
}

// Validate normalises and checks every field, returning an InvalidProfileInput
// error on the first violation. The server never trusts client-side limits.
func (in ProfileInput) Validate() (Profile, error) {
	name, err := normalizeField("name", in.Name, MaxNameLen)
	if err != nil {
		return Profile{}, err
	}
	hobby, err := normalizeField("hobby", in.Hobby, MaxHobbyLen)
	if err != nil {
		return Profile{}, err
	}
	bio, err := normalizeField("bio", in.Bio, MaxBioLen)
	if err != nil {
		return Profile{}, err
	}
	return Profile{Name: name, Hobby: hobby, Bio: bio}, nil
}

// normalizeField trims surrounding whitespace, turns a blank value into unset,
// and rejects multi-line text, over-long text and explicit URLs.
//
// Everything else is kept verbatim: the value is plain text that the frontend
// escapes on render, so HTML-looking input is simply shown as characters.
func normalizeField(field string, raw *string, maxLen int) (*string, error) {
	if raw == nil {
		return nil, nil
	}

	v := strings.TrimSpace(*raw)
	if v == "" {
		return nil, nil
	}

	if strings.ContainsAny(v, "\n\r") {
		return nil, apperr.New(apperr.CodeInvalidProfileInput, field+" must be a single line")
	}
	if utf8.RuneCountInString(v) > maxLen {
		return nil, apperr.New(apperr.CodeInvalidProfileInput, field+" is too long")
	}
	if containsURL(v) {
		return nil, apperr.New(apperr.CodeInvalidProfileInput, field+" must not contain a URL")
	}

	return &v, nil
}

// containsURL rejects only explicit http/https URLs, as the spec requires.
// No attempt is made to detect phone numbers, mail addresses or SNS handles.
func containsURL(v string) bool {
	lower := strings.ToLower(v)
	return strings.Contains(lower, "http://") || strings.Contains(lower, "https://")
}
