// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package sender

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/message"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/networking/router"
	"github.com/ava-labs/avalanchego/snow/networking/timeout"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/prometheus/client_golang/prometheus"
)

// Sender is a wrapper around an ExternalSender.
// Messages to this node are put directly into [router] rather than
// being sent over the network via the wrapped ExternalSender.
// Sender registers outbound requests with [router] so that [router]
// fires a timeout if we don't get a response to the request.
type Sender struct {
	ctx        *snow.Context
	msgCreator message.Creator
	sender     ExternalSender // Actually does the sending over the network
	router     router.Router
	timeouts   *timeout.Manager

	// Request message type --> Counts how many of that request
	// have failed because the node was benched
	failedDueToBench map[message.Op]prometheus.Counter
}

// Initialize this sender
func (s *Sender) Initialize(
	ctx *snow.Context,
	msgCreator message.Creator,
	sender ExternalSender,
	router router.Router,
	timeouts *timeout.Manager,
	metricsNamespace string,
	metricsRegisterer prometheus.Registerer,
) error {
	s.ctx = ctx
	s.msgCreator = msgCreator
	s.sender = sender
	s.router = router
	s.timeouts = timeouts

	// Register metrics
	// Message type --> String representation for metrics
	requestTypes := map[message.Op]string{
		message.Get:                 "get",
		message.GetAccepted:         "get_accepted",
		message.GetAcceptedFrontier: "get_accepted_frontier",
		message.GetAncestors:        "get_ancestors",
		message.PullQuery:           "pull_query",
		message.PushQuery:           "push_query",
		message.AppRequest:          "app_request",
	}

	s.failedDueToBench = make(map[message.Op]prometheus.Counter, len(requestTypes))

	for msgType, asStr := range requestTypes {
		counter := prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: metricsNamespace,
				Name:      fmt.Sprintf("%s_failed_benched", asStr),
				Help:      fmt.Sprintf("# of times a %s request was not sent because the node was benched", asStr),
			},
		)
		if err := metricsRegisterer.Register(counter); err != nil {
			return fmt.Errorf("couldn't register metric for %s: %w", msgType, err)
		}
		s.failedDueToBench[msgType] = counter
	}
	return nil
}

// Context of this sender
func (s *Sender) Context() *snow.Context { return s.ctx }

func (s *Sender) SendGetAcceptedFrontier(nodeIDs ids.ShortSet, requestID uint32) {
	// Sending a message to myself. No need to send it over the network.
	// Just put it right into the router. Asynchronously to avoid deadlock.
	if nodeIDs.Contains(s.ctx.NodeID) {
		nodeIDs.Remove(s.ctx.NodeID)
		// Note that this timeout duration won't exactly match the one that gets registered. That's OK.
		timeoutDuration := s.timeouts.TimeoutDuration()
		deadline := uint64(time.Now().Add(timeoutDuration).Unix())

		inMsg := s.msgCreator.InboundGetAcceptedFrontier(s.ctx.ChainID, requestID, deadline, s.ctx.NodeID)

		// Tell the router to expect a reply message from this node
		s.router.RegisterRequest(s.ctx.NodeID, s.ctx.ChainID, requestID, message.GetAcceptedFrontier)
		go s.router.HandleInbound(inMsg)
	}

	// Try to send the messages over the network.
	// Note that this timeout duration won't exactly match the one that gets
	// registered. That's OK.
	deadline := uint64(s.timeouts.TimeoutDuration())
	outMsg, err := s.msgCreator.GetAcceptedFrontier(s.ctx.ChainID, requestID, deadline)
	s.ctx.Log.AssertNoError(err)
	sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false)

	// Tell the router to expect a reply message from these validators.
	// We register timeouts for all validators, regardless of if the message
	// failed to be sent, to avoid busy looping when disconnected from the
	// internet
	for nodeID := range nodeIDs {
		if !sentTo.Contains(nodeID) {
			s.ctx.Log.Debug("failed to send GetAcceptedFrontier(%s, %s, %d)",
				nodeID, s.ctx.ChainID, requestID)
		}
		s.router.RegisterRequest(nodeID, s.ctx.ChainID, requestID, message.GetAcceptedFrontier)
	}
}

func (s *Sender) SendAcceptedFrontier(nodeID ids.ShortID, requestID uint32, containerIDs []ids.ID) {
	if nodeID == s.ctx.NodeID {
		inMsg := s.msgCreator.InboundAcceptedFrontier(s.ctx.ChainID, requestID, containerIDs, nodeID)
		go s.router.HandleInbound(inMsg)
		return
	}

	outMsg, err := s.msgCreator.AcceptedFrontier(s.ctx.ChainID, requestID, containerIDs)
	if err != nil {
		s.ctx.Log.Error("failed to build AcceptedFrontier(%s, %d, %s): %s",
			s.ctx.ChainID,
			requestID,
			containerIDs,
			err)
		return
	}

	nodeIDs := ids.NewShortSet(1)
	nodeIDs.Add(nodeID)
	if sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false); sentTo.Len() == 0 {
		s.ctx.Log.Debug("failed to send AcceptedFrontier(%s, %s, %d, %s)",
			nodeID,
			s.ctx.ChainID,
			requestID,
			containerIDs)
	}
}

func (s *Sender) SendGetAccepted(nodeIDs ids.ShortSet, requestID uint32, containerIDs []ids.ID) {
	// Sending a message to myself. No need to send it over the network.
	// Just put it right into the router. Asynchronously to avoid deadlock.
	if nodeIDs.Contains(s.ctx.NodeID) {
		nodeIDs.Remove(s.ctx.NodeID)

		// Note that this timeout duration won't exactly match the one that gets registered. That's OK.
		timeoutDuration := s.timeouts.TimeoutDuration()
		deadline := uint64(time.Now().Add(timeoutDuration).Unix())

		inMsg := s.msgCreator.InboundGetAccepted(s.ctx.ChainID, requestID, deadline, containerIDs, s.ctx.NodeID)

		s.router.RegisterRequest(s.ctx.NodeID, s.ctx.ChainID, requestID, message.GetAccepted)
		go s.router.HandleInbound(inMsg)
	}

	// Try to send the messages over the network.
	// Note that this timeout duration won't exactly match the one that gets
	// registered. That's OK.
	deadline := uint64(s.timeouts.TimeoutDuration())
	outMsg, err := s.msgCreator.GetAccepted(s.ctx.ChainID, requestID, deadline, containerIDs)
	if err != nil {
		s.ctx.Log.Error("failed to build GetAccepted(%s, %d, %s): %s",
			s.ctx.ChainID,
			requestID,
			containerIDs,
			err)

		// duly register the failure
		for nodeID := range nodeIDs {
			inMsg := s.msgCreator.InternalGetAcceptedFailed(nodeID, s.ctx.ChainID, requestID)
			go s.router.HandleInbound(inMsg)
		}
		return
	}

	sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false)

	// Tell the router to expect a reply message from these validators
	// We register timeouts for all validators, regardless of if the message
	// failed to be sent, to avoid busy looping when disconnected from the
	// internet
	for nodeID := range nodeIDs {
		if !sentTo.Contains(nodeID) {
			s.ctx.Log.Debug("failed to send GetAccepted(%s, %s, %d, %s)",
				nodeID,
				s.ctx.ChainID,
				requestID,
				containerIDs)
		}
		s.router.RegisterRequest(nodeID, s.ctx.ChainID, requestID, message.GetAccepted)
	}
}

func (s *Sender) SendAccepted(nodeID ids.ShortID, requestID uint32, containerIDs []ids.ID) {
	if nodeID == s.ctx.NodeID {
		inMsg := s.msgCreator.InboundAccepted(s.ctx.ChainID, requestID, containerIDs, nodeID)
		go s.router.HandleInbound(inMsg)
		return
	}

	outMsg, err := s.msgCreator.Accepted(s.ctx.ChainID, requestID, containerIDs)
	if err != nil {
		s.ctx.Log.Error("failed to build Accepted(%s, %d, %s): %s",
			s.ctx.ChainID,
			requestID,
			containerIDs,
			err)
		return
	}

	nodeIDs := ids.NewShortSet(1)
	nodeIDs.Add(nodeID)
	if sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false); sentTo.Len() == 0 {
		s.ctx.Log.Debug("failed to send Accepted(%s, %s, %d, %s)",
			nodeID,
			s.ctx.ChainID,
			requestID,
			containerIDs)
	}
}

func (s *Sender) SendGetAncestors(nodeID ids.ShortID, requestID uint32, containerID ids.ID) {
	s.ctx.Log.Verbo("Sending GetAncestors to node %s. RequestID: %d. ContainerID: %s", nodeID.PrefixedString(constants.NodeIDPrefix), requestID, containerID)
	// Sending a GetAncestors to myself will always fail
	if nodeID == s.ctx.NodeID {
		inMsg := s.msgCreator.InternalGetAncestorsFailed(nodeID, s.ctx.ChainID, requestID)
		go s.router.HandleInbound(inMsg)
		return
	}

	// [nodeID] may be benched. That is, they've been unresponsive
	// so we don't even bother sending requests to them. We just have them immediately fail.
	if s.timeouts.IsBenched(nodeID, s.ctx.ChainID) {
		s.failedDueToBench[message.GetAncestors].Inc() // update metric
		s.timeouts.RegisterRequestToUnreachableValidator()
		inMsg := s.msgCreator.InternalGetAncestorsFailed(nodeID, s.ctx.ChainID, requestID)
		go s.router.HandleInbound(inMsg)
		return
	}

	// Note that this timeout duration won't exactly match the one that gets registered. That's OK.
	deadline := uint64(s.timeouts.TimeoutDuration())
	outMsg, err := s.msgCreator.GetAncestors(s.ctx.ChainID, requestID, deadline, containerID)
	if err != nil {
		s.ctx.Log.Error("failed to build GetAncestors message: %s", err)
		inMsg := s.msgCreator.InternalGetAncestorsFailed(nodeID, s.ctx.ChainID, requestID)
		go s.router.HandleInbound(inMsg)
		return
	}

	nodeIDs := ids.NewShortSet(1)
	nodeIDs.Add(nodeID)
	if sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false); sentTo.Len() == 0 {
		s.ctx.Log.Debug("failed to send GetAncestors(%s, %s, %d, %s)",
			nodeID,
			s.ctx.ChainID,
			requestID,
			containerID)
		s.timeouts.RegisterRequestToUnreachableValidator()
		inMsg := s.msgCreator.InternalGetAncestorsFailed(nodeID, s.ctx.ChainID, requestID)
		go s.router.HandleInbound(inMsg)
		return
	}

	// Tell the router to expect a reply message from this node
	s.router.RegisterRequest(nodeID, s.ctx.ChainID, requestID, message.GetAncestors)
}

// SendMultiPut sends a MultiPut message to the consensus engine running on the specified chain
// on the specified node.
// The MultiPut message gives the recipient the contents of several containers.
func (s *Sender) SendMultiPut(nodeID ids.ShortID, requestID uint32, containers [][]byte) {
	s.ctx.Log.Verbo("Sending MultiPut to node %s. RequestID: %d. NumContainers: %d", nodeID, requestID, len(containers))

	outMsg, err := s.msgCreator.MultiPut(s.ctx.ChainID, requestID, containers)
	if err != nil {
		s.ctx.Log.Error("failed to build MultiPut message because of container of size %d",
			len(containers))
		return
	}

	nodeIDs := ids.NewShortSet(1)
	nodeIDs.Add(nodeID)
	if sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false); sentTo.Len() == 0 {
		s.ctx.Log.Debug("failed to send MultiPut(%s, %s, %d, %d)",
			nodeID,
			s.ctx.ChainID,
			requestID,
			len(containers))
	}
}

// SendGet sends a Get message to the consensus engine running on the specified
// chain to the specified node. The Get message signifies that this
// consensus engine would like the recipient to send this consensus engine the
// specified container.
func (s *Sender) SendGet(nodeID ids.ShortID, requestID uint32, containerID ids.ID) {
	s.ctx.Log.Verbo("Sending Get to node %s. RequestID: %d. ContainerID: %s", nodeID.PrefixedString(constants.NodeIDPrefix), requestID, containerID)

	// Sending a Get to myself will always fail
	if nodeID == s.ctx.NodeID {
		inMsg := s.msgCreator.InternalGetFailed(nodeID, s.ctx.ChainID, requestID)
		go s.router.HandleInbound(inMsg)
		return
	}

	// [nodeID] may be benched. That is, they've been unresponsive
	// so we don't even bother sending requests to them. We just have them immediately fail.
	if s.timeouts.IsBenched(nodeID, s.ctx.ChainID) {
		s.failedDueToBench[message.Get].Inc() // update metric
		s.timeouts.RegisterRequestToUnreachableValidator()
		inMsg := s.msgCreator.InternalGetFailed(nodeID, s.ctx.ChainID, requestID)
		go s.router.HandleInbound(inMsg)
		return
	}

	// Note that this timeout duration won't exactly match the one that gets registered. That's OK.
	deadline := uint64(s.timeouts.TimeoutDuration())
	outMsg, err := s.msgCreator.Get(s.ctx.ChainID, requestID, deadline, containerID)
	s.ctx.Log.AssertNoError(err)

	nodeIDs := ids.NewShortSet(1)
	nodeIDs.Add(nodeID)
	if sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false); sentTo.Len() == 0 {
		s.ctx.Log.Debug("failed to send Get(%s, %s, %d, %s)",
			nodeID,
			s.ctx.ChainID,
			requestID,
			containerID)

		s.timeouts.RegisterRequestToUnreachableValidator()
		inMsg := s.msgCreator.InternalGetFailed(nodeID, s.ctx.ChainID, requestID)
		go s.router.HandleInbound(inMsg)
		return
	}

	// Tell the router to expect a reply message from this node
	s.router.RegisterRequest(nodeID, s.ctx.ChainID, requestID, message.Get)
}

// SendPut sends a Put message to the consensus engine running on the specified chain
// on the specified node.
// The Put message signifies that this consensus engine is giving to the recipient
// the contents of the specified container.
func (s *Sender) SendPut(nodeID ids.ShortID, requestID uint32, containerID ids.ID, container []byte) {
	s.ctx.Log.Verbo("Sending Put to node %s. RequestID: %d. ContainerID: %s", nodeID.PrefixedString(constants.NodeIDPrefix), requestID, containerID)

	outMsg, err := s.msgCreator.Put(s.ctx.ChainID, requestID, containerID, container)
	if err != nil {
		s.ctx.Log.Error("failed to build Put(%s, %d, %s): %s. len(container) : %d",
			s.ctx.ChainID,
			requestID,
			containerID,
			err,
			len(container))
		return
	}

	nodeIDs := ids.NewShortSet(1)
	nodeIDs.Add(nodeID)
	if sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false); sentTo.Len() == 0 {
		s.ctx.Log.Debug("failed to send Put(%s, %s, %d, %s)",
			nodeID,
			s.ctx.ChainID,
			requestID,
			containerID)
		s.ctx.Log.Verbo("container: %s", formatting.DumpBytes{Bytes: container})
	}
}

// SendPushQuery sends a PushQuery message to the consensus engines running on the specified chains
// on the specified nodes.
// The PushQuery message signifies that this consensus engine would like each node to send
// their preferred frontier given the existence of the specified container.
func (s *Sender) SendPushQuery(nodeIDs ids.ShortSet, requestID uint32, containerID ids.ID, container []byte) {
	s.ctx.Log.Verbo("Sending PushQuery to nodes %v. RequestID: %d. ContainerID: %s", nodeIDs, requestID, containerID)

	// Note that this timeout duration won't exactly match the one that gets registered. That's OK.
	timeoutDuration := s.timeouts.TimeoutDuration()

	// Sending a message to myself. No need to send it over the network.
	// Just put it right into the router. Do so asynchronously to avoid deadlock.
	if nodeIDs.Contains(s.ctx.NodeID) {
		nodeIDs.Remove(s.ctx.NodeID)

		timeoutDuration := s.timeouts.TimeoutDuration()
		deadline := uint64(time.Now().Add(timeoutDuration).Unix())

		inMsg := s.msgCreator.InboundPushQuery(s.ctx.ChainID, requestID, deadline, containerID, container, s.ctx.NodeID)

		// Register a timeout in case I don't respond to myself
		s.router.RegisterRequest(s.ctx.NodeID, s.ctx.ChainID, requestID, message.PushQuery)
		go s.router.HandleInbound(inMsg)
	}

	// Some of [nodeIDs] may be benched. That is, they've been unresponsive
	// so we don't even bother sending messages to them. We just have them immediately fail.
	for nodeID := range nodeIDs {
		if s.timeouts.IsBenched(nodeID, s.ctx.ChainID) {
			s.failedDueToBench[message.PushQuery].Inc() // update metric
			nodeIDs.Remove(nodeID)
			s.timeouts.RegisterRequestToUnreachableValidator()
			// Immediately register a failure. Do so asynchronously to avoid deadlock.
			inMsg := s.msgCreator.InternalQueryFailed(nodeID, s.ctx.ChainID, requestID)
			go s.router.HandleInbound(inMsg)
		}
	}

	// Try to send the messages over the network.
	// [sentTo] are the IDs of validators who may receive the message.
	deadline := uint64(timeoutDuration)
	outMsg, err := s.msgCreator.PushQuery(s.ctx.ChainID, requestID, deadline, containerID, container)
	if err != nil {
		s.ctx.Log.Error("failed to build PushQuery(%s, %d, %s): %s. len(container): %d",
			s.ctx.ChainID,
			requestID,
			containerID,
			err,
			len(container))
		s.ctx.Log.Verbo("container: %s", formatting.DumpBytes{Bytes: container})

		// duly register the failure
		for nodeID := range nodeIDs {
			inMsg := s.msgCreator.InternalQueryFailed(nodeID, s.ctx.ChainID, requestID)
			go s.router.HandleInbound(inMsg)
		}
		return // Packing message failed
	}

	sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false)
	for nodeID := range nodeIDs {
		if sentTo.Contains(nodeID) {
			// Tell the router to expect a reply message from this validator
			s.router.RegisterRequest(nodeID, s.ctx.ChainID, requestID, message.PushQuery)
			nodeIDs.Remove(nodeID)
		} else {
			s.ctx.Log.Debug("failed to send PushQuery(%s, %s, %d, %s)",
				nodeID,
				s.ctx.ChainID,
				requestID,
				containerID)
			s.ctx.Log.Verbo("container: %s", formatting.DumpBytes{Bytes: container})
		}
	}

	// Register failures for nodes we didn't even send a request to.
	for nodeID := range nodeIDs {
		s.timeouts.RegisterRequestToUnreachableValidator()
		inMsg := s.msgCreator.InternalQueryFailed(nodeID, s.ctx.ChainID, requestID)
		go s.router.HandleInbound(inMsg)
	}
}

// SendPullQuery sends a PullQuery message to the consensus engines running on the specified chains
// on the specified nodes.
// The PullQuery message signifies that this consensus engine would like each node to send
// their preferred frontier.
func (s *Sender) SendPullQuery(nodeIDs ids.ShortSet, requestID uint32, containerID ids.ID) {
	s.ctx.Log.Verbo("Sending PullQuery. RequestID: %d. ContainerID: %s", requestID, containerID)

	// Note that this timeout duration won't exactly match the one that gets registered. That's OK.
	timeoutDuration := s.timeouts.TimeoutDuration()

	// Sending a message to myself. No need to send it over the network.
	// Just put it right into the router. Do so asynchronously to avoid deadlock.
	if nodeIDs.Contains(s.ctx.NodeID) {
		nodeIDs.Remove(s.ctx.NodeID)

		deadline := uint64(time.Now().Add(timeoutDuration).Unix())
		inMsg := s.msgCreator.InboundPullQuery(s.ctx.ChainID, requestID, deadline, containerID, s.ctx.NodeID)

		// Register a timeout in case I don't respond to myself
		s.router.RegisterRequest(s.ctx.NodeID, s.ctx.ChainID, requestID, message.PullQuery)
		go s.router.HandleInbound(inMsg)
	}

	// Some of the nodes in [nodeIDs] may be benched. That is, they've been unresponsive
	// so we don't even bother sending messages to them. We just have them immediately fail.
	for nodeID := range nodeIDs {
		if s.timeouts.IsBenched(nodeID, s.ctx.ChainID) {
			s.failedDueToBench[message.PullQuery].Inc() // update metric
			nodeIDs.Remove(nodeID)
			s.timeouts.RegisterRequestToUnreachableValidator()
			// Immediately register a failure. Do so asynchronously to avoid deadlock.
			inMsg := s.msgCreator.InternalQueryFailed(nodeID, s.ctx.ChainID, requestID)
			go s.router.HandleInbound(inMsg)
		}
	}

	// Try to send the messages over the network.
	deadline := uint64(timeoutDuration)
	outMsg, err := s.msgCreator.PullQuery(s.ctx.ChainID, requestID, deadline, containerID)
	s.ctx.Log.AssertNoError(err)
	sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false)

	for nodeID := range nodeIDs {
		if sentTo.Contains(nodeID) {
			// Tell the router to expect a reply message from this validator
			s.router.RegisterRequest(nodeID, s.ctx.ChainID, requestID, message.PullQuery)
			nodeIDs.Remove(nodeID)
		} else {
			s.ctx.Log.Debug("failed to send PullQuery(%s, %s, %d, %s)",
				nodeID,
				s.ctx.ChainID,
				requestID,
				containerID)
		}
	}

	// Register failures for nodes we didn't even send a request to.
	for nodeID := range nodeIDs {
		s.timeouts.RegisterRequestToUnreachableValidator()
		inMsg := s.msgCreator.InternalQueryFailed(nodeID, s.ctx.ChainID, requestID)
		go s.router.HandleInbound(inMsg)
	}
}

// SendChits sends chits
func (s *Sender) SendChits(nodeID ids.ShortID, requestID uint32, votes []ids.ID) {
	s.ctx.Log.Verbo("Sending Chits to node %s. RequestID: %d. Votes: %s", nodeID.PrefixedString(constants.NodeIDPrefix), requestID, votes)
	// If [nodeID] is myself, send this message directly
	// to my own router rather than sending it over the network
	if nodeID == s.ctx.NodeID {
		inMsg := s.msgCreator.InboundChits(s.ctx.ChainID, requestID, votes, nodeID)
		go s.router.HandleInbound(inMsg)
		return
	}

	outMsg, err := s.msgCreator.Chits(s.ctx.ChainID, requestID, votes)
	if err != nil {
		s.ctx.Log.Error("failed to build Chits(%s, %d, %s): %s",
			s.ctx.ChainID,
			requestID,
			votes,
			err)
		return
	}

	nodeIDs := ids.NewShortSet(1)
	nodeIDs.Add(nodeID)
	if sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false); sentTo.Len() == 0 {
		s.ctx.Log.Debug("failed to send Chits(%s, %s, %d, %s)",
			nodeID,
			s.ctx.ChainID,
			requestID,
			votes)
	}
}

// SendAppRequest sends an application-level request to the given nodes.
// The meaning of this request, and how it should be handled, is defined by the VM.
func (s *Sender) SendAppRequest(nodeIDs ids.ShortSet, requestID uint32, appRequestBytes []byte) error {
	s.ctx.Log.Verbo("Sending AppRequest. RequestID: %d. Message: %s", requestID, formatting.DumpBytes{Bytes: appRequestBytes})

	// Note that this timeout duration won't exactly match the one that gets registered. That's OK.
	timeoutDuration := s.timeouts.TimeoutDuration()

	// Sending a message to myself. No need to send it over the network.
	// Just put it right into the router. Do so asynchronously to avoid deadlock.
	if nodeIDs.Contains(s.ctx.NodeID) {
		nodeIDs.Remove(s.ctx.NodeID)

		deadline := uint64(time.Now().Add(timeoutDuration).Unix())
		inMsg := s.msgCreator.InboundAppRequest(s.ctx.ChainID, requestID, deadline, appRequestBytes, s.ctx.NodeID)

		// Register a timeout in case I don't respond to myself
		s.router.RegisterRequest(s.ctx.NodeID, s.ctx.ChainID, requestID, message.AppRequest)
		go s.router.HandleInbound(inMsg)
	}

	// Some of the nodes in [nodeIDs] may be benched. That is, they've been unresponsive
	// so we don't even bother sending messages to them. We just have them immediately fail.
	for nodeID := range nodeIDs {
		if s.timeouts.IsBenched(nodeID, s.ctx.ChainID) {
			s.failedDueToBench[message.AppRequest].Inc() // update metric
			nodeIDs.Remove(nodeID)
			s.timeouts.RegisterRequestToUnreachableValidator()

			// Immediately register a failure. Do so asynchronously to avoid deadlock.
			inMsg := s.msgCreator.InternalAppRequestFailed(nodeID, s.ctx.ChainID, requestID)
			go s.router.HandleInbound(inMsg)
		}
	}

	// Try to send the messages over the network.
	// [sentTo] are the IDs of nodes who may receive the message.
	deadline := uint64(timeoutDuration)
	outMsg, err := s.msgCreator.AppRequest(s.ctx.ChainID, requestID, deadline, appRequestBytes)
	if err != nil {
		s.ctx.Log.Error("failed to build AppRequest(%s, %d): %s", s.ctx.ChainID, requestID, err)
		s.ctx.Log.Verbo("message: %s", formatting.DumpBytes{Bytes: appRequestBytes})

		// duly register the failure
		for nodeID := range nodeIDs {
			inMsg := s.msgCreator.InternalAppRequestFailed(nodeID, s.ctx.ChainID, requestID)
			go s.router.HandleInbound(inMsg)
		}
		return nil
	}

	sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false)
	for nodeID := range nodeIDs {
		if sentTo.Contains(nodeID) {
			// Tell the router to expect a reply message from this validator
			s.router.RegisterRequest(nodeID, s.ctx.ChainID, requestID, message.AppRequest)
			nodeIDs.Remove(nodeID)
		} else {
			s.ctx.Log.Debug("failed to send AppRequest(%s, %s, %d)", nodeID, s.ctx.ChainID, requestID)
			s.ctx.Log.Verbo("failed message: %s", formatting.DumpBytes{Bytes: appRequestBytes})
		}
	}

	// Register failures for nodes we didn't even send a request to.
	for nodeID := range nodeIDs {
		s.timeouts.RegisterRequestToUnreachableValidator()
		inMsg := s.msgCreator.InternalAppRequestFailed(nodeID, s.ctx.ChainID, requestID)
		go s.router.HandleInbound(inMsg)
	}
	return nil
}

// SendAppResponse sends a response to an application-level request from the
// given node
func (s *Sender) SendAppResponse(nodeID ids.ShortID, requestID uint32, appResponseBytes []byte) error {
	if nodeID == s.ctx.NodeID {
		inMsg := s.msgCreator.InboundAppResponse(s.ctx.ChainID, requestID, appResponseBytes, nodeID)
		go s.router.HandleInbound(inMsg)
		return nil
	}

	outMsg, err := s.msgCreator.AppResponse(s.ctx.ChainID, requestID, appResponseBytes)
	if err != nil {
		s.ctx.Log.Error("failed to build AppResponse(%s, %d): %s", s.ctx.ChainID, requestID, err)
		s.ctx.Log.Verbo("message: %s", formatting.DumpBytes{Bytes: appResponseBytes})
		return nil
	}

	nodeIDs := ids.NewShortSet(1)
	nodeIDs.Add(nodeID)
	if sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false); sentTo.Len() == 0 {
		s.ctx.Log.Debug("failed to send AppResponse(%s, %s, %d)", nodeID, s.ctx.ChainID, requestID)
		s.ctx.Log.Verbo("container: %s", formatting.DumpBytes{Bytes: appResponseBytes})
	}

	return nil
}

func (s *Sender) SendAppGossipSpecific(nodeIDs ids.ShortSet, appGossipBytes []byte) error {
	outMsg, err := s.msgCreator.AppGossip(s.ctx.ChainID, appGossipBytes)
	if err != nil {
		s.ctx.Log.Error("failed to build AppGossip(%s) for SpecificGossip: %s", s.ctx.ChainID, err)
		s.ctx.Log.Verbo("message: %s", formatting.DumpBytes{Bytes: appGossipBytes})
	}

	// TODO ABENEGIA: add subnet and isValidator to send
	// if !s.sender.Gossip(outMsg, nodeIDs, s.ctx.SubnetID, s.ctx.IsValidatorOnly()) {
	if sentTo := s.sender.Send(outMsg, nodeIDs, s.ctx.SubnetID, false); sentTo.Len() == 0 {
		s.ctx.Log.Debug("failed to gossip SpecificGossip(%s)", s.ctx.ChainID)
		s.ctx.Log.Verbo("failed message: %s", formatting.DumpBytes{Bytes: appGossipBytes})
	}
	return nil
}

// SendAppGossip sends an application-level gossip message.
func (s *Sender) SendAppGossip(appGossipBytes []byte) error {
	outMsg, err := s.msgCreator.AppGossip(s.ctx.ChainID, appGossipBytes)
	if err != nil {
		s.ctx.Log.Error("failed to build AppGossip(%s): %s", s.ctx.ChainID, err)
		s.ctx.Log.Verbo("message: %s", formatting.DumpBytes{Bytes: appGossipBytes})
	}

	if !s.sender.Gossip(outMsg, s.ctx.SubnetID, s.ctx.IsValidatorOnly()) {
		s.ctx.Log.Debug("failed to gossip AppGossip(%s)", s.ctx.ChainID)
		s.ctx.Log.Verbo("failed message: %s", formatting.DumpBytes{Bytes: appGossipBytes})
	}
	return nil
}

// SendGossip gossips the provided container
func (s *Sender) SendGossip(containerID ids.ID, container []byte) {
	s.ctx.Log.Verbo("Gossiping %s", containerID)
	outMsg, err := s.msgCreator.Put(s.ctx.ChainID, constants.GossipMsgRequestID, containerID, container)
	if err != nil {
		s.ctx.Log.Error("failed to build Put message for gossip.\nContainer length %d, err :  %s",
			len(container),
			err)
		return
	}

	if !s.sender.Gossip(outMsg, s.ctx.SubnetID, s.ctx.IsValidatorOnly()) {
		s.ctx.Log.Debug("failed to gossip GossipMsg(%s)", s.ctx.ChainID)
	}
}
