package mlflow

import (
	"log"
	"os"
	"path/filepath"
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

func ExampleActiveRunFromEnv() {
	run, err := ActiveRunFromEnv("", log.Default())
	if err != nil {
		panic(err)
	}
	for i := int64(0); i < 10; i++ {
		run.LogMetric("metric0", float64(i+1), i)
	}
	run.SetTag("tag0", "value0")
	run.LogParam("param0", "value0")

	tempDir, err := os.MkdirTemp("", "*")
	if err != nil {
		panic(err)
	}
	artifactPath := filepath.Join(tempDir, "artifact0.txt")
	if err = os.WriteFile(artifactPath, []byte("hello\n"), 0644); err != nil {
		panic(err)
	}
	if err = run.LogArtifact(artifactPath, ""); err != nil {
		panic(err)
	}

	if err = run.End(); err != nil {
		panic(err)
	}
}
