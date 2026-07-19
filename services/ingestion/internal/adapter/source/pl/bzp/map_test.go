package bzp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/pl/bzpapi"
)

func TestMap(t *testing.T) {
	deadline := time.Date(2024, 2, 21, 8, 0, 0, 0, time.UTC)
	// publicationDate "2024-02-14T08:02:05.2588314Z" — fractional seconds preserved.
	published := time.Date(2024, 2, 14, 8, 2, 5, 258831400, time.UTC)

	tests := []struct {
		name    string
		notice  bzpapi.Notice
		want    tender.Tender
		wantDL  *time.Time
		wantPub bool // whether PublishedAt should be non-nil
	}{
		{
			name: "contract notice open, plain cpv",
			notice: bzpapi.Notice{
				ObjectID:             "08dc2d33-3bfb-c867-cf03-f600119345c3",
				NoticeType:           "ContractNotice",
				OrderObject:          "Kompleksowe sprzątanie",
				OrganizationName:     "Gmina Pyrzyce",
				CpvCode:              "45000000",
				SubmittingOffersDate: "2024-02-21T08:00:00Z",
				PublicationDate:      "2024-02-14T08:02:05.2588314Z",
			},
			want: tender.Tender{
				Source: "pl-bzp", SourceRef: "08dc2d33-3bfb-c867-cf03-f600119345c3",
				Title: "Kompleksowe sprzątanie", Buyer: tender.Buyer{Name: "Gmina Pyrzyce"},
				Status: tender.StatusOpen, Country: "PL", Language: "pl", CPV: "45000000",
			},
			wantDL:  &deadline,
			wantPub: true,
		},
		{
			name: "messy cpv list with commas inside descriptions splits primary + secondary",
			notice: bzpapi.Notice{
				ObjectID: "id-2", NoticeType: "ContractNotice",
				CpvCode: "45230000-8 (Roboty budowlane w zakresie budowy rurociągów, linii komunikacyjnych i elektroenergetycznych, autostrad, dróg, lotnisk i kolei; wyrównywanie terenu),45300000-0 (Roboty instalacyjne w budynkach)",
			},
			want: tender.Tender{
				Source: "pl-bzp", SourceRef: "id-2", Status: tender.StatusOpen,
				Country: "PL", Language: "pl",
				CPV: "45230000-8", CPVSecondary: []string{"45300000-0"},
			},
		},
		{
			name: "award notice",
			notice: bzpapi.Notice{
				ObjectID: "id-3", NoticeType: "ContractAwardNotice", CpvCode: "33690000-3",
			},
			want: tender.Tender{
				Source: "pl-bzp", SourceRef: "id-3", Status: tender.StatusAwarded,
				Country: "PL", Language: "pl", CPV: "33690000-3",
			},
		},
		{
			name: "performing notice maps to unknown, empty deadline stays nil",
			notice: bzpapi.Notice{
				ObjectID: "id-4", NoticeType: "ContractPerformingNotice",
				CpvCode: "71630000-3", SubmittingOffersDate: "",
			},
			want: tender.Tender{
				Source: "pl-bzp", SourceRef: "id-4", Status: tender.StatusUnknown,
				Country: "PL", Language: "pl", CPV: "71630000-3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Map(tt.notice, "pl-bzp")

			if got.Source != tt.want.Source {
				t.Errorf("Source = %q, want %q", got.Source, tt.want.Source)
			}
			if got.SourceRef != tt.want.SourceRef {
				t.Errorf("SourceRef = %q, want %q", got.SourceRef, tt.want.SourceRef)
			}
			if got.Title != tt.want.Title {
				t.Errorf("Title = %q, want %q", got.Title, tt.want.Title)
			}
			if got.Buyer.Name != tt.want.Buyer.Name {
				t.Errorf("Buyer.Name = %q, want %q", got.Buyer.Name, tt.want.Buyer.Name)
			}
			if got.Status != tt.want.Status {
				t.Errorf("Status = %q, want %q", got.Status, tt.want.Status)
			}
			if got.Country != "PL" || got.Language != "pl" {
				t.Errorf("locale = %q/%q, want PL/pl", got.Country, got.Language)
			}
			if got.CPV != tt.want.CPV {
				t.Errorf("CPV = %q, want %q", got.CPV, tt.want.CPV)
			}
			if !equalStrings(got.CPVSecondary, tt.want.CPVSecondary) {
				t.Errorf("CPVSecondary = %v, want %v", got.CPVSecondary, tt.want.CPVSecondary)
			}
			if got.Value != nil {
				t.Errorf("Value = %v, want nil (BZP search gateway carries no numeric value)", got.Value)
			}
			if !equalTimePtr(got.Deadline, tt.wantDL) {
				t.Errorf("Deadline = %v, want %v", got.Deadline, tt.wantDL)
			}
			if tt.wantPub {
				if got.PublishedAt == nil || !got.PublishedAt.Equal(published) {
					t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, published)
				}
			}
		})
	}
}

func TestMap_KeepsRawUntouched(t *testing.T) {
	raw := json.RawMessage(`{"objectId":"z","isTenderAmountBelowEU":true}`)
	got := Map(bzpapi.Notice{ObjectID: "z", NoticeType: "ContractNotice", Raw: raw}, "pl-bzp")
	if string(got.Raw) != string(raw) {
		t.Errorf("Raw = %s, want the untouched provider element %s", got.Raw, raw)
	}
}

func TestMap_MapsPdfDocumentWhenPresent(t *testing.T) {
	got := Map(bzpapi.Notice{ObjectID: "d", NoticeType: "ContractNotice", PdfURL: "https://ez.gov.pl/n.pdf"}, "pl-bzp")
	if len(got.Documents) != 1 || got.Documents[0].URL != "https://ez.gov.pl/n.pdf" || got.Documents[0].Type != "notice" {
		t.Errorf("Documents = %+v, want one notice document", got.Documents)
	}

	none := Map(bzpapi.Notice{ObjectID: "d", NoticeType: "ContractNotice"}, "pl-bzp")
	if none.Documents != nil {
		t.Errorf("Documents = %+v, want nil when pdfUrl is empty", none.Documents)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
