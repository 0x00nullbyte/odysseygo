// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snowman

import (
	"github.com/ava-labs/gecko/ids"
	"github.com/ava-labs/gecko/snow"
	"github.com/ava-labs/gecko/snow/consensus/snowball"
)

// TopologicalFactory implements Factory by returning a topological struct
type TopologicalFactory struct{}

// New implements Factory
func (TopologicalFactory) New() Consensus { return &Topological{} }

// Topological implements the Snowman interface by using a tree tracking the
// strongly preferred branch. This tree structure amortizes network polls to
// vote on more than just the next block.
type Topological struct {
	metrics

	ctx    *snow.Context
	params snowball.Parameters

	head   ids.ID
	blocks map[[32]byte]*snowmanBlock // ParentID -> Snowball instance
	tail   ids.ID
}

// Used to track the kahn topological sort status
type kahnNode struct {
	// inDegree is the number of children that haven't been processed yet. If
	// inDegree is 0, then this node is a leaf
	inDegree int
	// votes for all the children of this node, so far
	votes ids.Bag
}

// Used to track which children should receive votes
type votes struct {
	// parentID is the parent of all the votes provided in the votes bag
	parentID ids.ID
	// votes for all the children of the parent
	votes ids.Bag
}

// Initialize implements the Snowman interface
func (ts *Topological) Initialize(ctx *snow.Context, params snowball.Parameters, rootID ids.ID) {
	ts.ctx = ctx
	ts.params = params

	if err := ts.metrics.Initialize(ctx.Log, params.Namespace, params.Metrics); err != nil {
		ts.ctx.Log.Error("%s", err)
	}

	ts.head = rootID
	ts.blocks = map[[32]byte]*snowmanBlock{
		rootID.Key(): &snowmanBlock{
			sm: ts,
		},
	}
	ts.tail = rootID
}

// Parameters implements the Snowman interface
func (ts *Topological) Parameters() snowball.Parameters { return ts.params }

// Add implements the Snowman interface
func (ts *Topological) Add(blk Block) {
	parent := blk.Parent()
	parentID := parent.ID()
	parentKey := parentID.Key()

	blkID := blk.ID()
	blkBytes := blk.Bytes()

	// Notify anyone listening that this block was issued.
	ts.ctx.DecisionDispatcher.Issue(ts.ctx.ChainID, blkID, blkBytes)
	ts.ctx.ConsensusDispatcher.Issue(ts.ctx.ChainID, blkID, blkBytes)
	ts.metrics.Issued(blkID)

	parentNode, ok := ts.blocks[parentKey]
	if !ok {
		// If the ancestor is missing, this means the ancestor must have already
		// been pruned. Therefore, the dependent should be transitively
		// rejected.
		blk.Reject()

		// Notify anyone listening that this block was rejected.
		ts.ctx.DecisionDispatcher.Reject(ts.ctx.ChainID, blkID, blkBytes)
		ts.ctx.ConsensusDispatcher.Reject(ts.ctx.ChainID, blkID, blkBytes)
		ts.metrics.Rejected(blkID)
		return
	}

	parentNode.AddChild(blk)

	ts.blocks[blkID.Key()] = &snowmanBlock{
		sm:  ts,
		blk: blk,
	}

	// If we are extending the tail, this is the new tail
	if ts.tail.Equals(parentID) {
		ts.tail = blkID
	}
}

// Issued implements the Snowman interface
func (ts *Topological) Issued(blk Block) bool {
	// If the block is decided, then it must have been previously issued.
	if blk.Status().Decided() {
		return true
	}
	// If the block is in the map of current blocks, then the block was issued.
	_, ok := ts.blocks[blk.ID().Key()]
	return ok
}

// Preference implements the Snowman interface
func (ts *Topological) Preference() ids.ID { return ts.tail }

// RecordPoll implements the Snowman interface
// This performs Kahn’s algorithm.
// When a node is removed from the leaf queue, it is checked to see if the
// number of votes is >= alpha. If it is, then it is added to the vote stack.
// Once there are no nodes in the leaf queue. The vote stack is unwound and
// voted on. If a decision is made, then that choice is marked as accepted, and
// all alternative choices are marked as rejected.
// The complexity of this function is:
// Runtime = 3 * |live set| + |votes|
// Space = |live set| + |votes|
func (ts *Topological) RecordPoll(votes ids.Bag) {
	// Runtime = |live set| + |votes| ; Space = |live set| + |votes|
	kahnGraph, leaves := ts.calculateInDegree(votes)

	// Runtime = |live set| ; Space = |live set|
	voteStack := ts.pushVotes(kahnGraph, leaves)

	// Runtime = |live set| ; Space = Constant
	preferred := ts.vote(voteStack)

	// Runtime = |live set| ; Space = Constant
	ts.tail = ts.getPreferredDecendent(preferred)
}

// Finalized implements the Snowman interface
func (ts *Topological) Finalized() bool { return len(ts.blocks) == 1 }

// takes in a list of votes and sets up the topological ordering. Returns the
// reachable section of the graph annotated with the number of inbound edges and
// the non-transitively applied votes. Also returns the list of leaf nodes.
func (ts *Topological) calculateInDegree(
	votes ids.Bag) (map[[32]byte]kahnNode, []ids.ID) {
	kahns := make(map[[32]byte]kahnNode)
	leaves := ids.Set{}

	for _, vote := range votes.List() {
		voteNode, validVote := ts.blocks[vote.Key()]
		// If it is not found, then the vote is either for something rejected,
		// or something we haven't heard of yet.
		if validVote && voteNode.blk != nil && !voteNode.blk.Status().Decided() {
			parentID := voteNode.blk.Parent().ID()
			parentKey := parentID.Key()
			kahn, previouslySeen := kahns[parentKey]
			// Add this new vote to the current bag of votes
			kahn.votes.AddCount(vote, votes.Count(vote))
			kahns[parentKey] = kahn

			if !previouslySeen {
				// If I've never seen this node before, it is currently a leaf.
				leaves.Add(parentID)

				for n, e := ts.blocks[parentKey]; e; n, e = ts.blocks[parentKey] {
					if n.blk == nil || n.blk.Status().Decided() {
						break // Ensure that we haven't traversed off the tree
					}
					parentID := n.blk.Parent().ID()
					parentKey = parentID.Key()

					kahn := kahns[parentKey]
					kahn.inDegree++
					kahns[parentKey] = kahn

					if kahn.inDegree == 1 {
						// If I am transitively seeing this node for the first
						// time, it is no longer a leaf.
						leaves.Remove(parentID)
					} else {
						// If I have already traversed this branch, stop.
						break
					}
				}
			}
		}
	}

	return kahns, leaves.List()
}

// convert the tree into a branch of snowball instances with an alpha threshold
func (ts *Topological) pushVotes(
	kahnNodes map[[32]byte]kahnNode, leaves []ids.ID) []votes {
	voteStack := []votes(nil)
	for len(leaves) > 0 {
		newLeavesSize := len(leaves) - 1
		leaf := leaves[newLeavesSize]
		leaves = leaves[:newLeavesSize]

		leafKey := leaf.Key()
		kahn := kahnNodes[leafKey]

		if node, shouldVote := ts.blocks[leafKey]; shouldVote {
			if kahn.votes.Len() >= ts.params.Alpha {
				voteStack = append(voteStack, votes{
					parentID: leaf,
					votes:    kahn.votes,
				})
			}

			if node.blk == nil || node.blk.Status().Decided() {
				continue // Stop traversing once we pass into the decided frontier
			}

			parentID := node.blk.Parent().ID()
			parentKey := parentID.Key()
			if depNode, notPruned := kahnNodes[parentKey]; notPruned {
				// Remove one of the in-bound edges
				depNode.inDegree--
				// Push the votes to my parent
				depNode.votes.AddCount(leaf, kahn.votes.Len())
				kahnNodes[parentKey] = depNode

				if depNode.inDegree == 0 {
					// Once I have no in-bound edges, I'm a leaf
					leaves = append(leaves, parentID)
				}
			}
		}
	}
	return voteStack
}

func (ts *Topological) vote(voteStack []votes) ids.ID {
	if len(voteStack) == 0 {
		headKey := ts.head.Key()
		headNode := ts.blocks[headKey]
		headNode.shouldFalter = true

		ts.ctx.Log.Verbo("No progress was made on this vote even though we have %d pending blocks", len(ts.blocks)-1)
		return ts.tail
	}

	onTail := true
	tail := ts.head
	for len(voteStack) > 0 {
		newStackSize := len(voteStack) - 1
		voteGroup := voteStack[newStackSize]
		voteStack = voteStack[:newStackSize]

		voteParentKey := voteGroup.parentID.Key()
		parentNode, stillExists := ts.blocks[voteParentKey]
		if !stillExists {
			break
		}

		shouldTransFalter := parentNode.shouldFalter
		if parentNode.shouldFalter {
			parentNode.sb.RecordUnsuccessfulPoll()
			parentNode.shouldFalter = false
			ts.ctx.Log.Verbo("Reset confidence below %s", voteGroup.parentID)
		}
		parentNode.sb.RecordPoll(voteGroup.votes)

		// Only accept when you are finalized and the head.
		if parentNode.sb.Finalized() && ts.head.Equals(voteGroup.parentID) {
			ts.accept(parentNode)
			tail = parentNode.sb.Preference()
			delete(ts.blocks, voteParentKey)
		}

		// If this is the last id that got votes, default to the empty id. This
		// will cause all my children to be reset below.
		nextID := ids.ID{}
		if len(voteStack) > 0 {
			nextID = voteStack[newStackSize-1].parentID
		}

		onTail = onTail && nextID.Equals(parentNode.sb.Preference())
		if onTail {
			tail = nextID
		}

		// If there wasn't an alpha threshold on the branch (either on this vote
		// or a past transitive vote), I should falter now.
		for childIDBytes := range parentNode.children {
			if childID := ids.NewID(childIDBytes); shouldTransFalter || !childID.Equals(nextID) {
				if childNode, childExists := ts.blocks[childIDBytes]; childExists {
					// The existence check is needed in case the current node
					// was finalized. However, in this case, we still need to
					// check for the next id.
					ts.ctx.Log.Verbo("Defering confidence reset below %s with %d children. NextID: %s", childID, len(parentNode.children), nextID)
					childNode.shouldFalter = true
				}
			}
		}
	}
	return tail
}

// Get the preferred decendent of the provided block ID
func (ts *Topological) getPreferredDecendent(blkID ids.ID) ids.ID {
	// Traverse from the provided ID to the preferred child until there are no
	// children.
	for block := ts.blocks[blkID.Key()]; block.sb != nil; block = ts.blocks[blkID.Key()] {
		blkID = block.sb.Preference()
	}
	return blkID
}

func (ts *Topological) accept(n *snowmanBlock) {
	// Accept the preference, reject all transitive rejections
	pref := n.sb.Preference()

	rejects := []ids.ID(nil)
	for childIDBytes := range n.children {
		if childID := ids.NewID(childIDBytes); !childID.Equals(pref) {
			child := n.children[childIDBytes]
			child.Reject()

			bytes := child.Bytes()
			ts.ctx.DecisionDispatcher.Reject(ts.ctx.ChainID, childID, bytes)
			ts.ctx.ConsensusDispatcher.Reject(ts.ctx.ChainID, childID, bytes)
			ts.metrics.Rejected(childID)

			rejects = append(rejects, childID)
		}
	}
	ts.rejectTransitively(rejects...)

	ts.head = pref
	child := n.children[pref.Key()]
	ts.ctx.Log.Verbo("Accepting block with ID %s", child.ID())

	bytes := child.Bytes()
	ts.ctx.DecisionDispatcher.Accept(ts.ctx.ChainID, child.ID(), bytes)
	ts.ctx.ConsensusDispatcher.Accept(ts.ctx.ChainID, child.ID(), bytes)

	child.Accept()
	ts.metrics.Accepted(pref)
}

// Takes in a list of newly rejected ids and rejects everything that depends on
// them
func (ts *Topological) rejectTransitively(rejected ...ids.ID) {
	for len(rejected) > 0 {
		newRejectedSize := len(rejected) - 1
		rejectID := rejected[newRejectedSize]
		rejected = rejected[:newRejectedSize]

		rejectKey := rejectID.Key()
		rejectNode := ts.blocks[rejectKey]
		delete(ts.blocks, rejectKey)

		for childIDBytes, child := range rejectNode.children {
			childID := ids.NewID(childIDBytes)
			rejected = append(rejected, childID)
			child.Reject()

			bytes := child.Bytes()
			ts.ctx.DecisionDispatcher.Reject(ts.ctx.ChainID, childID, bytes)
			ts.ctx.ConsensusDispatcher.Reject(ts.ctx.ChainID, childID, bytes)
			ts.metrics.Rejected(childID)
		}
	}
}
