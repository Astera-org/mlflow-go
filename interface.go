package mlflow

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

const (
	TrackingURIEnvName  = "MLFLOW_TRACKING_URI"
	ExperimentIDEnvName = "MLFLOW_EXPERIMENT_ID"
	RunIDEnvName        = "MLFLOW_RUN_ID"
	BearerTokenEnvName  = "MLFLOW_TRACKING_TOKEN"

	// https://www.mlflow.org/docs/latest/tracking.html#system-tags
	GitCommitTagKey   = "mlflow.source.git.commit"
	ParentRunIDTagKey = "mlflow.parentRunId"
	UserTagKey        = "mlflow.user"
	SourceNameTagKey  = "mlflow.source.name"
	SourceTypeTagKey  = "mlflow.source.type"

	SourceTypeJob   = "JOB"
	SourceTypeLocal = "LOCAL"

	HostTagKey = "host"

	// https://github.com/mlflow/mlflow/blob/da4fe0f1509ff5062016b2efc05e73876db118c2/mlflow/tracking/default_experiment/__init__.py#L1
	defaultExperimentID = "0"
	// https://github.com/mlflow/mlflow/blob/da4fe0f1509ff5062016b2efc05e73876db118c2/mlflow/entities/experiment.py#L14
	defaultName = "Default"

	// These are obelisk specific, does not correspond to open source mlflow.
	tokenPath          = "mlflow-token.txt"
	defaultTrackingURI = "https://dbc-e67ede65-16bc.cloud.databricks.com"
)

var (
	ErrUnsupported = errors.New("this operation not supported by this tracking client")
)

type Tracking interface {
	ExperimentsByName() (map[string]Experiment, error)
	CreateExperiment(name string) (Experiment, error)
	GetOrCreateExperimentWithName(name string) (Experiment, error)
	GetExperiment(id string) (Experiment, error)
	URI() string
	UIURL() string
	// Returns (matching runs, next page token, error)
	SearchRuns(experimentIDs []string, filter string, orderBy []string, pageToken string) ([]Run, string, error)
}

type Experiment interface {
	CreateRun(name string) (Run, error)
	GetRun(runId string) (Run, error)
	ID() string
}

type Metric struct {
	Key string
	Val float64
}

type Param struct {
	Key string
	Val string
}

type Tag struct {
	Key string
	Val string
}

type Run interface {
	SetName(name string) error
	Name() string
	SetTag(key, value string) error
	SetTags(tags []Tag) error
	GetTag(key string) (string, error)
	LogArtifact(localPath, artifactPath string) error
	LogMetric(key string, val float64, step int64) error
	LogMetrics(metrics []Metric, step int64) error
	LogParam(key, value string) error
	LogParams(params []Param) error
	GetParam(key string) (string, error)
	End() error
	Fail() error
	UIURL() string
	ID() string
	ExperimentID() string
}

type ArtifactRepo interface {
	LogArtifact(localPath, artifactPath string) error
	LogArtifacts(localDir, artifactPath string) error
}

func NewTracking(uri, bearerToken string, l *log.Logger) (Tracking, error) {
	if uri == "" {
		uri = os.Getenv(TrackingURIEnvName)
	}
	if uri == "" {
		uri = defaultTrackingURI
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if bearerToken == "" {
		bearerToken = getToken(l)
	}
	if bearerToken == "" && uri == defaultTrackingURI {
		return nil, fmt.Errorf("no %q found, but it's required for tracking server %s", tokenPath, uri)
	}
	switch parsed.Scheme {
	case "file", "":
		if bearerToken != "" && l != nil {
			l.Println("Bearer token ignored for local file tracking URI")
		}
		return NewFileStore(parsed.Path)
	case "http", "https":
		return NewRESTStore(uri, bearerToken)
	}
	return nil, fmt.Errorf("support for tracking service with URI scheme %s not implemented", parsed.Scheme)
}

var activeRunMtx sync.Mutex
var activeRun Run = nil

func getToken(l *log.Logger) string {
	token := os.Getenv(BearerTokenEnvName)
	if token != "" {
		return token
	}
	var f *os.File
	var err error

	// Check current directory and its ancestors.
	dir := "."
	for {
		dir, err = filepath.Abs(dir)
		if err != nil {
			if l != nil {
				l.Printf("getToken() failed to get absolute path for %q: %v", dir, err)
			}
			return ""
		}
		if f, err = os.Open(filepath.Join(dir, tokenPath)); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		// Hit root of repo or file system.
		if _, err = os.Stat(filepath.Join(dir, ".git")); err == nil || parent == dir {
			break
		} else {
			dir = parent
			continue
		}
	}

	if f == nil {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			f, err = os.Open(filepath.Join(homeDir, tokenPath))
		}
		if err != nil {
			if l != nil {
				l.Printf("getToken() failed to find a file named %q in CWD, its ancestors, or home dir", tokenPath)
			}
			return ""
		}
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return ""
	}
	return scanner.Text()
}

// Returns the singleton active run. If it has not been set,
// a new run will be created in the experiment named experimentName.
// If experimentName is not set, falls back to:
// 1. The value of the MLFLOW_EXPERIMENT_ID environment variable.
// 2. The experiment with ID "0".
// This doesn't currently match the semantics of the python client.
// In particular we don't have nested runs and we don't switch to
// a new run if the active run finishes.
func ActiveRunFromEnv(experimentName string, l *log.Logger) (Run, error) {
	return getActiveRun(experimentName, l, os.Getenv)
}

func ActiveRunFromConfig(experimentName string, l *log.Logger, config interface{}) (Run, error) {
	return getActiveRun(experimentName, l, func(key string) string {
		return stringFieldFromStruct(key, config)
	})
}

func getActiveRun(experimentName string, l *log.Logger, getConfig func(string) string) (Run, error) {
	activeRunMtx.Lock()
	defer activeRunMtx.Unlock()
	if activeRun != nil && l != nil && experimentName != "" {
		l.Println("Active run already exists, ignoring experiment name")
	} else {
		tracking, err := NewTracking("", getConfig(BearerTokenEnvName), l)
		if err != nil {
			return nil, err
		}
		var exp Experiment
		expID := getConfig(ExperimentIDEnvName)
		if expID != "" {
			exp, err = tracking.GetExperiment(expID)
			if experimentName != "" && l != nil {
				l.Printf("Ignoring experiment name %q, using experiment ID %q", experimentName, expID)
			}
		} else if experimentName != "" {
			exp, err = tracking.GetOrCreateExperimentWithName(experimentName)
		} else {
			exp, err = tracking.GetExperiment("")
		}

		if err != nil {
			return nil, err
		}

		runID := getConfig(RunIDEnvName)
		if runID != "" {
			// In theory we could create the run here, but to match
			// the behavior of the Python client, we just fail.
			activeRun, err = exp.GetRun(runID)
			if err != nil {
				return nil, err
			}

		} else {
			activeRun, err = exp.CreateRun("")
			if err != nil {
				return nil, err
			}
			host, _ := os.Hostname()
			tags := []Tag{{SourceTypeTagKey, SourceTypeLocal}, {HostTagKey, host}}
			// Note: UserTagKey may noly be set during CreateRun, hence not set here.
			if err = activeRun.SetTags(tags); err != nil {
				return nil, err
			}
		}
		if l != nil {
			uri := tracking.URI()
			if strings.HasPrefix(uri, "file:") || !strings.Contains(uri, ":") {
				l.Println("MLFlow logging to local files only. To view, run: mlflow ui --backend-store-uri", uri, "--port 0")
			} else {
				l.Println("To view MLFlow, open", activeRun.UIURL())
			}
		}
	}
	return activeRun, nil
}

func endIfActive(run Run) {
	activeRunMtx.Lock()
	if activeRun == run {
		activeRun = nil
	}
	activeRunMtx.Unlock()
}

func stringFieldFromStruct(key string, config interface{}) string {
	val := reflect.ValueOf(config)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return ""
	}
	field := val.FieldByName(key)
	if field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

func LogStructAsParams(run Run, obj interface{}) error {
	objVal := reflect.ValueOf(obj)
	if objVal.Kind() == reflect.Ptr {
		objVal = objVal.Elem()
	}
	if objVal.Kind() != reflect.Struct {
		return fmt.Errorf("LogStructAsParams expected struct, got %v", objVal.Kind())
	}
	params := make([]Param, 0)
	for _, field := range reflect.VisibleFields(objVal.Type()) {
		fieldName := field.Name
		value := objVal.FieldByName(fieldName)
		if value.Kind() == reflect.Slice {
			for i := 0; i < value.Len(); i++ {
				idx := i
				params = append(params, Param{
					Key: fmt.Sprintf("%s_%d", fieldName, idx), Val: fmt.Sprintf("%v", value.Index(idx))})
			}
		} else {
			params = append(params, Param{Key: fieldName, Val: fmt.Sprintf("%v", value)})
		}
	}
	return run.LogParams(params)
}
