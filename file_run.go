package mlflow

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type runMeta struct {
	ArtifactURI    string `yaml:"artifact_uri"`
	ExperimentID   string `yaml:"experiment_id"`
	LifecycleStage string `yaml:"lifecycle_stage"`
	EndTime        int64  `yaml:"end_time"`
	EntryPointName string `yaml:"entry_point_name"`
	RunID          string `yaml:"run_id"`
	RunName        string `yaml:"run_name"`
	RunUUID        string `yaml:"run_uuid"`
	SourceName     string `yaml:"source_name"`
	// https://github.com/mlflow/mlflow/blob/8cd2eb0f7975decefb88af60ac5cc4f968458ab3/mlflow/protos/service.proto#L421
	SourceType    int    `yaml:"source_type"`
	SourceVersion string `yaml:"source_version"`
	StartTime     int64  `yaml:"start_time"`
	// https://github.com/mlflow/mlflow/blob/8cd2eb0f7975decefb88af60ac5cc4f968458ab3/mlflow/protos/service.proto#L439
	Status int      `yaml:"status"`
	Tags   []string `yaml:"tags"`
	UserID string   `yaml:"user_id"`
}

type fileRun struct {
	runMeta
	rootDir string
}

func (r *fileRun) ID() string {
	return r.RunID
}

func (r *fileRun) UIURL() string {
	// Assumes UI is running on default port.
	return fmt.Sprintf("http://127.0.0.1:5000/#/experiments/%s/runs/%s", r.runMeta.ExperimentID, r.RunID)
}

func (r *fileRun) syncMeta() error {
	metaBytes, err := yaml.Marshal(r.runMeta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.rootDir, metaDataFileName), metaBytes, 0644)
}

func (r *fileRun) SetTag(key, value string) error {
	return os.WriteFile(filepath.Join(r.rootDir, tagsFolderName, key), []byte(value), 0644)
}

func (r *fileRun) SetTags(tags []Tag) error {
	for _, tag := range tags {
		if err := r.SetTag(tag.Key, tag.Val); err != nil {
			return err
		}
	}
	return nil
}

func (r *fileRun) GetTag(key string) (string, error) {
	valBytes, err := os.ReadFile(filepath.Join(r.rootDir, tagsFolderName, key))
	if err != nil {
		return "", err
	}
	return string(valBytes), nil
}

func (r *fileRun) ArtifactDir() string {
	return filepath.Join(r.rootDir, artifactsFolderName)
}

func (r *fileRun) LogArtifact(localPath, artifactPath string) error {
	repo, err := NewFileArtifactRepo(r.ArtifactDir())
	if err != nil {
		return err
	}
	return repo.LogArtifact(localPath, artifactPath)
}

func (r *fileRun) LogMetric(key string, val float64, step int64) error {
	if r.LifecycleStage != LifecycleStageActive {
		return fmt.Errorf("run %s is not active", r.RunName)
	}
	if r.Status != runStatusRunning {
		return fmt.Errorf("run %s is not running", r.RunName)
	}
	path := filepath.Join(r.rootDir, metricsFolderName, key)
	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("%d %g %d\n", time.Now().UnixMilli(), val, step)
	if _, err := f.Write([]byte(line)); err != nil {
		f.Close() // ignore error; Write error takes precedence
		return err
	}
	return f.Close()
}

func (r *fileRun) LogMetrics(metrics []Metric, step int64) error {
	for _, metric := range metrics {
		if err := r.LogMetric(metric.Key, metric.Val, step); err != nil {
			return err
		}
	}
	return nil
}

func (r *fileRun) paramPath(key string) string {
	return filepath.Join(r.rootDir, paramsFolderName, key)
}

func (r *fileRun) LogParam(key, value string) error {
	return os.WriteFile(r.paramPath(key), []byte(value), 0644)
}

func (r *fileRun) LogParams(params []Param) error {
	for _, param := range params {
		if err := r.LogParam(param.Key, param.Val); err != nil {
			return err
		}
	}
	return nil
}

func (r *fileRun) End() error {
	r.EndTime = time.Now().UnixMilli()
	r.Status = runStatusFinished
	if err := r.syncMeta(); err != nil {
		return err
	}
	endIfActive(r)
	return nil
}

func (r *fileRun) Fail() error {
	r.EndTime = time.Now().UnixMilli()
	r.Status = runStatusFailed
	if err := r.syncMeta(); err != nil {
		return err
	}
	endIfActive(r)
	return nil
}

func (r *fileRun) ExperimentID() string {
	return r.runMeta.ExperimentID
}

func (r *fileRun) SetName(name string) error {
	r.RunName = name
	return r.syncMeta()
}

func (r *fileRun) Name() string {
	return r.RunName
}

func (r *fileRun) GetParam(key string) (string, error) {
	f, err := os.Open(r.paramPath(key))
	if err != nil {
		return "", fmt.Errorf("param with key %s not found", key)
	}
	defer f.Close()
	valBytes, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(valBytes), nil
}
