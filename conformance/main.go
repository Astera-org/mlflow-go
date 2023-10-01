package main

import (
	"log"
	"os"
	"path/filepath"

	mlflow "github.com/Astera-org/mlflow-go"
)

func main() {
	run, err := mlflow.ActiveRunFromEnv("", log.Default())
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
