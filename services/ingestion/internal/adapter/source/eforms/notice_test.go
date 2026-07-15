package eforms

import "testing"

func TestDecode(t *testing.T) {
	data := []byte(`{
		"publication-number": "472141-2026",
		"procedure-identifier": "proc-1",
		"notice-type": "cn-standard",
		"buyer-country": ["ROU"],
		"links": {"pdf": {"RON": "https://ted.europa.eu/ro/notice/472141-2026/pdf"}}
	}`)

	n, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if n.PublicationNumber != "472141-2026" {
		t.Errorf("PublicationNumber = %q, want %q", n.PublicationNumber, "472141-2026")
	}
	if n.ProcedureIdentifier != "proc-1" {
		t.Errorf("ProcedureIdentifier = %q, want %q", n.ProcedureIdentifier, "proc-1")
	}
	if len(n.BuyerCountry) != 1 || n.BuyerCountry[0] != "ROU" {
		t.Errorf("BuyerCountry = %v, want [ROU]", n.BuyerCountry)
	}
	if n.Links.PDF["RON"] != "https://ted.europa.eu/ro/notice/472141-2026/pdf" {
		t.Errorf("Links.PDF[RON] = %q, want the RON pdf URL", n.Links.PDF["RON"])
	}
	if string(n.Raw) != string(data) {
		t.Errorf("Raw does not match the original input bytes")
	}
}

func TestDecode_InvalidJSON(t *testing.T) {
	_, err := Decode([]byte(`not json`))
	if err == nil {
		t.Fatal("Decode: want error for invalid JSON, got nil")
	}
}
