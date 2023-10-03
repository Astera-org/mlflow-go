package mlflow

import (
	"os"
	"os/exec"
	"path/filepath"
)

// FileArtifactRepo writes to a local file system.
// Generally it is used indirectly via [Run.LogArtifact].
type FileArtifactRepo struct {
	rootDir string
}

func NewFileArtifactRepo(rootDir string) (ArtifactRepo, error) {
	return &FileArtifactRepo{rootDir: rootDir}, nil
}

func (repo *FileArtifactRepo) LogArtifact(localPath, artifactPath string) error {
	if artifactPath == "" {
		artifactPath = filepath.Base(localPath)
	}
	err := os.Link(localPath, filepath.Join(repo.rootDir, artifactPath))
	if err != nil {
		err = exec.Command("cp", "-r", localPath, filepath.Join(repo.rootDir, artifactPath)).Run()
	}
	return err
}

func (repo *FileArtifactRepo) LogArtifacts(localPath, artifactPath string) error {
	return repo.LogArtifact(localPath, artifactPath)
}
