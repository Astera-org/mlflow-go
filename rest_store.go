package mlflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/Astera-org/mlflow-go/protos"
	"github.com/google/uuid"
)

// Implements Tracking interface
// See https://www.mlflow.org/docs/latest/rest-api.html
// for the REST API documentation.
type RESTStore struct {
	baseURL     string
	bearerToken string
}

func NewRESTStore(baseURL, bearerToken string) (Tracking, error) {
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	return &RESTStore{baseURL: baseURL, bearerToken: bearerToken}, nil
}

func (rs *RESTStore) do(method, path string, req, res interface{}) error {
	if method == http.MethodGet && req != nil {
		return fmt.Errorf("GET requests cannot have a body")
	}
	url := rs.baseURL + "/api/2.0/mlflow/" + path
	var reqBody io.Reader
	if req != nil {
		reqJSON, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshall request to JSON: %v", err)
		}
		reqBody = bytes.NewReader(reqJSON)
	}

	httpReq, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if rs.bearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+rs.bearerToken)
	}

	httpRes, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to %s: %v", method, err)
	}
	defer httpRes.Body.Close()
	resBody, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	if httpRes.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s failed with status %s: %s", method, url, httpRes.Status, resBody)
	}
	if err = json.Unmarshal(resBody, res); err != nil {
		return fmt.Errorf("failed to unmarshall response body: %s\n%v", resBody, err)
	}
	return nil
}

func (rs *RESTStore) URI() string {
	return rs.baseURL
}

func (rs *RESTStore) UIURL() string {
	middle := ""
	if strings.Contains(rs.baseURL, "databricks.com") {
		middle = "#mlflow/"
	}
	return fmt.Sprintf("%s/%s", rs.baseURL, middle)
}

func (rs *RESTStore) CreateExperiment(name string) (Experiment, error) {
	var resp protos.CreateExperiment_Response
	err := rs.do(http.MethodPost,
		"experiments/create",
		protos.CreateExperiment{Name: &name},
		&resp)
	if err != nil {
		return nil, err
	}
	return &restExperiment{rs, *resp.ExperimentId}, nil
}

func (rs *RESTStore) ExperimentsByName() (map[string]Experiment, error) {
	var resp protos.SearchExperiments_Response
	maxResults := int64(1000)
	experiments := make(map[string]Experiment)
	path := "experiments/search"
	for {
		err := rs.do(http.MethodPost,
			path,
			protos.SearchExperiments{
				MaxResults: &maxResults,
				PageToken:  resp.NextPageToken,
			},
			&resp)
		if err != nil {
			return nil, err
		}
		for _, exp := range resp.Experiments {
			experiments[*exp.Name] = &restExperiment{rs, *exp.ExperimentId}
		}
		if resp.NextPageToken == nil {
			break
		}
	}
	return experiments, nil
}

func (rs *RESTStore) GetOrCreateExperimentWithName(name string) (Experiment, error) {
	if name == "" {
		name = defaultName
	}
	expsByName, err := rs.ExperimentsByName()
	if err != nil {
		return nil, err
	}
	if exp, ok := expsByName[name]; ok {
		return exp, nil
	}
	return rs.CreateExperiment(name)
}

func (rs *RESTStore) GetExperiment(id string) (Experiment, error) {
	if id == "" {
		id = defaultExperimentID
	}
	var resp protos.GetExperiment_Response
	err := rs.do(http.MethodGet, "experiments/get?experiment_id="+url.QueryEscape(id), nil, &resp)
	if err != nil {
		return nil, err
	}
	return &restExperiment{rs, *resp.Experiment.ExperimentId}, nil
}

func (rs *RESTStore) SearchRuns(experimentIDs []string, filter string, orderBy []string, pageToken string) ([]Run, string, error) {
	var resp protos.SearchRuns_Response
	req := protos.SearchRuns{
		ExperimentIds: experimentIDs,
		Filter:        &filter,
		OrderBy:       orderBy,
	}
	if pageToken != "" {
		req.PageToken = &pageToken
	}
	if err := rs.do(http.MethodPost, "runs/search", &req, &resp); err != nil {
		return nil, "", err
	}
	runs := make([]Run, len(resp.Runs))
	for i, run := range resp.Runs {
		runs[i] = &restRun{run.Info, run.Data, rs}
	}
	var nextPageToken string
	if resp.NextPageToken != nil {
		nextPageToken = *resp.NextPageToken
	}
	return runs, nextPageToken, nil
}

// Implements Experiment interface
type restExperiment struct {
	store *RESTStore
	id    string
}

func (exp *restExperiment) CreateRun(name string) (Run, error) {
	if name == "" {
		// This differs from Python client which generates a random adjective-noun-number.
		uuid := strings.ReplaceAll(uuid.NewString(), "-", "")
		name = uuid[0:8]
	}
	var resp protos.CreateRun_Response
	startTime := time.Now().UnixMilli()
	userKey := UserTagKey
	userName := ""
	user, err := user.Current()
	if err == nil {
		userName = user.Username
	}
	err = exp.store.do(http.MethodPost,
		"runs/create",
		protos.CreateRun{
			ExperimentId: &exp.id,
			RunName:      &name,
			StartTime:    &startTime,
			Tags: []*protos.RunTag{
				// Unfortunately Databricks ignores this tag.
				{Key: &userKey, Value: &userName},
			},
		},
		&resp)
	if err != nil {
		return nil, err
	}
	return &restRun{resp.Run.Info, resp.Run.Data, exp.store}, nil
}

func (exp *restExperiment) GetRun(runID string) (Run, error) {
	var resp protos.GetRun_Response
	err := exp.store.do(http.MethodGet, "runs/get?run_id="+url.QueryEscape(runID), nil, &resp)
	if err != nil {
		return nil, err
	}
	return &restRun{resp.Run.Info, resp.Run.Data, exp.store}, nil
}

func (exp *restExperiment) ID() string {
	return exp.id
}

type restRun struct {
	*protos.RunInfo
	*protos.RunData
	store *RESTStore
}

func (r *restRun) SetTag(key, value string) error {
	var resp protos.SetTag_Response
	if err := r.store.do(http.MethodPost,
		"runs/set-tag",
		protos.SetTag{RunId: r.RunId, Key: &key, Value: &value},
		&resp); err != nil {
		return err
	}
	for _, tag := range r.Tags {
		if *tag.Key == key {
			tag.Value = &value
			return nil
		}
	}
	r.Tags = append(r.Tags, &protos.RunTag{Key: &key, Value: &value})
	return nil
}

func (r *restRun) GetTag(key string) (string, error) {
	for _, tag := range r.Tags {
		if *tag.Key == key {
			return *tag.Value, nil
		}
	}
	return "", fmt.Errorf("tag %s not found", key)
}

func (r *restRun) LogArtifact(localPath, artifactPath string) error {
	// based on
	// https://github.com/mlflow/mlflow/blob/e7ff52d724e3218704fde225493e52c5acd41bb6/mlflow/tracking/_tracking_service/client.py#L401
	artifactRepo, err := r.store.newArtifactRepo(*r.ArtifactUri)
	if err != nil {
		return err
	}
	localInfo, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if localInfo.IsDir() {
		if artifactPath != "" {
			artifactPath = path.Join(artifactPath, localInfo.Name())
		}
		return artifactRepo.LogArtifacts(localPath, artifactPath)
	}
	return artifactRepo.LogArtifact(localPath, artifactPath)
}

func (r *restRun) LogMetric(key string, val float64, step int64) error {
	var resp protos.LogMetric_Response
	timestamp := time.Now().UnixMilli()
	return r.store.do(http.MethodPost,
		"runs/log-metric",
		protos.LogMetric{RunId: r.RunId, Key: &key, Value: &val, Step: &step, Timestamp: &timestamp},
		&resp)
}

func chunkEndIndices(arrayLen, chunkSize int) []int {
	res := make([]int, 0, (arrayLen+chunkSize-1)/chunkSize)
	for i := 0; i < arrayLen; i += chunkSize {
		end := i + chunkSize
		if end > arrayLen {
			end = arrayLen
		}
		res = append(res, end)
	}
	return res
}

func (r *restRun) LogMetrics(metrics []Metric, step int64) error {
	const maxMetricsPerBatch = 1000
	endIdxs := chunkEndIndices(len(metrics), maxMetricsPerBatch)
	i := 0
	timestamp := time.Now().UnixMilli()
	var resp protos.LogBatch_Response
	for _, endIdx := range endIdxs {
		metricsProtos := make([]*protos.Metric, 0, endIdx-i)
		for ; i < endIdx; i++ {
			metricsProtos = append(metricsProtos, &protos.Metric{
				Key:       &metrics[i].Key,
				Value:     &metrics[i].Val,
				Step:      &step,
				Timestamp: &timestamp,
			})
		}
		if err := r.store.do(http.MethodPost,
			"runs/log-batch",
			protos.LogBatch{RunId: r.RunId, Metrics: metricsProtos},
			&resp); err != nil {
			return err
		}
	}
	return nil
}

func (r *restRun) LogParam(key, value string) error {
	var resp protos.LogParam_Response
	return r.store.do(http.MethodPost,
		"runs/log-parameter",
		protos.LogParam{RunId: r.RunId, Key: &key, Value: &value},
		&resp)
}

func (r *restRun) LogParams(params []Param) error {
	const maxParamsPerBatch = 100
	endIdxs := chunkEndIndices(len(params), maxParamsPerBatch)
	i := 0
	var resp protos.LogBatch_Response
	for _, endIdx := range endIdxs {
		paramsProtos := make([]*protos.Param, 0, endIdx-i)
		for ; i < endIdx; i++ {
			paramsProtos = append(paramsProtos, &protos.Param{
				Key:   &params[i].Key,
				Value: &params[i].Val,
			})
		}
		if err := r.store.do(http.MethodPost,
			"runs/log-batch",
			protos.LogBatch{RunId: r.RunId, Params: paramsProtos},
			&resp); err != nil {
			return err
		}
	}
	return nil
}

func (r *restRun) SetName(name string) error {
	var resp protos.UpdateRun_Response
	if err := r.store.do(http.MethodPost,
		"runs/update",
		protos.UpdateRun{RunId: r.RunId, RunName: &name},
		&resp); err != nil {
		return err
	}
	r.RunName = &name
	return nil
}

func (r *restRun) Name() string {
	return *r.RunName
}

func (r *restRun) SetTags(tags []Tag) error {
	const maxTagsPerBatch = 100
	endIdxs := chunkEndIndices(len(tags), maxTagsPerBatch)
	i := 0
	var resp protos.LogBatch_Response
	for _, endIdx := range endIdxs {
		tagProtos := make([]*protos.RunTag, 0, endIdx-i)
		for ; i < endIdx; i++ {
			tagProtos = append(tagProtos, &protos.RunTag{
				Key:   &tags[i].Key,
				Value: &tags[i].Val,
			})
		}
		if err := r.store.do(http.MethodPost,
			"runs/log-batch",
			protos.LogBatch{RunId: r.RunId, Tags: tagProtos},
			&resp); err != nil {
			return err
		}
	}
	for _, tag := range tags {
		set := false
		for _, oldTag := range r.Tags {
			if *oldTag.Key == tag.Key {
				oldTag.Value = &tag.Val
				set = true
				break
			}
		}
		if !set {
			r.Tags = append(r.Tags, &protos.RunTag{Key: &tag.Key, Value: &tag.Val})
		}
	}
	return nil
}

func (r *restRun) End() error {
	var resp protos.UpdateRun_Response
	endTime := time.Now().UnixMilli()
	status := protos.RunStatus_FINISHED
	err := r.store.do(http.MethodPost,
		"runs/update",
		protos.UpdateRun{RunId: r.RunId, EndTime: &endTime, Status: &status},
		&resp)
	if err != nil {
		return err
	}
	endIfActive(r)
	return nil
}

func (r *restRun) Fail() error {
	var resp protos.UpdateRun_Response
	endTime := time.Now().UnixMilli()
	status := protos.RunStatus_FAILED
	err := r.store.do(http.MethodPost,
		"runs/update",
		protos.UpdateRun{RunId: r.RunId, EndTime: &endTime, Status: &status},
		&resp)
	if err != nil {
		return err
	}
	endIfActive(r)
	return nil
}

func (r *restRun) UIURL() string {
	middle := ""
	if strings.Contains(r.store.baseURL, "databricks.com") {
		middle = "#mlflow/"
	}
	return fmt.Sprintf("%s/%sexperiments/%s/runs/%s", r.store.baseURL, middle, *r.ExperimentId, *r.RunId)
}

func (r *restRun) ID() string {
	return *r.RunId
}

func (r *restRun) ExperimentID() string {
	return *r.ExperimentId
}

func (r *restRun) GetParam(key string) (string, error) {
	for _, param := range r.Params {
		if *param.Key == key {
			return *param.Value, nil
		}
	}
	return "", fmt.Errorf("param with key %s not found", key)
}

func (store *RESTStore) newArtifactRepo(artifactURI string) (ArtifactRepo, error) {
	parsed, err := url.Parse(artifactURI)
	if err != nil {
		return nil, err
	}
	switch parsed.Scheme {
	case "dbfs":
		return NewDBFSArtifactRepo(store, artifactURI)
	case "file", "":
		return NewFileArtifactRepo(parsed.Path)
	}
	return nil, fmt.Errorf("support for artifact repo with URI scheme %s not implemented", parsed.Scheme)
}
