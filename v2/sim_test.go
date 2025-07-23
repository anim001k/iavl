package iavl

import (
	"os"
	"testing"

	"cosmossdk.io/log"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
	"pgregory.net/rapid"
)

func TestIAVLV2Sims(t *testing.T) {
	rapid.Check(t, testIAVLV2Sims)
}

func FuzzIAVLV2(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(testIAVLV2Sims))
}

func testIAVLV2Sims(t *rapid.T) {
	logger := log.NewTestLogger(t)
	dbV1 := dbm.NewMemDB()
	treeV1 := iavl.NewMutableTree(dbV1, 500000, true, logger)

	tempSqlitePath, err := os.MkdirTemp("", "iavl2")
	defer os.RemoveAll(tempSqlitePath)

	require.NoError(t, err, "failed to create temp directory for SQLite database")
	nodePool := NewNodePool()
	sqliteOpts := SqliteDbOptions{
		Path:   tempSqlitePath,
		Logger: logger,
	}
	sqliteDb, err := NewSqliteDb(nodePool, sqliteOpts)
	require.NoError(t, err, "failed to create SQLite database")

	treeOpts := DefaultTreeOptions()

	treeV2 := NewTree(sqliteDb, nodePool, treeOpts)
	simMachine := &SimMachine{
		treeV1:       treeV1,
		treeV2:       treeV2,
		existingKeys: map[string][]byte{},
	}

	t.Repeat(rapid.StateMachineActions(simMachine))
}

type SimMachine struct {
	treeV1 *iavl.MutableTree
	treeV2 *Tree
	// existingKeys keeps track of keys that have been set in the tree or deleted. Deleted keys are retained as nil values.
	existingKeys map[string][]byte
}

var _ rapid.StateMachine = &SimMachine{}

func (s SimMachine) Check(_ *rapid.T) {}

func (s SimMachine) Set(t *rapid.T) {
	// choose either a new or an existing key
	key := s.selectKey(t)
	value := rapid.SliceOfN(rapid.Byte(), 0, 10).Draw(t, "value")
	// set in both trees
	updated, errV1 := s.treeV1.Set(key, value)
	require.NoError(t, errV1, "failed to set key in V1 tree")
	updatedV2, errV2 := s.treeV2.Set(key, value)
	require.NoError(t, errV2, "failed to set key in V2 tree")
	require.Equal(t, updated, updatedV2, "update status mismatch between V1 and V2 trees")
	if updated {
		require.NotNil(t, s.existingKeys[string(key)], "key shouldn't have been marked as updated")
	} else {
		existing, found := s.existingKeys[string(key)]
		if found {
			require.Nil(t, existing, value, "marked as not an update but existin key is nil")
		}
	}
	s.existingKeys[string(key)] = value // mark as existing
}

func (s SimMachine) Get(t *rapid.T) {
	var key = s.selectKey(t)
	valueV1, errV1 := s.treeV1.Get(key)
	require.NoError(t, errV1, "failed to get key from V1 tree")
	valueV2, errV2 := s.treeV2.Get(key)
	require.NoError(t, errV2, "failed to get key from V2 tree")
	require.Equal(t, valueV1, valueV2, "value mismatch between V1 and V2 trees")
	expectedValue, found := s.existingKeys[string(key)]
	if found {
		require.Equal(t, expectedValue, valueV1, "expected value mismatch for key %s", key)
	} else {
		require.Nil(t, valueV1, "expected nil value for non-existing key %s", key)
	}
}

func (s SimMachine) selectKey(t *rapid.T) []byte {
	if len(s.existingKeys) > 0 && rapid.Bool().Draw(t, "existingKey") {
		return []byte(rapid.SampledFrom(maps.Keys(s.existingKeys)).Draw(t, "key"))
	} else {
		return rapid.SliceOfN(rapid.Byte(), 0, 10).Draw(t, "key")
	}
}

func (s SimMachine) Delete(t *rapid.T) {
	key := s.selectKey(t)
	existingValue, found := s.existingKeys[string(key)]
	exists := found && existingValue != nil
	// delete in both trees
	_, removedV1, errV1 := s.treeV1.Remove(key)
	require.NoError(t, errV1, "failed to remove key from V1 tree")
	_, removedV2, errV2 := s.treeV2.Remove(key)
	require.NoError(t, errV2, "failed to remove key from V2 tree")
	require.Equal(t, removedV1, removedV2, "removed status mismatch between V1 and V2 trees")
	// TODO v1 & v2 have slightly different behaviors for the value returned on removal. We should re-enable this and check.
	//if valueV1 == nil || len(valueV1) == 0 {
	//	require.Empty(t, valueV2, "value should be empty for removed key in V2 tree")
	//} else {
	//	require.Equal(t, valueV1, valueV2, "value mismatch between V1 and V2 trees")
	//}
	require.Equal(t, exists, removedV1, "removed status should match existence of key")
	s.existingKeys[string(key)] = nil // mark as deleted
}
