package mlflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// https://github.com/mlflow/mlflow/blob/8cd2eb0f7975decefb88af60ac5cc4f968458ab3/mlflow/store/tracking/file_store.py#L132
	trashFolderName     = ".trash"
	artifactsFolderName = "artifacts"
	metricsFolderName   = "metrics"
	paramsFolderName    = "params"
	tagsFolderName      = "tags"
	metaDataFileName    = "meta.yaml"

	// https://github.com/mlflow/mlflow/blob/8cd2eb0f7975decefb88af60ac5cc4f968458ab3/mlflow/entities/lifecycle_stage.py#L5
	LifecycleStageActive  = "active"
	LifecycleStageDeleted = "deleted"

	// https://github.com/mlflow/mlflow/blob/8cd2eb0f7975decefb88af60ac5cc4f968458ab3/mlflow/protos/service.proto#L439
	runStatusRunning   = 1
	runStatusScheduled = 2
	runStatusFinished  = 3
	runStatusFailed    = 4
	runStatusKilled    = 5
)

// Implements Tracking interface
type FileStore struct {
	rootDir string
}

func NewFileStore(rootDir string) (*FileStore, error) {
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("mlflow.NewFileStore: error getting absolute path: %w", err)
	}
	// Match the Python behavior of creating the default experiment.
	fs := &FileStore{rootDir: rootDir}
	if exp, _ := fs.GetExperiment(defaultExperimentID); exp == nil {
		if _, err := fs.createExperiment(defaultName, defaultExperimentID); err != nil {
			return nil, err
		}
	}
	return fs, nil
}

func (s *FileStore) URI() string {
	return s.rootDir
}

func (f *FileStore) ExperimentsByName() (map[string]Experiment, error) {
	files, err := os.ReadDir(f.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("mlflow.FileStore.ExperimentsByName: error reading directory: %w", err)
	}
	experiments := make(map[string]Experiment)
	for _, file := range files {
		metaPath := filepath.Join(f.rootDir, file.Name(), metaDataFileName)
		_, err := os.Stat(metaPath)
		if err != nil && os.IsNotExist(err) {
			// Ignore
			continue
		}
		metaBytes, err := os.ReadFile(metaPath)
		if err != nil {
			return nil, fmt.Errorf("mlflow.FileStore.ExperimentsByName: error reading file: %w", err)
		}
		exp := &fileExperiment{
			rootDir: filepath.Join(f.rootDir, file.Name()),
		}
		if err = yaml.Unmarshal(metaBytes, &exp.experimentMeta); err != nil {
			return nil, err
		}
		experiments[exp.Name] = exp
	}
	return experiments, nil
}

// Gets or creates an experiment and returns it.
func (fs *FileStore) GetOrCreateExperimentWithName(name string) (Experiment, error) {
	if name == "" {
		name = defaultName
	}
	expsByName, err := fs.ExperimentsByName()
	if err != nil {
		return nil, err
	}
	if exp, ok := expsByName[name]; ok {
		return exp, nil
	}
	// Pick the ID here to avoid extra file I/O in createExperiment.
	highestID := -1
	for _, exp := range expsByName {
		idInt, err := strconv.Atoi(exp.(*fileExperiment).ExperimentID)
		if err == nil && idInt > highestID {
			highestID = idInt
		}
	}
	return fs.createExperiment(name, strconv.Itoa(highestID+1))
}

func (fs *FileStore) GetExperiment(id string) (Experiment, error) {
	if id == "" {
		id = defaultExperimentID
	}
	expsByName, err := fs.ExperimentsByName()
	if err != nil {
		return nil, err
	}
	for _, exp := range expsByName {
		if exp.(*fileExperiment).ExperimentID == id {
			return exp, nil
		}
	}
	return nil, fmt.Errorf("no experiment with id %s", id)
}

func ToURI(path string) string {
	pathGeneric := filepath.ToSlash(path)
	// Windows paths don't necessarily start with /
	if pathGeneric[0] == '/' {
		return "file://" + pathGeneric
	} else {
		return "file:///" + pathGeneric
	}
}

func (fs *FileStore) CreateExperiment(name string) (Experiment, error) {
	return fs.createExperiment(name, "")
}

func (fs *FileStore) createExperiment(name, id string) (Experiment, error) {
	if id == "" {
		highestID := -1
		files, err := os.ReadDir(fs.rootDir)
		if err != nil {
			return nil, fmt.Errorf("mlflow.FileStore.createExperiment: error reading dir: %w", err)
		}
		for _, file := range files {
			idInt, err := strconv.Atoi(file.Name())
			if err == nil && idInt > highestID {
				highestID = idInt
			}
		}
		id = strconv.Itoa(highestID + 1)
	}
	experimentPath := filepath.Join(fs.rootDir, id)
	if err := os.MkdirAll(experimentPath, 0755); err != nil {
		return nil, fmt.Errorf("mlflow.FileStore.createExperiment: error creating experiment dir: %w", err)
	}
	now := time.Now().UnixMilli()
	if name == "" {
		name = "Default"
	}
	meta := experimentMeta{
		ExperimentID:   id,
		LifecycleStage: LifecycleStageActive,
		CreationTime:   now,
		LastUpdateTime: now,
		Name:           name,
	}
	meta.ArtifactLocation = ToURI(experimentPath)
	exp := &fileExperiment{
		experimentMeta: meta,
		rootDir:        experimentPath,
	}
	exp.syncMeta()
	return exp, nil
}

// Very limited filter support just for testing the one use case we have.
func newRunFilter(filter string) (func(Run) bool, error) {
	if filter == "" {
		return func(Run) bool { return true }, nil
	}
	regexp := regexp.MustCompile(`tags\.` + "`?" + `(.+?)` + "`?" + `\s*=\s*'([^']*)'`)
	matches := regexp.FindStringSubmatch(filter)
	if len(matches) != 3 {
		return nil, fmt.Errorf("mlflow.FileStore currently only supports filtering on a single tag. Got: %s", filter)
	}
	tagName := matches[1]
	tagValue := matches[2]
	return func(run Run) bool {
		value, err := run.GetTag(tagName)
		return err == nil && value == tagValue
	}, nil
}

func (fs *FileStore) SearchRuns(experimentIDs []string, filter string, orderBy []string, pageToken string) ([]Run, string, error) {
	filterFunc, err := newRunFilter(filter)
	if err != nil {
		return nil, "", err
	}
	experiments := make([]*fileExperiment, len(experimentIDs))
	for i, id := range experimentIDs {
		if id == "" {
			return nil, "", fmt.Errorf("SearchRuns: empty experiment ID is not valid")
		}
		exp, err := fs.GetExperiment(id)
		if err != nil {
			return nil, "", err
		}
		experiments[i] = exp.(*fileExperiment)
	}
	runs := make([]Run, 0)
	for _, exp := range experiments {
		files, err := os.ReadDir(exp.rootDir)
		if err != nil {
			return nil, "", fmt.Errorf("mlflow.FileStore.SearchRuns: error reading dir: %w", err)
		}
		for _, file := range files {
			if !file.IsDir() {
				continue
			}
			run, err := exp.GetRun(file.Name())
			if err != nil {
				return nil, "", err
			}
			if filterFunc(run) {
				runs = append(runs, run)
			}
		}
	}
	return runs, "", nil
}

func (fs *FileStore) UIURL() string {
	// Assumes UI is running on default port.
	return "http://127.0.0.1:5000/#"
}
