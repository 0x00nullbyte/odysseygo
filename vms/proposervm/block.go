// (c) 2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package proposervm

import (
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
)

const (
	// allowable block issuance in the future
	syncBound = 10 * time.Second
)

var (
	errUnsignedChild            = errors.New("expected child to be signed")
	errUnexpectedBlockType      = errors.New("unexpected proposer block type type")
	errInnerParentMismatch      = errors.New("inner parentID didn't match expected parent")
	errTimeNotMonotonic         = errors.New("time must monotonically increase")
	errPChainHeightNotMonotonic = errors.New("p chain height must monotonically increase")
	errTimeTooAdvanced          = errors.New("time is too far advanced")
	errProposerWindowNotStarted = errors.New("proposer window hasn't started")

	errProposersNotActivated = errors.New("proposers haven't been activated yet")
	errProposersActivated    = errors.New("proposers have been activated")
)

type Block interface {
	snowman.Block

	verifyPreForkChild(child *preForkBlock) error
	verifyPostForkChild(child *postForkBlock) error
	verifyPostForkOption(child *postForkOption) error

	buildChild(innerBlock snowman.Block) (Block, error)
}
