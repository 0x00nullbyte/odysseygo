// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snowman

import (
	"fmt"
	"time"

	"github.com/ava-labs/gecko/ids"
	"github.com/ava-labs/gecko/network"
	"github.com/ava-labs/gecko/snow"
	"github.com/ava-labs/gecko/snow/choices"
	"github.com/ava-labs/gecko/snow/consensus/snowball"
	"github.com/ava-labs/gecko/snow/consensus/snowman"
	"github.com/ava-labs/gecko/snow/consensus/snowman/poll"
	"github.com/ava-labs/gecko/snow/engine/common"
	"github.com/ava-labs/gecko/snow/engine/snowman/bootstrap"
	"github.com/ava-labs/gecko/snow/events"
	"github.com/ava-labs/gecko/utils/formatting"
	"github.com/ava-labs/gecko/utils/wrappers"
)

const (
	// TODO define this constant in one place rather than here and in snowman
	// Max containers size in a MultiPut message
	maxContainersLen = int(4 * network.DefaultMaxMessageSize / 5)
)

// Transitive implements the Engine interface by attempting to fetch all
// transitive dependencies.
type Transitive struct {
	bootstrap.Bootstrapper
	metrics

	params    snowball.Parameters
	consensus snowman.Consensus

	// track outstanding preference requests
	polls poll.Set

	// blocks that have we have sent get requests for but haven't yet receieved
	blkReqs common.Requests

	// blocks that are queued to be added to consensus once missing dependencies are fetched
	pending ids.Set

	// operations that are blocked on a block being issued. This could be
	// issuing another block, responding to a query, or applying votes to consensus
	blocked events.Blocker

	// errs tracks if an error has occurred in a callback
	errs wrappers.Errs
}

// Initialize implements the Engine interface
func (t *Transitive) Initialize(config Config) error {
	config.Context.Log.Info("initializing consensus engine")

	t.params = config.Params
	t.consensus = config.Consensus

	factory := poll.NewEarlyTermNoTraversalFactory(int(config.Params.Alpha))
	t.polls = poll.NewSet(factory,
		config.Context.Log,
		config.Params.Namespace,
		config.Params.Metrics,
	)

	if err := t.metrics.Initialize(config.Params.Namespace, config.Params.Metrics); err != nil {
		return err
	}

	return t.Bootstrapper.Initialize(
		config.Config,
		t.finishBootstrapping,
		fmt.Sprintf("%s_bs", config.Params.Namespace),
		config.Params.Metrics,
	)
}

// When bootstrapping is finished, this will be called.
// This initializes the consensus engine with the last accepted block.
func (t *Transitive) finishBootstrapping() error {
	// initialize consensus to the last accepted blockID
	lastAcceptedID := t.VM.LastAccepted()
	t.consensus.Initialize(t.Context(), t.params, lastAcceptedID)

	tail, err := t.VM.GetBlock(lastAcceptedID)
	if err != nil {
		t.Context().Log.Error("failed to get last accepted block due to: %s", err)
		return err
	}
	// to maintain the invariant that oracle blocks are issued in the correct
	// preferences, we need to handle the case that we are bootstrapping into an oracle block
	switch blk := tail.(type) {
	case OracleBlock:
		options, err := blk.Options()
		if err != nil {
			return err
		}
		for _, blk := range options {
			// note that deliver will set the VM's preference
			if err := t.deliver(blk); err != nil {
				return err
			}
		}
	default:
		// if there aren't blocks we need to deliver on startup, we need to set
		// the preference to the last accepted block
		t.VM.SetPreference(lastAcceptedID)
	}

	t.Context().Log.Info("bootstrapping finished with %s as the last accepted block", lastAcceptedID)
	return nil
}

// Gossip implements the Engine interface
func (t *Transitive) Gossip() error {
	blkID := t.VM.LastAccepted()
	blk, err := t.VM.GetBlock(blkID)
	if err != nil {
		t.Context().Log.Warn("dropping gossip request as %s couldn't be loaded due to %s", blkID, err)
		return nil
	}

	t.Context().Log.Verbo("gossiping %s as accepted to the network", blkID)
	t.Sender.Gossip(blkID, blk.Bytes())
	return nil
}

// Shutdown implements the Engine interface
func (t *Transitive) Shutdown() error {
	t.Context().Log.Info("shutting down consensus engine")
	return t.VM.Shutdown()
}

// Context implements the Engine interface
func (t *Transitive) Context() *snow.Context { return t.Config.Context }

// Get implements the Engine interface
func (t *Transitive) Get(vdr ids.ShortID, requestID uint32, blkID ids.ID) error {
	blk, err := t.VM.GetBlock(blkID)
	if err != nil {
		// If we failed to get the block, that means either an unexpected error
		// has occurred, the validator is not following the protocol, or the
		// block has been pruned.
		t.Context().Log.Debug("Get(%s, %d, %s) failed with: %s", vdr, requestID, blkID, err)
		return nil
	}

	// Respond to the validator with the fetched block and the same requestID.
	t.Sender.Put(vdr, requestID, blkID, blk.Bytes())
	return nil
}

// GetAncestors implements the Engine interface
func (t *Transitive) GetAncestors(vdr ids.ShortID, requestID uint32, blkID ids.ID) error {
	startTime := time.Now()
	blk, err := t.VM.GetBlock(blkID)
	if err != nil { // Don't have the block. Drop this request.
		t.Context().Log.Verbo("couldn't get block %s. dropping GetAncestors(%s, %d, %s)", blkID, vdr, requestID, blkID)
		return nil
	}

	ancestorsBytes := make([][]byte, 1, common.MaxContainersPerMultiPut) // First elt is byte repr. of blk, then its parents, then grandparent, etc.
	ancestorsBytes[0] = blk.Bytes()
	ancestorsBytesLen := len(blk.Bytes()) + wrappers.IntLen // length, in bytes, of all elements of ancestors

	for numFetched := 1; numFetched < common.MaxContainersPerMultiPut && time.Since(startTime) < common.MaxTimeFetchingAncestors; numFetched++ {
		blk = blk.Parent()
		if blk.Status() == choices.Unknown {
			break
		}
		blkBytes := blk.Bytes()
		// Ensure response size isn't too large. Include wrappers.IntLen because the size of the message
		// is included with each container, and the size is repr. by an int.
		if newLen := wrappers.IntLen + ancestorsBytesLen + len(blkBytes); newLen < maxContainersLen {
			ancestorsBytes = append(ancestorsBytes, blkBytes)
			ancestorsBytesLen = newLen
		} else { // reached maximum response size
			break
		}
	}

	t.Sender.MultiPut(vdr, requestID, ancestorsBytes)
	return nil
}

// Put implements the Engine interface
func (t *Transitive) Put(vdr ids.ShortID, requestID uint32, blkID ids.ID, blkBytes []byte) error {
	// bootstrapping isn't done --> we didn't send any gets --> this put is invalid
	if !t.Context().IsBootstrapped() {
		if requestID == network.GossipMsgRequestID {
			t.Context().Log.Verbo("dropping gossip Put(%s, %d, %s) due to bootstrapping", vdr, requestID, blkID)
		} else {
			t.Context().Log.Debug("dropping Put(%s, %d, %s) due to bootstrapping", vdr, requestID, blkID)
		}
		return nil
	}

	blk, err := t.VM.ParseBlock(blkBytes)
	if err != nil {
		t.Context().Log.Debug("failed to parse block %s: %s", blkID, err)
		t.Context().Log.Verbo("block:\n%s", formatting.DumpBytes{Bytes: blkBytes})
		// because GetFailed doesn't utilize the assumption that we actually
		// sent a Get message, we can safely call GetFailed here to potentially
		// abandon the request.
		return t.GetFailed(vdr, requestID)
	}

	// insert the block into consensus. If the block has already been issued,
	// this will be a noop. If this block has missing dependencies, vdr will
	// receive requests to fill the ancestry. dependencies that have already
	// been fetched, but with missing dependencies themselves won't be requested
	// from the vdr.
	_, err = t.insertFrom(vdr, blk)
	return err
}

// GetFailed implements the Engine interface
func (t *Transitive) GetFailed(vdr ids.ShortID, requestID uint32) error {
	// not done bootstrapping --> didn't send a get --> this message is invalid
	if !t.Context().IsBootstrapped() {
		t.Context().Log.Debug("dropping GetFailed(%s, %d) due to bootstrapping")
		return nil
	}

	// we don't use the assumption that this function is called after a failed
	// Get message. So we first check to see if we have an outstanding request
	// and also get what the request was for if it exists
	blkID, ok := t.blkReqs.Remove(vdr, requestID)
	if !ok {
		t.Context().Log.Debug("getFailed(%s, %d) called without having sent corresponding Get", vdr, requestID)
		return nil
	}

	// because the get request was dropped, we no longer are expected blkID to
	// be issued.
	t.blocked.Abandon(blkID)
	return t.errs.Err
}

// PullQuery implements the Engine interface
func (t *Transitive) PullQuery(vdr ids.ShortID, requestID uint32, blkID ids.ID) error {
	// if the engine hasn't been bootstrapped, we aren't ready to respond to
	// queries
	if !t.Context().IsBootstrapped() {
		t.Context().Log.Debug("dropping PullQuery(%s, %d, %s) due to bootstrapping", vdr, requestID, blkID)
		return nil
	}

	// Will send chits once we have all the dependencies for block [blkID]
	c := &convincer{
		consensus: t.consensus,
		sender:    t.Sender,
		vdr:       vdr,
		requestID: requestID,
		errs:      &t.errs,
	}

	added, err := t.fetchOrInsert(vdr, blkID)
	if err != nil {
		return err
	}

	// if we aren't able to have issued this block, then it is a dependency for
	// this reply
	if !added {
		c.deps.Add(blkID)
	}

	t.blocked.Register(c)
	return t.errs.Err
}

// PushQuery implements the Engine interface
func (t *Transitive) PushQuery(vdr ids.ShortID, requestID uint32, blkID ids.ID, blkBytes []byte) error {
	// if the engine hasn't been bootstrapped, we aren't ready to respond to
	// queries
	if !t.Context().IsBootstrapped() {
		t.Context().Log.Debug("dropping PushQuery(%s, %d, %s) due to bootstrapping", vdr, requestID, blkID)
		return nil
	}

	blk, err := t.VM.ParseBlock(blkBytes)
	// If the parsing fails, we just drop the request, as we didn't ask for it
	if err != nil {
		t.Context().Log.Debug("failed to parse block %s: %s", blkID, err)
		t.Context().Log.Verbo("block:\n%s", formatting.DumpBytes{Bytes: blkBytes})
		return nil
	}

	// insert the block into consensus. If the block has already been issued,
	// this will be a noop. If this block has missing dependencies, vdr will
	// receive requests to fill the ancestry. dependencies that have already
	// been fetched, but with missing dependencies themselves won't be requested
	// from the vdr.
	if _, err := t.insertFrom(vdr, blk); err != nil {
		return err
	}

	// register the chit request
	return t.PullQuery(vdr, requestID, blk.ID())
}

// Chits implements the Engine interface
func (t *Transitive) Chits(vdr ids.ShortID, requestID uint32, votes ids.Set) error {
	// if the engine hasn't been bootstrapped, we shouldn't be receiving chits
	if !t.Context().IsBootstrapped() {
		t.Context().Log.Debug("dropping Chits(%s, %d) due to bootstrapping", vdr, requestID)
		return nil
	}

	// Since this is snowman, there should only be one ID in the vote set
	if votes.Len() != 1 {
		t.Context().Log.Debug("Chits(%s, %d) was called with %d votes (expected 1)", vdr, requestID, votes.Len())
		// because QueryFailed doesn't utilize the assumption that we actually
		// sent a Query message, we can safely call QueryFailed here to
		// potentially abandon the request.
		return t.QueryFailed(vdr, requestID)
	}
	vote := votes.List()[0]

	t.Context().Log.Verbo("Chits(%s, %d) contains vote for %s", vdr, requestID, vote)

	v := &voter{
		t:         t,
		vdr:       vdr,
		requestID: requestID,
		response:  vote,
	}

	added, err := t.fetchOrInsert(vdr, vote)
	if err != nil {
		return err
	}

	// if we aren't able to have issued the vote's block, then it is a
	// dependency for applying the vote
	if !added {
		v.deps.Add(vote)
	}

	t.blocked.Register(v)
	return t.errs.Err
}

// QueryFailed implements the Engine interface
func (t *Transitive) QueryFailed(vdr ids.ShortID, requestID uint32) error {
	// if the engine hasn't been bootstrapped, we won't have sent a query
	if !t.Context().IsBootstrapped() {
		t.Context().Log.Warn("dropping QueryFailed(%s, %d) due to bootstrapping", vdr, requestID)
		return nil
	}

	t.blocked.Register(&voter{
		t:         t,
		vdr:       vdr,
		requestID: requestID,
	})
	return t.errs.Err
}

// Notify implements the Engine interface
func (t *Transitive) Notify(msg common.Message) error {
	// if the engine hasn't been bootstrapped, we shouldn't issuing blocks
	if !t.Context().IsBootstrapped() {
		t.Context().Log.Debug("dropping Notify due to bootstrapping")
		return nil
	}

	t.Context().Log.Verbo("snowman engine notified of %s from the vm", msg)
	switch msg {
	case common.PendingTxs:
		// the pending txs message means we should attempt to build a block.
		blk, err := t.VM.BuildBlock()
		if err != nil {
			t.Context().Log.Debug("VM.BuildBlock errored with: %s", err)
			return nil
		}

		// a newly created block is expected to be processing. If this check
		// fails, there is potentially an error in the VM this engine is running
		if status := blk.Status(); status != choices.Processing {
			t.Context().Log.Warn("attempting to issue a block with status: %s, expected Processing", status)
		}

		// the newly created block should be built on top of the preferred
		// block. Otherwise, the new block doesn't have the best chance of being
		// confirmed.
		parentID := blk.Parent().ID()
		if pref := t.consensus.Preference(); !parentID.Equals(pref) {
			t.Context().Log.Warn("built block with parent: %s, expected %s", parentID, pref)
		}

		added, err := t.insertAll(blk)
		if err != nil {
			return err
		}

		// inserting the block shouldn't have any missing dependencies
		if added {
			t.Context().Log.Verbo("successfully issued new block from the VM")
		} else {
			t.Context().Log.Warn("VM.BuildBlock returned a block that is pending for ancestors")
		}
	default:
		t.Context().Log.Warn("unexpected message from the VM: %s", msg)
	}
	return nil
}

func (t *Transitive) repoll() {
	// if we are issuing a repoll, we should gossip our current preferences to
	// propagate the most likely branch as quickly as possible
	prefID := t.consensus.Preference()

	for i := t.polls.Len(); i < t.params.ConcurrentRepolls; i++ {
		t.pullSample(prefID)
	}
}

// fetchOrInsert attempts to issue the branch ending with a block [blkID] into consensus.
// If we do not have [blkID], request it.
// Returns true if the block was issued, now or previously, to consensus.
func (t *Transitive) fetchOrInsert(vdr ids.ShortID, blkID ids.ID) (bool, error) {
	blk, err := t.VM.GetBlock(blkID)
	if err != nil {
		t.sendRequest(vdr, blkID)
		return false, nil
	}
	return t.insertFrom(vdr, blk)
}

// insertFrom attempts to issue the branch ending with block [blkID] to consensus.
// Returns true if the block was issued, now or previously, to consensus.
// This is useful to check the local DB before requesting a block in case we
// have the block for some reason. If a dependency is missing, the validator
// will be sent a Get message.
func (t *Transitive) insertFrom(vdr ids.ShortID, blk snowman.Block) (bool, error) {
	blkID := blk.ID()
	// Issue [blk] and its ancestors to consensus.
	// If the block has been issued, we don't need to insert it.
	// If the block is queued to be issued, we don't need to insert it.
	for !t.consensus.Issued(blk) && !t.pending.Contains(blkID) {
		if err := t.insert(blk); err != nil {
			return false, err
		}

		blk = blk.Parent()
		blkID = blk.ID()

		// If we don't have this ancestor, request it from [vdr]
		if !blk.Status().Fetched() {
			t.sendRequest(vdr, blkID)
			return false, nil
		}
	}
	return t.consensus.Issued(blk), nil
}

// insertAll attempts to issue the branch ending with [blk] to consensus.
// Returns true if [blk] was issued, now or previously, to consensus.
// This is useful to check the local DB before requesting a block in case we don't
// have the block for some reason. If a dependency is missing and the dependency
// hasn't been requested, the issuance will be abandoned.
func (t *Transitive) insertAll(blk snowman.Block) (bool, error) {
	blkID := blk.ID()
	// Insert all of [blk]'s ancestors into consensus
	for blk.Status().Fetched() && !t.consensus.Issued(blk) && !t.pending.Contains(blkID) {
		if err := t.insert(blk); err != nil {
			return false, err
		}

		blk = blk.Parent()
		blkID = blk.ID()
	}

	// if issuance the block was successful, this is the happy path
	if t.consensus.Issued(blk) {
		return true, nil
	}

	// if this branch is waiting on a block that we supposedly have a source of,
	// we can just wait for that request to succeed or fail
	if t.blkReqs.Contains(blkID) {
		return false, nil
	}

	// if we have no reason to expect that this block will be inserted, we
	// should abandon the block to avoid a memory leak
	t.blocked.Abandon(blkID)
	return false, t.errs.Err
}

// Attempt to insert [blk] to consensus once its parent has been issued or abandoned.
func (t *Transitive) insert(blk snowman.Block) error {
	blkID := blk.ID()

	// mark that the block has been fetched but is pending
	t.pending.Add(blkID)

	// if we have any outstanding requests for this block, remove the pending requests
	t.blkReqs.RemoveAny(blkID)

	i := &issuer{
		t:   t,
		blk: blk,
	}

	// block on the parent if needed
	if parent := blk.Parent(); !t.consensus.Issued(parent) {
		parentID := parent.ID()
		t.Context().Log.Verbo("block %s waiting for parent %s", blkID, parentID)
		i.deps.Add(parentID)
	}

	t.blocked.Register(i)

	// Tracks performance statistics
	t.numRequests.Set(float64(t.blkReqs.Len()))
	t.numBlocked.Set(float64(t.pending.Len()))
	return t.errs.Err
}

func (t *Transitive) sendRequest(vdr ids.ShortID, blkID ids.ID) {
	// only send one request at a time for a block
	if t.blkReqs.Contains(blkID) {
		return
	}

	t.RequestID++
	t.blkReqs.Add(vdr, t.RequestID, blkID)
	t.Context().Log.Verbo("sending Get(%s, %d, %s)", vdr, t.RequestID, blkID)
	t.Sender.Get(vdr, t.RequestID, blkID)

	// Tracks performance statistics
	t.numRequests.Set(float64(t.blkReqs.Len()))
}

// send a pull request for this block ID
func (t *Transitive) pullSample(blkID ids.ID) {
	t.Context().Log.Verbo("about to sample from: %s", t.Config.Validators)
	p := t.consensus.Parameters()
	vdrs := t.Config.Validators.Sample(p.K)
	vdrSet := ids.ShortSet{}
	for _, vdr := range vdrs {
		vdrSet.Add(vdr.ID())
	}

	toSample := ids.ShortSet{}
	toSample.Union(vdrSet)

	t.RequestID++
	if numVdrs := len(vdrs); numVdrs == p.K && t.polls.Add(t.RequestID, vdrSet) {
		t.Sender.PullQuery(toSample, t.RequestID, blkID)
	} else if numVdrs < p.K {
		t.Context().Log.Error("query for %s was dropped due to an insufficient number of validators", blkID)
	}
}

// send a push request for this block
func (t *Transitive) pushSample(blk snowman.Block) {
	t.Context().Log.Verbo("about to sample from: %s", t.Config.Validators)
	p := t.consensus.Parameters()
	vdrs := t.Config.Validators.Sample(p.K)
	vdrSet := ids.ShortSet{}
	for _, vdr := range vdrs {
		vdrSet.Add(vdr.ID())
	}

	toSample := ids.ShortSet{}
	toSample.Union(vdrSet)

	t.RequestID++
	if numVdrs := len(vdrs); numVdrs == p.K && t.polls.Add(t.RequestID, vdrSet) {
		t.Sender.PushQuery(toSample, t.RequestID, blk.ID(), blk.Bytes())
	} else if numVdrs < p.K {
		t.Context().Log.Error("query for %s was dropped due to an insufficient number of validators", blk.ID())
	}
}

func (t *Transitive) deliver(blk snowman.Block) error {
	if t.consensus.Issued(blk) {
		return nil
	}

	// we are adding the block to consensus, so it is no longer pending
	blkID := blk.ID()
	t.pending.Remove(blkID)

	if err := blk.Verify(); err != nil {
		t.Context().Log.Debug("block failed verification due to %s, dropping block", err)

		// if verify fails, then all decedents are also invalid
		t.blocked.Abandon(blkID)
		t.numBlocked.Set(float64(t.pending.Len())) // Tracks performance statistics
		return t.errs.Err
	}

	t.Context().Log.Verbo("adding block to consensus: %s", blkID)
	t.consensus.Add(blk)

	// Add all the oracle blocks if they exist. We call verify on all the blocks
	// and add them to consensus before marking anything as fulfilled to avoid
	// any potential reentrant bugs.
	added := []snowman.Block{}
	dropped := []snowman.Block{}
	switch blk := blk.(type) {
	case OracleBlock:
		options, err := blk.Options()
		if err != nil {
			return err
		}
		for _, blk := range options {
			if err := blk.Verify(); err != nil {
				t.Context().Log.Debug("block failed verification due to %s, dropping block", err)
				dropped = append(dropped, blk)
			} else {
				t.consensus.Add(blk)
				added = append(added, blk)
			}
		}
	}

	t.VM.SetPreference(t.consensus.Preference())

	// launch a query for the newly added block
	t.pushSample(blk)

	t.blocked.Fulfill(blkID)
	for _, blk := range added {
		t.pushSample(blk)

		blkID := blk.ID()
		t.pending.Remove(blkID)
		t.blocked.Fulfill(blkID)
	}
	for _, blk := range dropped {
		blkID := blk.ID()
		t.pending.Remove(blkID)
		t.blocked.Abandon(blkID)
	}

	// If we should issue multiple queries at the same time, we need to repoll
	t.repoll()

	// Tracks performance statistics
	t.numRequests.Set(float64(t.blkReqs.Len()))
	t.numBlocked.Set(float64(t.pending.Len()))
	return t.errs.Err
}

// IsBootstrapped returns true iff this chain is done bootstrapping
func (t *Transitive) IsBootstrapped() bool {
	return t.Context().IsBootstrapped()
}
