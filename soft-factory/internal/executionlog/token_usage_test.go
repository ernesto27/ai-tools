package executionlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"soft-factory/internal/tokencost"

	"github.com/stretchr/testify/require"
)

func TestReadTotalTokens(t *testing.T) {
	calculator, err := tokencost.NewTokenCost()
	require.NoError(t, err)
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
		{
			name: "full shared token usage counts total once",
			log:  `{"type":"usage","agent":"codex","usage":{"input_tokens":1000,"cached_input_tokens":500,"output_tokens":100,"total_tokens":1100}}`,
			want: 1100,
		},
		{name: "null usage", log: `{"type":"usage","usage":null}`, wantErr: true},
		{name: "absent usage object", log: `{"type":"usage"}`, wantErr: true},
		{name: "empty log", wantErr: true},
		{name: "missing usage", log: "Interrupted without usage\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "stage.log")
			if err := os.WriteFile(path, []byte(tc.log), 0600); err != nil {
				t.Fatal(err)
			}
			run := &Run{stageLogPath: path, tokenCost: calculator, stageAgent: "codex", stageModel: "gpt-6.1-sol"}
			got, _, err := run.readTotalTokens()
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
