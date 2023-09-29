package mlflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkEndIndices(t *testing.T) {
	idxs := chunkEndIndices(0, 10)
	assert.Equal(t, []int{}, idxs)

	idxs = chunkEndIndices(1, 10)
	assert.Equal(t, []int{1}, idxs)

	idxs = chunkEndIndices(10, 10)
	assert.Equal(t, []int{10}, idxs)

	idxs = chunkEndIndices(11, 10)
	assert.Equal(t, []int{10, 11}, idxs)

	idxs = chunkEndIndices(20, 10)
	assert.Equal(t, []int{10, 20}, idxs)

	idxs = chunkEndIndices(21, 10)
	assert.Equal(t, []int{10, 20, 21}, idxs)

	idxs = chunkEndIndices(22, 10)
	assert.Equal(t, []int{10, 20, 22}, idxs)
}
