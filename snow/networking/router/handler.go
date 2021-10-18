// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package router

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/message"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/networking/tracker"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/utils/uptime"
)

// Handler passes incoming messages from the network to the consensus engine.
// (Actually, it receives the incoming messages from a ChainRouter, but same difference.)
type Handler struct {
	ctx *snow.Context
	// Useful for faking time in tests
	clock   mockable.Clock
	mc      message.Creator
	metrics handlerMetrics
	// The validator set that validates this chain
	validators validators.Set
	// The consensus engine
	engine common.Engine
	// Closed when this handler and [engine] are done shutting down
	closed chan struct{}
	// Receives messages from the VM
	msgFromVMChan <-chan common.Message
	// Tracks CPU time spent processing messages from each node
	cpuTracker tracker.TimeTracker
	// Called in a goroutine when this handler/engine shuts down.
	// May be nil.
	onCloseF            func()
	unprocessedMsgsCond *sync.Cond
	// Holds messages that [engine] hasn't processed yet.
	// [unprocessedMsgsCond.L] must be held while accessing [unprocessedMsgs].
	unprocessedMsgs unprocessedMsgs
	closing         utils.AtomicBool
}

// Initialize this consensus handler
// [engine] must be initialized before initializing this handler
func (h *Handler) Initialize(
	mc message.Creator,
	engine common.Engine,
	validators validators.Set,
	msgFromVMChan <-chan common.Message,
	metricsNamespace string,
	metricsRegisterer prometheus.Registerer,
) error {
	h.ctx = engine.Context()
	if err := h.metrics.Initialize(metricsNamespace, metricsRegisterer); err != nil {
		return fmt.Errorf("initializing handler metrics errored with: %s", err)
	}
	h.mc = mc
	h.closed = make(chan struct{})
	h.msgFromVMChan = msgFromVMChan
	h.engine = engine
	h.validators = validators
	var lock sync.Mutex
	h.unprocessedMsgsCond = sync.NewCond(&lock)
	h.cpuTracker = tracker.NewCPUTracker(uptime.IntervalFactory{}, defaultCPUInterval)
	var err error
	h.unprocessedMsgs, err = newUnprocessedMsgs(h.ctx.Log, h.validators, h.cpuTracker, metricsNamespace, metricsRegisterer)
	return err
}

// Context of this Handler
func (h *Handler) Context() *snow.Context { return h.engine.Context() }

// Engine returns the engine this handler dispatches to
func (h *Handler) Engine() common.Engine { return h.engine }

// SetEngine sets the engine for this handler to dispatch to
func (h *Handler) SetEngine(engine common.Engine) { h.engine = engine }

// Dispatch waits for incoming messages from the router
// and, when they arrive, sends them to the consensus engine
func (h *Handler) Dispatch() {
	defer h.shutdown()

	// Handle messages from the VM
	go h.dispatchInternal()

	// Handle messages from the router
	for {
		// Wait until there is an unprocessed message
		h.unprocessedMsgsCond.L.Lock()
		for {
			if closing := h.closing.GetValue(); closing {
				h.unprocessedMsgsCond.L.Unlock()
				return
			}
			if h.unprocessedMsgs.Len() == 0 {
				// Signalled in [h.push] and [h.StartShutdown]
				h.unprocessedMsgsCond.Wait()
				continue
			}
			break
		}

		// Get the next message we should process
		msg := h.unprocessedMsgs.Pop()
		h.unprocessedMsgsCond.L.Unlock()

		// If this message's deadline has passed, don't process it.
		if !msg.ExpirationTime().IsZero() && h.clock.Time().After(msg.ExpirationTime()) {
			nodeID := msg.NodeID()
			h.ctx.Log.Verbo("Dropping message from %s%s due to timeout. msg: %s",
				constants.NodeIDPrefix, nodeID, msg)
			h.metrics.expired.Inc()
			msg.OnFinishedHandling()
			continue
		}

		// Process the message.
		// If there was an error, shut down this chain
		if err := h.handleMsg(msg); err != nil {
			h.ctx.Log.Fatal("chain shutting down due to error %q while processing message: %s",
				err, msg)
			h.StartShutdown()
			return
		}
	}
}

// IsPeriodic returns true if this message is of a type that is sent on a
// periodic basis.
func isPeriodic(inMsg message.InboundMessage) bool {
	var reqID uint32
	if inMsg.Op() == message.AppGossip {
		reqID = constants.GossipMsgRequestID
	} else {
		reqIDInf := inMsg.Get(message.RequestID)
		if reqIDInf == nil {
			return false
		}

		reqID = reqIDInf.(uint32)
	}

	return reqID == constants.GossipMsgRequestID ||
		inMsg.Op() == message.GossipRequest
}

// Dispatch a message to the consensus engine.
func (h *Handler) handleMsg(msg message.InboundMessage) error {
	startTime := h.clock.Time()

	isPeriodic := isPeriodic(msg)
	if isPeriodic {
		h.ctx.Log.Verbo("Forwarding message to consensus: %s", msg)
	} else {
		h.ctx.Log.Debug("Forwarding message to consensus: %s", msg)
	}

	h.ctx.Lock.Lock()
	defer h.ctx.Lock.Unlock()

	var err error
	msgType := msg.Op()
	switch msgType {
	case message.Notify:
		vmMsg := msg.Get(message.VMMessage).(uint32)
		err = h.engine.Notify(common.Message(vmMsg))
		h.metrics.notify.Observe(float64(h.clock.Time().Sub(startTime)))
	case message.GossipRequest:
		err = h.engine.Gossip()
		h.metrics.gossip.Observe(float64(h.clock.Time().Sub(startTime)))
	case message.Timeout:
		err = h.engine.Timeout()
		h.metrics.timeout.Observe(float64(h.clock.Time().Sub(startTime)))
	default:
		err = h.handleConsensusMsg(msg)
		endTime := h.clock.Time()
		handleDuration := endTime.Sub(startTime)
		histogram := h.metrics.getMSGHistogram(msg.Op())
		histogram.Observe(float64(handleDuration))
		nodeID := msg.NodeID()
		h.cpuTracker.UtilizeTime(nodeID, startTime, endTime)
	}

	msg.OnFinishedHandling()

	if isPeriodic {
		h.ctx.Log.Verbo("Finished handling message: %s", msgType)
	} else {
		h.ctx.Log.Debug("Finished handling message: %s", msgType)
	}
	return err
}

// Assumes [h.ctx.Lock] is locked
// Relevant fields in msgs must be validated before being dispatched to the engine.
// An invalid msg is logged and dropped silently since err would cause a chain shutdown.
func (h *Handler) handleConsensusMsg(msg message.InboundMessage) error {
	nodeID := msg.NodeID()

	switch msg.Op() {
	case message.GetAcceptedFrontier:
		reqID := msg.Get(message.RequestID).(uint32)
		return h.engine.GetAcceptedFrontier(nodeID, reqID)

	case message.AcceptedFrontier:
		reqID := msg.Get(message.RequestID).(uint32)
		containerIDs, err := message.DecodeContainerIDs(msg)
		if err != nil {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: %s",
				msg.Op(), nodeID, h.engine.Context().ChainID, reqID, err)
			return nil
		}
		return h.engine.AcceptedFrontier(nodeID, reqID, containerIDs)

	case message.GetAcceptedFrontierFailed:
		reqID := msg.Get(message.RequestID).(uint32)
		return h.engine.GetAcceptedFrontierFailed(nodeID, reqID)

	case message.GetAccepted:
		reqID := msg.Get(message.RequestID).(uint32)
		containerIDs, err := message.DecodeContainerIDs(msg)
		if err != nil {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: %s",
				msg.Op(), nodeID, h.engine.Context().ChainID, reqID, err)
			return nil
		}
		return h.engine.GetAccepted(nodeID, reqID, containerIDs)

	case message.Accepted:
		reqID := msg.Get(message.RequestID).(uint32)
		containerIDs, err := message.DecodeContainerIDs(msg)
		if err != nil {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: %s",
				msg.Op(), nodeID, h.engine.Context().ChainID, reqID, err)
			return nil
		}
		return h.engine.Accepted(nodeID, reqID, containerIDs)

	case message.GetAcceptedFailed:
		reqID := msg.Get(message.RequestID).(uint32)
		return h.engine.GetAcceptedFailed(nodeID, reqID)

	case message.GetAncestors:
		reqID := msg.Get(message.RequestID).(uint32)
		containerID, err := ids.ToID(msg.Get(message.ContainerID).([]byte))
		if err != nil {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: %s",
				msg.Op(), nodeID, h.engine.Context().ChainID, reqID, err)
			return nil
		}
		return h.engine.GetAncestors(nodeID, reqID, containerID)

	case message.GetAncestorsFailed:
		reqID := msg.Get(message.RequestID).(uint32)
		return h.engine.GetAncestorsFailed(nodeID, reqID)

	case message.MultiPut:
		reqID := msg.Get(message.RequestID).(uint32)
		containers, ok := msg.Get(message.MultiContainerBytes).([][]byte)
		if !ok {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: could not parse MultiContainerBytes",
				msg.Op(), nodeID, h.engine.Context().ChainID, reqID)
			return nil
		}
		return h.engine.MultiPut(nodeID, reqID, containers)

	case message.Get:
		reqID := msg.Get(message.RequestID).(uint32)
		containerID, err := ids.ToID(msg.Get(message.ContainerID).([]byte))
		h.ctx.Log.AssertNoError(err)
		return h.engine.Get(nodeID, reqID, containerID)

	case message.GetFailed:
		reqID := msg.Get(message.RequestID).(uint32)
		return h.engine.GetFailed(nodeID, reqID)

	case message.Put:
		reqID := msg.Get(message.RequestID).(uint32)
		containerID, err := ids.ToID(msg.Get(message.ContainerID).([]byte))
		h.ctx.Log.AssertNoError(err)
		container, ok := msg.Get(message.ContainerBytes).([]byte)
		if !ok {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: could not parse ContainerBytes",
				msg.Op(), nodeID, h.engine.Context().ChainID, reqID)
			return nil
		}
		return h.engine.Put(nodeID, reqID, containerID, container)

	case message.PushQuery:
		reqID := msg.Get(message.RequestID).(uint32)
		containerID, err := ids.ToID(msg.Get(message.ContainerID).([]byte))
		h.ctx.Log.AssertNoError(err)
		container, ok := msg.Get(message.ContainerBytes).([]byte)
		if !ok {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: could not parse ContainerBytes",
				msg.Op(), nodeID, h.engine.Context().ChainID, reqID)
			return nil
		}
		return h.engine.PushQuery(nodeID, reqID, containerID, container)

	case message.PullQuery:
		reqID := msg.Get(message.RequestID).(uint32)
		containerID, err := ids.ToID(msg.Get(message.ContainerID).([]byte))
		h.ctx.Log.AssertNoError(err)
		return h.engine.PullQuery(nodeID, reqID, containerID)

	case message.QueryFailed:
		reqID := msg.Get(message.RequestID).(uint32)
		return h.engine.QueryFailed(nodeID, reqID)

	case message.Chits:
		reqID := msg.Get(message.RequestID).(uint32)
		votes, err := message.DecodeContainerIDs(msg)
		if err != nil {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: %s",
				msg.Op(), nodeID, h.engine.Context().ChainID, reqID, err)
			return nil
		}
		return h.engine.Chits(nodeID, reqID, votes)

	case message.Connected:
		return h.engine.Connected(nodeID)

	case message.Disconnected:
		return h.engine.Disconnected(nodeID)

	case message.AppRequest:
		reqID := msg.Get(message.RequestID).(uint32)
		appRequestBytes, ok := msg.Get(message.AppRequestBytes).([]byte)
		if !ok {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: could not parse AppRequestBytes",
				msg.Op(), nodeID, h.engine.Context().ChainID, reqID)
			return nil
		}
		return h.engine.AppRequest(nodeID, reqID, appRequestBytes)

	case message.AppResponse:
		reqID := msg.Get(message.RequestID).(uint32)
		appResponseBytes, ok := msg.Get(message.AppResponseBytes).([]byte)
		if !ok {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: could not parse AppResponseBytes",
				msg.Op(), nodeID, h.engine.Context().ChainID, reqID)
			return nil
		}
		return h.engine.AppResponse(nodeID, reqID, appResponseBytes)

	case message.AppGossip:
		appGossipBytes, ok := msg.Get(message.AppGossipBytes).([]byte)
		if !ok {
			h.ctx.Log.Debug("Malformed message %s from (%s, %s, %d) dropped. Error: could not parse AppGossipBytes",
				msg.Op(), nodeID, h.engine.Context().ChainID, constants.GossipMsgRequestID)
			return nil
		}
		return h.engine.AppGossip(nodeID, appGossipBytes)

	default:
		h.ctx.Log.Warn("Attempt to submit to engine unhandled consensus msg %s from from (%s, %s). Dropping it",
			msg.Op(), nodeID, h.engine.Context().ChainID)
		return nil
	}
}

// Timeout passes a new timeout notification to the consensus engine
func (h *Handler) Timeout() {
	inMsg := h.mc.InternalTimeout(h.ctx.NodeID)
	h.push(inMsg)
}

// Connected passes a new connection notification to the consensus engine
func (h *Handler) Connected(nodeID ids.ShortID) {
	inMsg := h.mc.InternalConnected(nodeID)
	h.push(inMsg)
}

// Disconnected passes a new connection notification to the consensus engine
func (h *Handler) Disconnected(nodeID ids.ShortID) {
	inMsg := h.mc.InternalDisconnected(nodeID)
	h.push(inMsg)
}

// Gossip passes a gossip request to the consensus engine
func (h *Handler) Gossip() {
	if !h.ctx.IsBootstrapped() {
		// Shouldn't send gossiping messages while the chain is bootstrapping
		return
	}

	inMsg := h.mc.InternalGossipRequest(h.ctx.NodeID)
	h.push(inMsg)
}

// StartShutdown starts the shutdown process for this handler/engine.
// [h] must never be invoked again after calling this method.
// This method causes [shutdown] to eventually be called.
// [h.closed] is closed when this handler/engine are done shutting down.
func (h *Handler) StartShutdown() {
	// Must hold [h.unprocessedMsgsCond.L] here to ensure
	// there's no race condition in Dispatch where we check
	// the value of [h.closing].
	h.unprocessedMsgsCond.L.Lock()
	h.closing.SetValue(true)
	h.unprocessedMsgsCond.L.Unlock()

	// If we're waiting in [Dispatch] wake up.
	h.unprocessedMsgsCond.Signal()
	// Don't process any more bootstrap messages.
	// If [h.engine] is processing a bootstrap message, stop.
	// We do this because if we didn't, and the engine was in the
	// middle of executing state transitions during bootstrapping,
	// we wouldn't be able to grab [h.ctx.Lock] until the engine
	// finished executing state transitions, which may take a long time.
	// As a result, the router would time out on shutting down this chain.
	h.engine.Halt()
}

// Calls [h.engine.Shutdown] and [h.onCloseF]; closes [h.closed].
func (h *Handler) shutdown() {
	h.ctx.Lock.Lock()
	defer h.ctx.Lock.Unlock()

	startTime := h.clock.Time()
	if err := h.engine.Shutdown(); err != nil {
		h.ctx.Log.Error("Error while shutting down the chain: %s", err)
	}
	if h.onCloseF != nil {
		go h.onCloseF()
	}
	endTime := h.clock.Time()
	h.metrics.shutdown.Observe(float64(endTime.Sub(startTime)))
	close(h.closed)
}

// Assumes [h.unprocessedMsgsCond.L] is not held
func (h *Handler) push(msg message.InboundMessage) {
	nodeID := msg.NodeID()
	if nodeID == ids.ShortEmpty {
		// This should never happen
		h.ctx.Log.Warn("message does not have node ID of sender. Message: %s", msg)
	}

	h.unprocessedMsgsCond.L.Lock()
	defer h.unprocessedMsgsCond.L.Unlock()

	h.unprocessedMsgs.Push(msg)
	h.unprocessedMsgsCond.Signal()
}

func (h *Handler) dispatchInternal() {
	for {
		select {
		case <-h.closed:
			return
		case msg := <-h.msgFromVMChan:
			if closing := h.closing.GetValue(); closing {
				return
			}
			// handle a message from the VM
			inMsg := h.mc.InternalVMMessage(h.ctx.NodeID, uint32(msg))
			h.push(inMsg)
		}
	}
}

func (h *Handler) endInterval() {
	now := h.clock.Time()
	h.cpuTracker.EndInterval(now)
}

// if subnet is validator only and this is not a validator or self, returns false.
func (h *Handler) isValidator(nodeID ids.ShortID) bool {
	return !h.ctx.IsValidatorOnly() || nodeID == h.ctx.NodeID || h.validators.Contains(nodeID)
}
