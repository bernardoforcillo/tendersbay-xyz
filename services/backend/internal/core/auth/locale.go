package auth

import (
	"context"
	"strings"
)

// SupportedLocales are the 24 official EU locales the product ships, in the
// exact tags apps/platform/src/assets/locales uses. The backend's copy of this
// list exists so a stored locale can be VALIDATED rather than trusted: the
// value arrives from an Accept-Language header or a settings form, and an
// unrecognised one must become "" (unknown) rather than being persisted and
// later handed to a mail renderer that has nothing for it.
//
// It is a map rather than a slice because every use is a membership test.
var SupportedLocales = map[string]bool{
	"bg-bg": true, "cs-cz": true, "da-dk": true, "de-de": true,
	"el-gr": true, "en-ie": true, "es-es": true, "et-ee": true,
	"fi-fi": true, "fr-fr": true, "ga-ie": true, "hr-hr": true,
	"hu-hu": true, "it-it": true, "lt-lt": true, "lv-lv": true,
	"mt-mt": true, "nl-nl": true, "pl-pl": true, "pt-pt": true,
	"ro-ro": true, "sk-sk": true, "sl-si": true, "sv-se": true,
}

// defaultLinkLocale is the path segment used when we have no locale for a user
// but still have to build a URL. It is NOT a default for storage — see
// NormalizeLocale on why "" must stay representable.
const defaultLinkLocale = "en-ie"

// languageDefault maps a bare language subtag to the locale this product ships
// for it. A browser sending "it" or "it-CH" means Italian, and there is exactly
// one Italian build; refusing those would leave the commonest headers unmatched
// for the sake of a distinction the product does not make.
//
// Note es-es for "es" and pt-pt for "pt": those are the EU builds, and a
// Brazilian or Latin-American header lands on them deliberately rather than
// falling through to English.
var languageDefault = map[string]string{
	"bg": "bg-bg", "cs": "cs-cz", "da": "da-dk", "de": "de-de",
	"el": "el-gr", "en": "en-ie", "es": "es-es", "et": "et-ee",
	"fi": "fi-fi", "fr": "fr-fr", "ga": "ga-ie", "hr": "hr-hr",
	"hu": "hu-hu", "it": "it-it", "lt": "lt-lt", "lv": "lv-lv",
	"mt": "mt-mt", "nl": "nl-nl", "pl": "pl-pl", "pt": "pt-pt",
	"ro": "ro-ro", "sk": "sk-sk", "sl": "sl-si", "sv": "sv-se",
}

// NormalizeLocale turns a user-supplied or browser-supplied tag into one this
// product ships, or "" when it does not ship one.
//
// "" is a real answer and never a fallback to English. A stored locale means
// "this person told us, directly or through their browser"; English is what a
// renderer does when nobody told it. Collapsing the two would make an explicit
// English choice indistinguishable from never having been asked, and there
// would be no way to find the users still worth asking.
func NormalizeLocale(tag string) string {
	t := strings.ToLower(strings.TrimSpace(tag))
	if t == "" {
		return ""
	}
	t = strings.ReplaceAll(t, "_", "-")
	if SupportedLocales[t] {
		return t
	}
	// Fall back to the language subtag: "it-CH" and "pt-BR" are Italian and
	// Portuguese to a product that ships one build of each.
	if lang, _, found := strings.Cut(t, "-"); found {
		return languageDefault[lang]
	}
	return languageDefault[t]
}

// LocaleFromAcceptLanguage picks the best locale from an Accept-Language header.
//
// It walks the header in the order sent and returns the first tag the product
// ships, deliberately IGNORING q-weights. The header is ordered by preference
// already in every browser that sends one, and honouring q-values would mean
// preferring a 0.9 Spanish over a 0.8 Italian in a header that listed Italian
// first — a distinction no user made on purpose and one that would be invisible
// when it went wrong.
func LocaleFromAcceptLanguage(header string) string {
	for _, part := range strings.Split(header, ",") {
		tag, _, _ := strings.Cut(part, ";")
		if l := NormalizeLocale(tag); l != "" {
			return l
		}
	}
	return ""
}

// EnsureLocale stores tag for a user who has none, and does nothing otherwise.
//
// It exists to backfill: every account created before locale was persisted has
// "", and asking those people again is worse than reading what their browser
// already sends on every request. Calling it on login turns the next sign-in
// into the capture.
//
// It NEVER overwrites a locale that is already set. An Accept-Language header
// is a property of the device someone happens to be holding; a stored locale is
// a choice. Letting the header win would silently flip an Italian user's
// language because they opened the app on a work laptop configured in English —
// and they would have no idea why their reminders changed.
func (s *Service) EnsureLocale(ctx context.Context, userID, tag string) error {
	locale := NormalizeLocale(tag)
	if locale == "" {
		return nil
	}
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.Locale != "" {
		return nil
	}
	return s.users.UpdateLocale(ctx, userID, locale)
}
