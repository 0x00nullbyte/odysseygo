package indexer

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/prefixdb"
	"github.com/ava-labs/avalanchego/database/versiondb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/timer"
	"github.com/ava-labs/avalanchego/utils/wrappers"
)

const (
	// Maximum number of containers IDs that can be fetched at a time
	// in a call to GetContainerRange
	MaxFetchedByRange = 1024
)

var (
	// Maps to the byte representation of the next accepted index
	nextAcceptedIndexKey   []byte = []byte{0x00}
	indexToContainerPrefix []byte = []byte{0x01}
	containerToIDPrefix    []byte = []byte{0x02}
	errNoneAccepted               = errors.New("no containers have been accepted")
	errNumToFetchZero             = fmt.Errorf("numToFetch must be in [1,%d]", MaxFetchedByRange)

	_ Index = &index{}
)

// Index indexes container (a blob of bytes with an ID) in their order of acceptance
// Index implements triggers.Acceptor
// Index is thread-safe.
type Index interface {
	Accept(ctx *snow.Context, containerID ids.ID, container []byte) error
	GetContainerByIndex(index uint64) (Container, error)
	GetContainerRange(startIndex uint64, numToFetch uint64) ([]Container, error)
	GetLastAccepted() (Container, error)
	GetIndex(containerID ids.ID) (uint64, error)
	GetContainerByID(containerID ids.ID) (Container, error)
	Close() error
}

// Returns a new, thread-safe Index.
// Closes [baseDB] on close.
func newIndex(
	baseDB database.Database,
	log logging.Logger,
	codec codec.Manager,
	clock timer.Clock,
	isAcceptedFunc func(containerID ids.ID) bool,
) (Index, error) {
	vDB := versiondb.New(baseDB)
	indexToContainer := prefixdb.New(indexToContainerPrefix, vDB)
	containerToIndex := prefixdb.New(containerToIDPrefix, vDB)

	i := &index{
		clock:            clock,
		codec:            codec,
		baseDB:           baseDB,
		vDB:              vDB,
		indexToContainer: indexToContainer,
		containerToIndex: containerToIndex,
		log:              log,
		isAcceptedFunc:   isAcceptedFunc,
	}

	// Get next accepted index from db
	nextAcceptedIndexBytes, err := i.vDB.Get(nextAcceptedIndexKey)
	if err == database.ErrNotFound {
		// Couldn't find it in the database. Must not have accepted any containers in previous runs.
		i.log.Info("next accepted index %d", i.nextAcceptedIndex)
		return i, nil
	}
	if err != nil {
		return nil, fmt.Errorf("couldn't get next accepted index from database: %w", err)
	}
	p := wrappers.Packer{Bytes: nextAcceptedIndexBytes}
	i.nextAcceptedIndex = p.UnpackLong()
	if p.Err != nil {
		return nil, fmt.Errorf("couldn't parse next accepted index from bytes: %w", err)
	}
	i.log.Info("next accepted index %d", i.nextAcceptedIndex)

	// We may have committed some containers in the index's DB that were not committed at
	// the VM's DB. Go back through recently accepted things and make sure they're accepted.
	for j := i.nextAcceptedIndex; j >= 1; j-- {
		lastAccepted, err := i.getContainerByIndex(j - 1)
		if err != nil {
			return nil, fmt.Errorf("couldn't get container at index %d: %s", j-1, err)
		}
		if isAcceptedFunc(lastAccepted.ID) {
			break
		}
		if err := i.removeLastAccepted(lastAccepted.ID); err != nil {
			return nil, fmt.Errorf("couldn't remove container: %s", err)
		}
	}
	return i, nil
}

// indexer indexes all accepted transactions by the order in which they were accepted
type index struct {
	codec          codec.Manager
	isAcceptedFunc func(containerID ids.ID) bool
	clock          timer.Clock
	lock           sync.RWMutex
	// The index of the next accepted transaction
	nextAcceptedIndex uint64
	// When [baseDB] is committed, actual write to disk happens
	vDB    *versiondb.Database
	baseDB database.Database
	// Both [indexToContainer] and [containerToIndex] have [baseDB] underneath
	// Index --> Container
	indexToContainer database.Database
	// Container ID --> Index
	containerToIndex database.Database
	log              logging.Logger
}

// Close this index
func (i *index) Close() error {
	errs := wrappers.Errs{}
	errs.Add(i.indexToContainer.Close())
	errs.Add(i.containerToIndex.Close())
	errs.Add(i.vDB.Close())
	errs.Add(i.baseDB.Close())
	return errs.Err
}

// Index that the given transaction is accepted
// Returned error should be treated as fatal
func (i *index) Accept(ctx *snow.Context, containerID ids.ID, containerBytes []byte) error {
	i.lock.Lock()
	defer i.lock.Unlock()

	ctx.Log.Debug("indexing %d --> container %s", i.nextAcceptedIndex, containerID)
	// Persist index --> Container
	p := wrappers.Packer{MaxSize: wrappers.LongLen}
	p.PackLong(i.nextAcceptedIndex)
	if p.Err != nil {
		return fmt.Errorf("couldn't convert next accepted index to bytes: %w", p.Err)
	}
	bytes, err := i.codec.Marshal(codecVersion, Container{
		Bytes:     containerBytes,
		ID:        containerID,
		Timestamp: i.clock.Time().UnixNano(),
	})
	if err != nil {
		return fmt.Errorf("couldn't serialize container %s: %w", containerID, err)
	}
	if err := i.indexToContainer.Put(p.Bytes, bytes); err != nil {
		return fmt.Errorf("couldn't put accepted container %s into index: %w", containerID, err)
	}

	// Persist container ID --> index
	if err := i.containerToIndex.Put(containerID[:], p.Bytes); err != nil {
		return fmt.Errorf("couldn't map container %s to index: %w", containerID, err)
	}

	// Persist next accepted index
	i.nextAcceptedIndex++
	p = wrappers.Packer{MaxSize: wrappers.LongLen}
	p.PackLong(i.nextAcceptedIndex)
	if p.Err != nil {
		return fmt.Errorf("couldn't convert next accepted index to bytes: %w", p.Err)
	}
	if err := i.vDB.Put(nextAcceptedIndexKey, p.Bytes); err != nil {
		return fmt.Errorf("couldn't put accepted container %s into index: %w", containerID, err)
	}

	return i.vDB.Commit()
}

// Returns the ID of the [index]th accepted container and the container itself.
// For example, if [index] == 0, returns the first accepted container.
// If [index] == 1, returns the second accepted container, etc.
func (i *index) GetContainerByIndex(index uint64) (Container, error) {
	i.lock.RLock()
	defer i.lock.RUnlock()

	return i.getContainerByIndex(index)
}

func (i *index) getContainerByIndex(index uint64) (Container, error) {
	lastAcceptedIndex, ok := i.lastAcceptedIndex()
	if !ok || index > lastAcceptedIndex {
		return Container{}, fmt.Errorf("no container at index %d", index)
	}

	p := wrappers.Packer{MaxSize: wrappers.LongLen}
	p.PackLong(index)
	if p.Err != nil {
		return Container{}, fmt.Errorf("couldn't convert index to bytes: %w", p.Err)
	}

	containerBytes, err := i.indexToContainer.Get(p.Bytes)
	switch {
	case err == database.ErrNotFound:
		return Container{}, fmt.Errorf("no container at index %d", index)
	case err != nil:
		i.log.Error("couldn't read container from database: %w", err)
		return Container{}, fmt.Errorf("couldn't read from database: %w", err)
	}

	var container Container
	if _, err = i.codec.Unmarshal(containerBytes, &container); err != nil {
		return Container{}, fmt.Errorf("couldn't unmarshal container: %w", err)
	}
	return container, nil
}

// GetContainerRange returns the IDs of containers at index
// [startIndex], [startIndex+1], ..., [startIndex+numToFetch-1]
func (i *index) GetContainerRange(startIndex, numToFetch uint64) ([]Container, error) {
	// Check arguments for validity
	if numToFetch == 0 {
		return nil, errNumToFetchZero
	} else if numToFetch > MaxFetchedByRange {
		return nil, fmt.Errorf("requested %d but maximum page size is %d", numToFetch, MaxFetchedByRange)
	}
	i.lock.RLock()
	defer i.lock.RUnlock()

	lastAcceptedIndex, ok := i.lastAcceptedIndex()
	if !ok {
		return nil, errNoneAccepted
	} else if startIndex > lastAcceptedIndex {
		return nil, fmt.Errorf("start index (%d) > last accepted index (%d)", startIndex, lastAcceptedIndex)
	}

	// Calculate the last index we will fetch
	lastIndex := math.Min64(startIndex+numToFetch-1, lastAcceptedIndex)
	// [lastIndex] is always >= [startIndex] so this is safe.
	// [n] is limited to [MaxFetchedByRange] so [containerIDs] can't be crazy big.
	containers := make([]Container, int(lastIndex)-int(startIndex)+1)

	n := 0
	for j := startIndex; j <= lastIndex; j++ {
		// Convert index to bytes
		p := wrappers.Packer{MaxSize: wrappers.LongLen}
		p.PackLong(j)
		if p.Err != nil {
			return nil, fmt.Errorf("couldn't convert index %d to bytes: %w", j, p.Err)
		}

		// Get container from database and deserialize
		containerBytes, err := i.indexToContainer.Get(p.Bytes)
		if err != nil {
			return nil, fmt.Errorf("couldn't get container from database: %w", err)
		}
		var container Container
		if _, err := i.codec.Unmarshal(containerBytes, &container); err != nil {
			return nil, fmt.Errorf("couldn't unmarshal container: %w", err)
		}
		containers[n] = container
		n++
	}
	return containers, nil
}

func (i *index) GetIndex(containerID ids.ID) (uint64, error) {
	i.lock.RLock()
	defer i.lock.RUnlock()

	indexBytes, err := i.containerToIndex.Get(containerID[:])
	if err != nil {
		return 0, err
	}

	p := wrappers.Packer{Bytes: indexBytes}
	index := p.UnpackLong()
	if p.Err != nil {
		// Should never happen
		i.log.Error("couldn't unpack index: %w", err)
		return 0, p.Err
	}
	return index, nil
}

func (i *index) GetContainerByID(containerID ids.ID) (Container, error) {
	i.lock.RLock()
	defer i.lock.RUnlock()

	// Read index from database
	indexBytes, err := i.containerToIndex.Get(containerID[:])
	if err != nil {
		return Container{}, err
	}

	// Read container from database
	containerBytes, err := i.indexToContainer.Get(indexBytes)
	if err != nil {
		err = fmt.Errorf("couldn't read container from database: %w", err)
		i.log.Error("%s", err)
		return Container{}, err
	}

	// Parse container
	var container Container
	if _, err = i.codec.Unmarshal(containerBytes, &container); err != nil {
		return Container{}, fmt.Errorf("couldn't unmarshal container: %w", err)
	}
	return container, nil
}

// GetLastAccepted returns the last accepted container
// Returns an error if no containers have been accepted
func (i *index) GetLastAccepted() (Container, error) {
	lastAcceptedIndex, exists := i.lastAcceptedIndex()
	if !exists {
		return Container{}, errNoneAccepted
	}
	return i.GetContainerByIndex(lastAcceptedIndex)
}

// Returns:
// 1) The index of the most recently accepted transaction,
//    or 0 if no transactions have been accepted
// 2) Whether at least 1 transaction has been accepted
func (i *index) lastAcceptedIndex() (uint64, bool) {
	i.lock.RLock()
	defer i.lock.RUnlock()

	return i.nextAcceptedIndex - 1, i.nextAcceptedIndex != 0
}

// Remove the last accepted container, whose ID is given, from the databases
// Assumes [p.nextAcceptedIndex] >= 1
// Assumes [containerID] is actually the ID of the last accepted container
func (i *index) removeLastAccepted(containerID ids.ID) error {
	i.lock.Lock()
	defer i.lock.Unlock()

	if err := i.containerToIndex.Delete(containerID[:]); err != nil {
		return err
	}

	p := wrappers.Packer{MaxSize: wrappers.LongLen}
	p.PackLong(i.nextAcceptedIndex - 1)
	if p.Err != nil {
		return fmt.Errorf("couldn't convert last accepted index to bytes: %w", p.Err)
	}

	if err := i.indexToContainer.Delete(p.Bytes); err != nil {
		return fmt.Errorf("couldn't remove last accepted: %w", err)
	}

	i.nextAcceptedIndex--

	p = wrappers.Packer{MaxSize: wrappers.LongLen}
	p.PackLong(i.nextAcceptedIndex)
	if p.Err != nil {
		return fmt.Errorf("couldn't convert next accepted index to bytes: %w", p.Err)
	}
	if err := i.vDB.Put(nextAcceptedIndexKey, p.Bytes); err != nil {
		return fmt.Errorf("couldn't put next accepted key: %s", err)
	}
	return i.vDB.Commit()
}
