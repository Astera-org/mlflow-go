package mlflow

import (
	"log"
	"os"
	"testing"
)

func TestActiveRunFromEnv(t *testing.T) {
	if err := os.Setenv(TrackingURIEnvName, "file://"+t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv(TrackingURIEnvName)
	run, err := ActiveRunFromEnv(t.Name(), log.Default())
	if err != nil {
		t.Fatal(err)
	}
	if run != activeRun {
		t.Fatal("expected active run to be set")
	}
	if err = run.End(); err != nil {
		t.Fatal(err)
	}
	if activeRun != nil {
		t.Fatal("expected active run to be nil after ending the run")
	}
}
