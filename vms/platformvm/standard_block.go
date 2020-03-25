// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"errors"

	"github.com/ava-labs/gecko/database"
	"github.com/ava-labs/gecko/database/versiondb"
	"github.com/ava-labs/gecko/ids"
	"github.com/ava-labs/gecko/snow/choices"
	"github.com/ava-labs/gecko/vms/components/core"
)

var (
	errConflictingParentTxs = errors.New("block contains a transaction that conflicts with a transaction in a parent block")
	errConflictingTxs       = errors.New("block contains conflicting transactions")
)

// DecisionTx is an operation that can be decided without being proposed
type DecisionTx interface {
	initialize(vm *VM) error

	InputUTXOs() ids.Set

	// Attempt to verify this transaction with the provided state. The provided
	// database can be modified arbitrarily. If a nil error is returned, it is
	// assumped onAccept is non-nil.
	SemanticVerify(database.Database) (onAccept func(), err error)
}

// StandardBlock being accepted results in the transactions contained in the
// block to be accepted and committed to the chain.
type StandardBlock struct {
	CommonDecisionBlock `serialize:"true"`

	Txs []DecisionTx `serialize:"true"`

	inputs ids.Set
}

// initialize this block
func (sb *StandardBlock) initialize(vm *VM, bytes []byte) error {
	if err := sb.CommonDecisionBlock.initialize(vm, bytes); err != nil {
		return err
	}
	for _, tx := range sb.Txs {
		if err := tx.initialize(vm); err != nil {
			return err
		}
	}
	return nil
}

// Reject implements the snowman.Block interface
func (sb *StandardBlock) conflicts(s ids.Set) bool {
	if sb.Status() == choices.Accepted {
		return false
	}
	if sb.inputs.Overlaps(s) {
		return true
	}
	return sb.parentBlock().conflicts(s)
}

// Verify this block performs a valid state transition.
//
// The parent block must be a proposal
//
// This function also sets onAcceptDB database if the verification passes.
func (sb *StandardBlock) Verify() error {
	parentBlock := sb.parentBlock()
	// StandardBlock is not a modifier on a proposal block, so its parent must
	// be a decision.
	parent, ok := parentBlock.(decision)
	if !ok {
		return errInvalidBlockType
	}

	pdb := parent.onAccept()

	sb.onAcceptDB = versiondb.New(pdb)
	funcs := []func(){}
	for _, tx := range sb.Txs {
		onAccept, err := tx.SemanticVerify(sb.onAcceptDB)
		if err != nil {
			return err
		}
		inputs := tx.InputUTXOs()
		if inputs.Overlaps(sb.inputs) {
			return errConflictingTxs
		}
		sb.inputs.Union(inputs)
		if onAccept != nil {
			funcs = append(funcs, onAccept)
		}
	}

	if parentBlock.conflicts(sb.inputs) {
		return errConflictingParentTxs
	}

	if numFuncs := len(funcs); numFuncs == 1 {
		sb.onAcceptFunc = funcs[0]
	} else if numFuncs > 1 {
		sb.onAcceptFunc = func() {
			for _, f := range funcs {
				f()
			}
		}
	}

	sb.vm.currentBlocks[sb.ID().Key()] = sb
	sb.parentBlock().addChild(sb)
	return nil
}

// newStandardBlock returns a new *StandardBlock where the block's parent, a
// decision block, has ID [parentID].
func (vm *VM) newStandardBlock(parentID ids.ID, txs []DecisionTx) (*StandardBlock, error) {
	sb := &StandardBlock{
		CommonDecisionBlock: CommonDecisionBlock{
			CommonBlock: CommonBlock{
				Block: core.NewBlock(parentID),
				vm:    vm,
			},
		},
		Txs: txs,
	}

	// We serialize this block as a Block so that it can be deserialized into a
	// Block
	blk := Block(sb)
	bytes, err := Codec.Marshal(&blk)
	if err != nil {
		return nil, err
	}
	sb.Block.Initialize(bytes, vm.SnowmanVM)
	return sb, nil
}
