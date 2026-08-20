package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNormalizeLocale(t *testing.T) {
	tests := []struct{ in, want string }{
		{"it-it", "it-it"},
		{"IT-IT", "it-it"},
		{"it_IT", "it-it"},  // underscore form, seen from some clients
		{"it", "it-it"},     // bare language
		{"it-CH", "it-it"},  // Italian in Switzerland is still Italian here
		{"pt-BR", "pt-pt"},  // one Portuguese build, and it is the EU one
		{"es-419", "es-es"}, // Latin-American Spanish lands on the EU build
		{"en", "en-ie"},     // the English this product ships is Irish
		{"  fr-FR  ", "fr-fr"},
		{"", ""},
		{"zz", ""}, // not shipped
		{"klingon", ""},
	}
	for _, tt := range tests {
		if got := NormalizeLocale(tt.in); got != tt.want {
			t.Errorf("NormalizeLocale(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestNormalizeLocaleNeverInventsEnglish pins the distinction the whole design
// rests on: "" means nobody told us, and a renderer choosing English is a
// separate decision. If this ever returns en-ie for an unknown tag, there is no
// longer any way to find the users still worth asking.
func TestNormalizeLocaleNeverInventsEnglish(t *testing.T) {
	for _, in := range []string{"", "zz", "xx-YY", "   "} {
		if got := NormalizeLocale(in); got != "" {
			t.Errorf("NormalizeLocale(%q) = %q, want empty", in, got)
		}
	}
}

func TestLocaleFromAcceptLanguage(t *testing.T) {
	tests := []struct{ in, want string }{
		{"it-IT,it;q=0.9,en-US;q=0.8,en;q=0.7", "it-it"},
		{"en-US,en;q=0.9", "en-ie"},
		// Order wins over q-weight on purpose: the header already lists the
		// user's preference first, and preferring a higher-weighted later entry
		// would be a choice they never made.
		{"it;q=0.8,es;q=0.9", "it-it"},
		{"zz,xx,de-DE", "de-de"}, // skips what we do not ship
		{"", ""},
		{"zz,xx", ""},
	}
	for _, tt := range tests {
		if got := LocaleFromAcceptLanguage(tt.in); got != tt.want {
			t.Errorf("LocaleFromAcceptLanguage(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestSupportedLocalesMatchesTheShippedSet guards the one way this list rots:
// a locale added to apps/platform/src/assets/locales and not here would be
// selectable in the UI and rejected on save.
func TestSupportedLocalesMatchesTheShippedSet(t *testing.T) {
	if len(SupportedLocales) != 24 {
		t.Errorf("SupportedLocales has %d entries, want the 24 official EU locales", len(SupportedLocales))
	}
	if len(languageDefault) != len(SupportedLocales) {
		t.Errorf("languageDefault has %d entries and SupportedLocales %d — every shipped locale needs a bare-language route in",
			len(languageDefault), len(SupportedLocales))
	}
	for lang, loc := range languageDefault {
		if !SupportedLocales[loc] {
			t.Errorf("languageDefault[%q] = %q, which is not a shipped locale", lang, loc)
		}
	}
}

type localeStore struct {
	user    User
	written []string
	findErr error
}

func (m *localeStore) Create(context.Context, User) (User, error)        { return User{}, nil }
func (m *localeStore) FindByEmail(context.Context, string) (User, error) { return User{}, nil }
func (m *localeStore) FindByID(context.Context, string) (User, error) {
	return m.user, m.findErr
}
func (m *localeStore) UpdatePassword(context.Context, string, string) error    { return nil }
func (m *localeStore) UpdateEmail(context.Context, string, string) error       { return nil }
func (m *localeStore) UpdateDisplayName(context.Context, string, string) error { return nil }
func (m *localeStore) UpdateLocale(_ context.Context, _, locale string) error {
	m.written = append(m.written, locale)
	return nil
}
func (m *localeStore) MarkEmailVerified(context.Context, string, time.Time) error { return nil }
func (m *localeStore) Delete(context.Context, string) error                       { return nil }

// TestEnsureLocaleNeverOverwritesAChoice is the guard that matters. An
// Accept-Language header describes the DEVICE someone is holding; a stored
// locale is a CHOICE. Letting the header win would flip an Italian user's
// language because they opened the app on a work laptop set to English, and
// they would have no idea why their reminders changed.
func TestEnsureLocaleNeverOverwritesAChoice(t *testing.T) {
	store := &localeStore{user: User{ID: "u1", Locale: "it-it"}}
	s := &Service{users: store}
	if err := s.EnsureLocale(context.Background(), "u1", "en-US,en;q=0.9"); err != nil {
		t.Fatalf("EnsureLocale: %v", err)
	}
	if len(store.written) != 0 {
		t.Errorf("overwrote a stored locale with %v", store.written)
	}
}

func TestEnsureLocaleBackfillsWhenEmpty(t *testing.T) {
	store := &localeStore{user: User{ID: "u1"}}
	s := &Service{users: store}
	if err := s.EnsureLocale(context.Background(), "u1", "it-IT,it;q=0.9"); err != nil {
		t.Fatalf("EnsureLocale: %v", err)
	}
	if len(store.written) != 1 || store.written[0] != "it-it" {
		t.Errorf("wrote %v, want [it-it]", store.written)
	}
}

// TestEnsureLocaleIgnoresUnusableHeaders: a header we ship nothing for must not
// even cost a lookup, and must never store a tag no renderer has copy for.
func TestEnsureLocaleIgnoresUnusableHeaders(t *testing.T) {
	store := &localeStore{user: User{ID: "u1"}, findErr: errors.New("should not be called")}
	s := &Service{users: store}
	for _, h := range []string{"", "zz,xx", "   "} {
		if err := s.EnsureLocale(context.Background(), "u1", h); err != nil {
			t.Errorf("EnsureLocale(%q) = %v, want nil without touching the store", h, err)
		}
	}
	if len(store.written) != 0 {
		t.Errorf("stored %v from an unusable header", store.written)
	}
}
