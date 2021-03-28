package platformvm

import (
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	assert := assert.New(t)

	db := memdb.New()
	_, err := initState(db)
	assert.NoError(err)

	is, err := loadState(db)
	assert.NoError(err)

	_ = is
}
