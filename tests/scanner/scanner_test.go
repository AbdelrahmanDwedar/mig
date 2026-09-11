package scanner_test

import (
	"strings"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/scanner"
)

func TestNew_Dispatch(t *testing.T) {
	tests := []struct {
		name       string
		driver     string
		wantErr    bool
		wantErrMsg string
	}{
		{name: "postgresql", driver: "postgresql"},
		{name: "mysql", driver: "mysql"},
		{name: "sqlite", driver: "sqlite"},
		{name: "unsupported driver name", driver: "oracle", wantErr: true, wantErrMsg: `unsupported driver: "oracle"`},
		{name: "empty driver name", driver: "", wantErr: true, wantErrMsg: `unsupported driver: ""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// New never dials the connection, so a nil *sql.DB is safe here.
			sc, err := scanner.New(tt.driver, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("got error %q, want it to contain %q", err.Error(), tt.wantErrMsg)
				}
				if sc != nil {
					t.Error("expected a nil Scanner on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sc == nil {
				t.Fatal("expected a non-nil Scanner")
			}
		})
	}
}
