package mlflow

// When modifying please run the manual test for this file with:
// go test -v -tags manual ./...
// or
// bazel test --test_output=all //:dbfs_artifact_repo_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/Astera-org/mlflow-go/protos"
)

const (
	dbfsMaxUploadFileSize = 64 * 1024 * 1024
)

// DBFSArtifactRepo uploads to DBFS (Databricks File System).
// Generally it is used indirectly via [Run.LogArtifact].
type DBFSArtifactRepo struct {
	// Based on
	// https://github.com/mlflow/mlflow/blob/e7ff52d724e3218704fde225493e52c5acd41bb6/mlflow/store/artifact/databricks_artifact_repo.py
	rest     *RESTStore
	rootPath string
	runID    string
}

// This assumes uri is for the root of a run.
// We don't handle sub-directories in the same way the python client does.
func NewDBFSArtifactRepo(restStore *RESTStore, uri string) (ArtifactRepo, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "dbfs" {
		return nil, fmt.Errorf("expected dbfs URI scheme, got %s", parsed.Scheme)
	}
	return &DBFSArtifactRepo{restStore, parsed.Path, path.Base(path.Dir(parsed.Path))}, nil
}

// Implements [ArtifactRepo.LogArtifact].
func (repo *DBFSArtifactRepo) LogArtifact(localPath, artifactPath string) error {
	destPath := path.Join(artifactPath, path.Base(localPath))
	getCredsReq := protos.GetCredentialsForWrite{
		RunId: &repo.runID,
		Path:  []string{destPath},
	}
	var getCredsRes protos.GetCredentialsForWrite_Response
	if err := repo.rest.do(http.MethodPost, "artifacts/credentials-for-write", &getCredsReq, &getCredsRes); err != nil {
		return err
	}
	if len(getCredsRes.CredentialInfos) != 1 {
		return fmt.Errorf("expected 1 credential, got %d", len(getCredsRes.CredentialInfos))
	}
	credInfo := getCredsRes.CredentialInfos[0]

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file %q, %v", localPath, err)
	}
	// We have to read the file into memory here rather than pass the file
	// in to http.NewRequest. Otherwise it will set the Transfer-Encoding
	// header to chunked, which AWS S3 does not support.
	localBytes := make([]byte, dbfsMaxUploadFileSize)
	n, err := f.Read(localBytes)
	if n == dbfsMaxUploadFileSize {
		return fmt.Errorf("file %q is too large (>= %d bytes) to upload in a single shot. "+
			"There must be some way to handle this but we don't yet", localPath, dbfsMaxUploadFileSize)
	}
	f.Close()
	if err != nil {
		if errors.Is(err, io.EOF) {
			err = nil
		} else {
			return fmt.Errorf("failed to read file %q, %w", localPath, err)
		}
	}
	localBytes = localBytes[:n]
	httpReq, err := http.NewRequest(http.MethodPut, *credInfo.SignedUri, bytes.NewReader(localBytes))
	if err != nil {
		return err
	}
	for _, header := range credInfo.Headers {
		httpReq.Header.Add(*header.Name, *header.Value)
	}
	httpRes, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to upload artifact using signed URI %s: %v", *credInfo.SignedUri, err)
	}
	defer httpRes.Body.Close()
	resBody, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	if httpRes.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload artifact using signed URI %s with status %s: %s",
			*credInfo.SignedUri, httpRes.Status, resBody)
	}
	return nil
}

// Implements [ArtifactRepo.LogArtifacts].
func (repo *DBFSArtifactRepo) LogArtifacts(localPath, artifactPath string) error {
	// We want to keep only the last directory in the path.
	// This is because the python client does this.
	prefixToStrip := filepath.Dir(localPath) + "/"
	if prefixToStrip == "./" {
		prefixToStrip = ""
	}
	return filepath.WalkDir(localPath, func(curPath string, curEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if curEntry.IsDir() {
			return nil
		}
		// Make relative.
		destDir := filepath.Dir(curPath)[len(prefixToStrip):]
		if artifactPath != "" {
			destDir = path.Join(artifactPath, destDir)
		}
		return repo.LogArtifact(curPath, destDir)
	})
}
