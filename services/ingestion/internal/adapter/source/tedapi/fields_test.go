package tedapi

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/eforms"
)

// TestSearchFieldsCoverNoticeJSONTags guards the "keep these in sync" comment
// on searchFields: every eforms.Notice field decoded from the Search API
// response (i.e. every json tag other than "-") must be requested via
// searchFields, or that Notice field silently stays empty.
func TestSearchFieldsCoverNoticeJSONTags(t *testing.T) {
	requested := make(map[string]bool, len(searchFields))
	for _, f := range searchFields {
		requested[f] = true
	}

	typ := reflect.TypeOf(eforms.Notice{})
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if tag == "-" || tag == "" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if !requested[name] {
			t.Errorf("eforms.Notice field %s has json tag %q, but tedapi.searchFields does not request it", typ.Field(i).Name, name)
		}
	}
}
