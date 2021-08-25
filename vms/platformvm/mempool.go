// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/utils/timer"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm/platformcodec"
	"github.com/ava-labs/avalanchego/vms/platformvm/transactions"
)

const (
	// syncBound is the synchrony bound used for safe decision making
	syncBound = 10 * time.Second

	// BatchSize is the number of decision transactions.to place into a block
	BatchSize = 30

	MaxMempoolByteSize   = 3 * units.GiB // TODO: Should be default, configurable by users
	rejectedTxsCacheSize = 50
)

var (
	errEndOfTime              = errors.New("program time is suspiciously far in the future. Either this codebase was way more successful than expected, or a critical error has occurred")
	errNoPendingBlocks        = errors.New("no pending blocks")
	errUnknownTxType          = errors.New("unknown transaction type")
	errAttemptReRegisterTx    = errors.New("transaction already in mempool, could no reinsert")
	errTxExceedingMempoolSize = errors.New("dropping incoming tx since mempool would breach maximum size")
)

// Mempool implements a simple mempool to convert txs into valid blocks
type Mempool struct {
	vm *VM

	// TODO: factor out VM into separable interfaces

	// platformcodec.Codec
	// vm.ctx.Log
	// vm.ctx.Lock

	// vm.DB
	// vm.State.PutBlock()
	// vm.DB.Commit()

	// vm.Preferred()
	// vm.getBlock()

	// vm.getTimestamp()
	// vm.nextStakerStop()
	// vm.nextStakerChangeTime()

	// vm.newAdvanceTimeTx()
	// vm.newRewardValidatorTx()

	// vm.newStandardBlock()
	// vm.newAtomicBlock()
	// vm.newProposalBlock()

	// vm.SnowmanVM.NotifyBlockReady()

	// This timer goes off when it is time for the next validator to add/leave
	// the validator set. When it goes off ResetTimer() is called, potentially
	// triggering creation of a new block.
	timer *timer.Timer

	// Transactions that have not been put into blocks yet
	dropIncoming bool

	unissuedProposalTxs *EventHeap
	unissuedDecisionTxs []*transactions.SignedTx
	unissuedAtomicTxs   []*transactions.SignedTx

	rejectedProposalTxs *cache.LRU
	rejectedDecisionTxs *cache.LRU
	rejectedAtomicTxs   *cache.LRU

	unissuedTxs    map[ids.ID]*transactions.SignedTx
	totalBytesSize int
}

func (m *Mempool) has(txID ids.ID) bool {
	_, ok := m.unissuedTxs[txID]
	return ok
}

func (m *Mempool) hasRoomFor(tx *transactions.SignedTx) bool {
	return m.totalBytesSize+len(tx.Bytes()) <= MaxMempoolByteSize
}

func (m *Mempool) markReject(tx *transactions.SignedTx) error {
	switch tx.UnsignedTx.(type) {
	case VerifiableUnsignedProposalTx:
		m.rejectedProposalTxs.Put(tx.ID(), struct{}{})
	case VerifiableUnsignedDecisionTx:
		m.rejectedDecisionTxs.Put(tx.ID(), struct{}{})
	case VerifiableUnsignedAtomicTx:
		m.rejectedAtomicTxs.Put(tx.ID(), struct{}{})
	default:
		return errUnknownTxType
	}
	return nil
}

func (m *Mempool) isAlreadyRejected(txID ids.ID) bool {
	res := false
	if _, exist := m.rejectedProposalTxs.Get(txID); exist {
		res = true
	} else if _, exist := m.rejectedDecisionTxs.Get(txID); exist {
		res = true
	} else if _, exist := m.rejectedAtomicTxs.Get(txID); exist {
		res = true
	}

	return res
}

func (m *Mempool) register(tx *transactions.SignedTx) {
	m.unissuedTxs[tx.ID()] = tx
	m.totalBytesSize += len(tx.Bytes())
}

func (m *Mempool) deregister(tx *transactions.SignedTx) {
	delete(m.unissuedTxs, tx.ID())
	m.totalBytesSize -= len(tx.Bytes())
}

// Initialize this mempool.
func (m *Mempool) Initialize(vm *VM) {
	m.vm = vm

	m.vm.ctx.Log.Verbo("initializing platformVM mempool")

	// Transactions from clients that have not yet been put into blocks and
	// added to consensus
	m.unissuedTxs = make(map[ids.ID]*transactions.SignedTx)
	m.unissuedProposalTxs = &EventHeap{SortByStartTime: true}

	m.rejectedProposalTxs = &cache.LRU{Size: rejectedTxsCacheSize}
	m.rejectedDecisionTxs = &cache.LRU{Size: rejectedTxsCacheSize}
	m.rejectedAtomicTxs = &cache.LRU{Size: rejectedTxsCacheSize}

	m.timer = timer.NewTimer(func() {
		m.vm.ctx.Lock.Lock()
		defer m.vm.ctx.Lock.Unlock()

		m.ResetTimer()
	})
	go m.vm.ctx.Log.RecoverAndPanic(m.timer.Dispatch)
}

// IssueTx enqueues the [tx] to be put into a block
func (m *Mempool) IssueTx(tx *transactions.SignedTx) error {
	if m.dropIncoming {
		return nil
	}

	// Initialize the transaction
	if err := tx.Sign(platformcodec.Codec, nil); err != nil {
		return err
	}

	switch err := m.AddUncheckedTx(tx); err {
	case nil:
		if time.Now().Before(m.vm.gossipActivationTime) {
			m.vm.ctx.Log.Verbo("issued tx before gossiping activation time. Not gossiping it")
			return nil
		}

		txID := tx.ID()
		m.vm.ctx.Log.Debug("Gossiping txID %v", txID)
		txIDBytes, err := platformcodec.Codec.Marshal(platformcodec.Version, txID)
		if err != nil {
			return err
		}
		return m.vm.appSender.SendAppGossip(txIDBytes)
	case errAttemptReRegisterTx:
		return nil // backward compatibility
	default:
		return err
	}
}

func (m *Mempool) AddUncheckedTx(tx *transactions.SignedTx) error {
	txID := tx.ID()
	if m.has(txID) {
		return errAttemptReRegisterTx
	}
	if !m.hasRoomFor(tx) {
		return errTxExceedingMempoolSize
	}

	switch tx.UnsignedTx.(type) {
	case TimedTx:
		m.unissuedProposalTxs.Add(tx)
	case VerifiableUnsignedDecisionTx:
		m.unissuedDecisionTxs = append(m.unissuedDecisionTxs, tx)
	case VerifiableUnsignedAtomicTx:
		m.unissuedAtomicTxs = append(m.unissuedAtomicTxs, tx)
	default:
		return errUnknownTxType
	}
	m.register(tx)
	m.ResetTimer()
	return nil
}

// BuildBlock builds a block to be added to consensus
func (m *Mempool) BuildBlock() (snowman.Block, error) {
	m.dropIncoming = true
	defer func() {
		m.dropIncoming = false
	}()

	m.vm.ctx.Log.Debug("in BuildBlock")

	// Get the preferred block (which we want to build off)
	preferred, err := m.vm.Preferred()
	if err != nil {
		return nil, fmt.Errorf("couldn't get preferred block: %w", err)
	}

	preferredDecision, ok := preferred.(decision)
	if !ok {
		// The preferred block should always be a decision block
		return nil, errInvalidBlockType
	}

	preferredID := preferred.ID()
	nextHeight := preferred.Height() + 1

	// If there are pending decision txs, build a block with a batch of them
	if len(m.unissuedDecisionTxs) > 0 {
		numTxs := BatchSize
		if numTxs > len(m.unissuedDecisionTxs) {
			numTxs = len(m.unissuedDecisionTxs)
		}
		var txs []*transactions.SignedTx
		txs, m.unissuedDecisionTxs = m.unissuedDecisionTxs[:numTxs], m.unissuedDecisionTxs[numTxs:]
		for _, tx := range txs {
			m.deregister(tx)
		}
		blk, err := m.vm.newStandardBlock(preferredID, nextHeight, txs)
		if err != nil {
			m.ResetTimer()
			return nil, err
		}

		if err := blk.Verify(); err != nil {
			m.ResetTimer()
			return nil, err
		}

		m.vm.internalState.AddBlock(blk)
		return blk, m.vm.internalState.Commit()
	}

	// If there is a pending atomic tx, build a block with it
	if len(m.unissuedAtomicTxs) > 0 {
		tx := m.unissuedAtomicTxs[0]
		m.unissuedAtomicTxs = m.unissuedAtomicTxs[1:]
		m.deregister(tx)

		blk, err := m.vm.newAtomicBlock(preferredID, nextHeight, *tx)
		if err != nil {
			m.ResetTimer()
			return nil, err
		}

		if err := blk.Verify(); err != nil {
			m.ResetTimer()
			return nil, err
		}

		m.vm.internalState.AddBlock(blk)
		return blk, m.vm.internalState.Commit()
	}

	// The state if the preferred block were to be accepted
	preferredState := preferredDecision.onAccept()

	// The chain time if the preferred block were to be committed
	currentChainTimestamp := preferredState.GetTimestamp()
	if !currentChainTimestamp.Before(timer.MaxTime) {
		return nil, errEndOfTime
	}

	currentStakers := preferredState.CurrentStakerChainState()

	// If the chain time would be the time for the next primary network staker
	// to leave, then we create a block that removes the staker and proposes
	// they receive a staker reward
	tx, _, err := currentStakers.GetNextStaker()
	if err != nil {
		return nil, err
	}
	staker, ok := tx.UnsignedTx.(TimedTx)
	if !ok {
		return nil, fmt.Errorf("expected staker tx to be TimedTx but got %T", tx.UnsignedTx)
	}
	nextValidatorEndtime := staker.EndTime()
	if currentChainTimestamp.Equal(nextValidatorEndtime) {
		rewardValidatorTx, err := m.vm.newRewardValidatorTx(tx.ID())
		if err != nil {
			return nil, err
		}
		blk, err := m.vm.newProposalBlock(preferredID, nextHeight, *rewardValidatorTx)
		if err != nil {
			return nil, err
		}

		m.vm.internalState.AddBlock(blk)
		return blk, m.vm.internalState.Commit()
	}

	// If local time is >= time of the next staker set change,
	// propose moving the chain time forward
	nextStakerChangeTime, err := m.vm.nextStakerChangeTime(preferredState)
	if err != nil {
		return nil, err
	}

	localTime := m.vm.clock.Time()
	if !localTime.Before(nextStakerChangeTime) {
		// local time is at or after the time for the next staker to start/stop
		advanceTimeTx, err := m.vm.newAdvanceTimeTx(nextStakerChangeTime)
		if err != nil {
			return nil, err
		}
		blk, err := m.vm.newProposalBlock(preferredID, nextHeight, *advanceTimeTx)
		if err != nil {
			return nil, err
		}

		m.vm.internalState.AddBlock(blk)
		return blk, m.vm.internalState.Commit()
	}

	// Propose adding a new validator but only if their start time is in the
	// future relative to local time (plus Delta)
	syncTime := localTime.Add(syncBound)
	for m.unissuedProposalTxs.Len() > 0 {
		tx := m.unissuedProposalTxs.Peek()
		txID := tx.ID()
		utx := tx.UnsignedTx.(TimedTx)
		startTime := utx.StartTime()
		if startTime.Before(syncTime) {
			m.unissuedProposalTxs.Remove()
			m.deregister(tx)

			errMsg := fmt.Sprintf(
				"synchrony bound (%s) is later than staker start time (%s)",
				syncTime,
				startTime,
			)
			m.vm.droppedTxCache.Put(txID, errMsg) // cache tx as dropped
			m.vm.ctx.Log.Debug("dropping tx %s: %s", txID, errMsg)
			continue
		}

		maxLocalStartTime := localTime.Add(maxFutureStartTime)
		// If the start time is too far in the future relative to local time
		// drop the transactions.and continue
		if startTime.After(maxLocalStartTime) {
			m.unissuedProposalTxs.Remove()
			m.deregister(tx)
			continue
		}

		// If the chain timestamp is too far in the past to issue this
		// transactions.but according to local time, it's ready to be issued,
		// then attempt to advance the timestamp, so it can be issued.
		maxChainStartTime := currentChainTimestamp.Add(maxFutureStartTime)
		if startTime.After(maxChainStartTime) {
			advanceTimeTx, err := m.vm.newAdvanceTimeTx(localTime)
			if err != nil {
				return nil, err
			}
			blk, err := m.vm.newProposalBlock(preferredID, nextHeight, *advanceTimeTx)
			if err != nil {
				return nil, err
			}

			m.vm.internalState.AddBlock(blk)
			return blk, m.vm.internalState.Commit()
		}

		// Attempt to issue the transaction
		m.unissuedProposalTxs.Remove()
		m.deregister(tx)
		blk, err := m.vm.newProposalBlock(preferredID, nextHeight, *tx)
		if err != nil {
			m.ResetTimer()
			return nil, err
		}

		if err := blk.Verify(); err != nil {
			m.ResetTimer()
			return nil, err
		}

		m.vm.internalState.AddBlock(blk)
		return blk, m.vm.internalState.Commit()
	}

	m.vm.ctx.Log.Debug("BuildBlock returning error (no blocks)")
	return nil, errNoPendingBlocks
}

// ResetTimer Check if there is a block ready to be added to consensus. If so, notify the
// consensus engine.
func (m *Mempool) ResetTimer() {
	// If there is a pending transactions. trigger building of a block with that
	// transaction
	if len(m.unissuedDecisionTxs) > 0 || len(m.unissuedAtomicTxs) > 0 {
		m.vm.NotifyBlockReady()
		return
	}

	// Get the preferred block (which we want to build off)
	preferred, err := m.vm.Preferred()
	if err != nil {
		m.vm.ctx.Log.Error("error fetching the preferred block: %s", err)
		return
	}

	preferredDecision, ok := preferred.(decision)
	if !ok {
		m.vm.ctx.Log.Error("the preferred block %q should be a decision block", preferred.ID())
		return
	}

	// The state if the preferred block were to be accepted
	preferredState := preferredDecision.onAccept()

	// The chain time if the preferred block were to be accepted
	timestamp := preferredState.GetTimestamp()
	if timestamp.Equal(timer.MaxTime) {
		m.vm.ctx.Log.Error("program time is suspiciously far in the future. Either this codebase was way more successful than expected, or a critical error has occurred")
		return
	}

	// If local time is >= time of the next change in the validator set,
	// propose moving forward the chain timestamp
	nextStakerChangeTime, err := m.vm.nextStakerChangeTime(preferredState)
	if err != nil {
		m.vm.ctx.Log.Error("couldn't get next staker change time: %s", err)
		return
	}
	if timestamp.Equal(nextStakerChangeTime) {
		m.vm.NotifyBlockReady() // Should issue a proposal to reward a validator
		return
	}

	localTime := m.vm.clock.Time()
	if !localTime.Before(nextStakerChangeTime) { // time is at or after the time for the next validator to join/leave
		m.vm.NotifyBlockReady() // Should issue a proposal to advance timestamp
		return
	}

	syncTime := localTime.Add(syncBound)
	for m.unissuedProposalTxs.Len() > 0 {
		startTime := m.unissuedProposalTxs.Peek().UnsignedTx.(TimedTx).StartTime()
		if !syncTime.After(startTime) {
			m.vm.NotifyBlockReady() // Should issue a ProposeAddValidator
			return
		}
		// If the tx doesn't meet the synchrony bound, drop it
		tx := m.unissuedProposalTxs.Remove()
		txID := tx.ID()
		m.deregister(tx)
		errMsg := fmt.Sprintf(
			"synchrony bound (%s) is later than staker start time (%s)",
			syncTime,
			startTime,
		)
		m.vm.droppedTxCache.Put( // cache tx as dropped
			txID,
			errMsg,
		)
		m.vm.ctx.Log.Debug("dropping tx %s: %s", txID, errMsg)
	}

	waitTime := nextStakerChangeTime.Sub(localTime)
	m.vm.ctx.Log.Debug("next scheduled event is at %s (%s in the future)", nextStakerChangeTime, waitTime)

	// Wake up when it's time to add/remove the next validator
	m.timer.SetTimeoutIn(waitTime)
}

// Shutdown this mempool
func (m *Mempool) Shutdown() {
	if m.timer == nil {
		return
	}

	// There is a potential deadlock if the timer is about to execute a timeout.
	// So, the lock must be released before stopping the timer.
	m.vm.ctx.Lock.Unlock()
	m.timer.Stop()
	m.vm.ctx.Lock.Lock()
}
