//go:build manual

// Manual because it assumes it can connect to an MLFlow server.
// To run this test, you need to set the following environment variables:
// MLFLOW_TRACKING_URI: the URI of the MLFlow server.

package mlflow

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestLogArtifact(t *testing.T) {
	run, err := ActiveRunFromEnv("/Shared/test", log.Default())
	if err != nil {
		t.Fatal(err.Error())
	}
	fmt.Println("Run ID", run.ID())
	artifactDir := t.TempDir()
	f, err := os.OpenFile(filepath.Join(artifactDir, "a.txt"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err.Error())
	}
	if _, err = f.WriteString("Hello, world!"); err != nil {
		t.Fatal(err.Error())
	}
	if err = f.Close(); err != nil {
		t.Fatal(err.Error())
	}
	if err = os.Mkdir(filepath.Join(artifactDir, "subdir"), 0755); err != nil {
		t.Fatal(err.Error())
	}
	f, err = os.OpenFile(filepath.Join(artifactDir, "subdir", "b.txt"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err.Error())
	}
	if _, err = f.WriteString("Hello, world!"); err != nil {
		t.Fatal(err.Error())
	}
	if err = f.Close(); err != nil {
		t.Fatal(err.Error())
	}
	f, err = os.OpenFile(filepath.Join(artifactDir, "empty.txt"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err.Error())
	}
	if err = f.Close(); err != nil {
		t.Fatal(err.Error())
	}
	if err = run.LogArtifact(artifactDir, ""); err != nil {
		t.Fatal(err.Error())
	}

	// Now relative path
	if err = os.Chdir(artifactDir); err != nil {
		t.Fatal(err.Error())
	}
	if err = run.LogArtifact("subdir", ""); err != nil {
		t.Fatal(err.Error())
	}

	fmt.Println("Check the artifacts at ", run.UIURL())
	fmt.Printf("They should contain %s and subdir", filepath.Base(artifactDir))
}
