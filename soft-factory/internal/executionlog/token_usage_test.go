package executionlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTotalTokens(t *testing.T) {
	for _, tc := range []struct {
		name    string
		log     string
		want    int64
		wantErr bool
	}{
		{
			name: "sum Claude events without final newline",
			log: "Normal agent output\n" +
				`{"type":"usage","agent":"claude","usage":{"total_tokens":291343}}` + "\n" +
				`{"type":"notice","usage":{"total_tokens":999}}` + "\n" +
				`{"type":"usage","usage":{}}` + "\n" +
				`{"type":"usage","usage":{"total_tokens":null}}` + "\n" +
				`{"type":"usage","usage":{"total_tokens":` + "\n" +
				`{"type":"usage","agent":"claude","usage":{"total_tokens":135422}}`,
			want: 426765,
		},
		{
			name: "single Codex event",
			log:  `{"type":"usage","agent":"codex","usage":{"total_tokens":3203156}}` + "\nFooter\n",
			want: 3203156,
		},
		{
			name: "long output line before usage",
			log: strings.Repeat("x", 128*1024) + "\n" +
				`{"type":"usage","usage":{"total_tokens":12}}` + "\n",
			want: 12,
		},
		{name: "zero usage is available", log: `{"type":"usage","usage":{"total_tokens":0}}`},
		{name: "empty log", wantErr: true},
		{name: "missing usage", log: "Interrupted without usage\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "stage.log")
			if err := os.WriteFile(path, []byte(tc.log), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := readTotalTokens(path)
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "usage event not found") {
					t.Fatalf("readTotalTokens() error = %v, want usage event not found", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("readTotalTokens() = %d, want %d", got, tc.want)
			}
		})
	}
}
