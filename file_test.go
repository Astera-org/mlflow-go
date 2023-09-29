package mlflow

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStoreExperiments(t *testing.T) {
	fs, err := NewFileStore(t.TempDir())
	require.NoError(t, err)
	expsByName, err := fs.ExperimentsByName()
	require.NoError(t, err)
	assert.Equal(t, len(expsByName), 1, "expected only the default experiment", len(expsByName))

	require.NoError(t, os.RemoveAll(fs.rootDir))
	expsByName, err = fs.ExperimentsByName()
	assert.NoError(t, err)
	assert.Equal(t, len(expsByName), 0)

	for i := 0; i < 2; i++ {
		name := fmt.Sprintf("test%d", i)
		exp, err := fs.GetOrCreateExperimentWithName(name)
		require.NoError(t, err)
		expsByName, err = fs.ExperimentsByName()
		require.NoError(t, err)
		assert.Equal(t, len(expsByName), i+1)
		assert.Equal(t, expsByName[name].(*fileExperiment).ExperimentID, exp.(*fileExperiment).ExperimentID)
	}
}

func TestRun(t *testing.T) {
	fs, err := NewFileStore(t.TempDir())
	require.NoError(t, err)
	exp, err := fs.GetOrCreateExperimentWithName("exp0")
	require.NoError(t, err)

	_, err = exp.GetRun("run0")
	require.Error(t, err)

	created, err := exp.CreateRun("run0")
	require.NoError(t, err)

	got, err := exp.GetRun(created.(*fileRun).RunID)
	require.NoError(t, err)

	assert.Equal(t, got.(*fileRun).RunID, created.(*fileRun).RunID)

	assert.NoError(t, got.SetName("new name"))

	const tagKey = "tag0"
	const tagVal = "val0"
	require.NoError(t, created.SetTag(tagKey, tagVal))
	gotTag, err := created.GetTag(tagKey)
	require.NoError(t, err)
	assert.Equal(t, gotTag, tagVal)

	assert.NoError(t, created.End())
}
