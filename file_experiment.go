package mlflow

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type experimentMeta struct {
	ArtifactLocation string `yaml:"artifact_location"`
	ExperimentID     string `yaml:"experiment_id"`
	LifecycleStage   string `yaml:"lifecycle_stage"`
	CreationTime     int64  `yaml:"creation_time"`
	LastUpdateTime   int64  `yaml:"last_update_time"`
	Name             string `yaml:"name"`
}

type fileExperiment struct {
	experimentMeta
	rootDir string
}

func (exp *fileExperiment) ID() string {
	return exp.ExperimentID
}

func (exp *fileExperiment) CreateRun(name string) (Run, error) {
	if exp.LifecycleStage != LifecycleStageActive {
		return nil, fmt.Errorf("experiment %s is not active", exp.Name)
	}
	runID := strings.ReplaceAll(uuid.NewString(), "-", "")
	if name == "" {
		// This differs from Python client which generates a random adjective-noun-number.
		name = runID[0:8]
	}
	userName := ""
	user, err := user.Current()
	if err == nil {
		userName = user.Username
	}
	runMeta := runMeta{
		ArtifactURI:    fmt.Sprintf("%s/%s/%s", exp.ArtifactLocation, runID, artifactsFolderName),
		ExperimentID:   exp.ExperimentID,
		LifecycleStage: LifecycleStageActive,
		RunName:        name,
		StartTime:      time.Now().UnixMilli(),
		Status:         runStatusRunning,
		UserID:         userName,
		RunID:          runID,
		RunUUID:        runID,
	}
	run := &fileRun{
		runMeta: runMeta,
		rootDir: filepath.Join(exp.rootDir, runID),
	}
	for _, subDir := range []string{artifactsFolderName, metricsFolderName, paramsFolderName, tagsFolderName} {
		if err = os.MkdirAll(filepath.Join(run.rootDir, subDir), 0755); err != nil {
			return nil, err
		}
	}
	if err = run.SetTag(UserTagKey, userName); err != nil {
		return nil, err
	}

	run.syncMeta()
	return run, nil
}

func (exp *fileExperiment) GetRun(runID string) (Run, error) {
	if runID == "" {
		return nil, fmt.Errorf("runID is empty")
	}
	metaPath := filepath.Join(exp.rootDir, runID, metaDataFileName)
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	runMeta := &runMeta{}
	if err = yaml.Unmarshal(metaBytes, runMeta); err != nil {
		return nil, err
	}
	return &fileRun{
		runMeta: *runMeta,
		rootDir: filepath.Join(exp.rootDir, runID),
	}, nil
}

// Writes ExperimentMeta to disk
func (exp *fileExperiment) syncMeta() error {
	exp.experimentMeta.LastUpdateTime = time.Now().UnixMilli()
	metaBytes, err := yaml.Marshal(exp.experimentMeta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(exp.rootDir, metaDataFileName), metaBytes, 0644)
}
