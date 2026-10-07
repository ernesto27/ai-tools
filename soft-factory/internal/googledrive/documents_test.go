package googledrive

import "testing"

func TestDocumentIDFromURL(t *testing.T) {
	const id = "1LeptkP2vylqYTNJT0OPl2gx8qxEpoi3N9IxNVwX4Dno"
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"tab URL", "https://docs.google.com/document/d/" + id + "/edit?tab=t.0", id},
		{"fragment", "https://docs.google.com/document/d/abc-_123/edit#heading=h.test", "abc-_123"},
		{"bare document URL", "https://docs.google.com/document/d/" + id, id},
		{"surrounding whitespace", " https://docs.google.com/document/d/" + id + "/edit\n", id},
		{"empty", "", ""},
		{"malformed URL", "https://docs.google.com/document/d/%zz", ""},
		{"wrong host", "https://example.com/document/d/abc/edit", ""},
		{"host suffix", "https://docs.google.com.example.com/document/d/abc/edit", ""},
		{"wrong scheme", "http://docs.google.com/document/d/abc/edit", ""},
		{"other file type", "https://docs.google.com/spreadsheets/d/abc/edit", ""},
		{"missing ID", "https://docs.google.com/document/d//edit", ""},
		{"invalid ID", "https://docs.google.com/document/d/abc%20def/edit", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DocumentIDFromURL(tt.url)
			if tt.want == "" {
				if err == nil || got != "" {
					t.Fatalf("got (%q, %v), want empty ID and error", got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got (%q, %v), want (%q, nil)", got, err, tt.want)
			}
		})
	}
}
