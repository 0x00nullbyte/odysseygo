// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package getter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DioneProtocol/odysseygo/ids"
	"github.com/DioneProtocol/odysseygo/snow"
	"github.com/DioneProtocol/odysseygo/snow/choices"
	"github.com/DioneProtocol/odysseygo/snow/consensus/odyssey"
	"github.com/DioneProtocol/odysseygo/snow/engine/odyssey/vertex"
	"github.com/DioneProtocol/odysseygo/snow/engine/common"
	"github.com/DioneProtocol/odysseygo/snow/validators"
	"github.com/DioneProtocol/odysseygo/utils/set"
)

var errUnknownVertex = errors.New("unknown vertex")

func testSetup(t *testing.T) (*vertex.TestManager, *common.SenderTest, common.Config) {
	peers := validators.NewSet()
	peer := ids.GenerateTestNodeID()
	if err := peers.Add(peer, nil, ids.Empty, 1); err != nil {
		t.Fatal(err)
	}

	sender := &common.SenderTest{T: t}
	sender.Default(true)
	sender.CantSendGetAcceptedFrontier = false

	isBootstrapped := false
	bootstrapTracker := &common.BootstrapTrackerTest{
		T: t,
		IsBootstrappedF: func() bool {
			return isBootstrapped
		},
		BootstrappedF: func(ids.ID) {
			isBootstrapped = true
		},
	}

	commonConfig := common.Config{
		Ctx:                            snow.DefaultConsensusContextTest(),
		Beacons:                        peers,
		SampleK:                        peers.Len(),
		Alpha:                          peers.Weight()/2 + 1,
		Sender:                         sender,
		BootstrapTracker:               bootstrapTracker,
		Timer:                          &common.TimerTest{},
		AncestorsMaxContainersSent:     2000,
		AncestorsMaxContainersReceived: 2000,
		SharedCfg:                      &common.SharedConfig{},
	}

	manager := vertex.NewTestManager(t)
	manager.Default(true)

	return manager, sender, commonConfig
}

func TestAcceptedFrontier(t *testing.T) {
	require := require.New(t)

	manager, sender, config := testSetup(t)

	vtxID0 := ids.GenerateTestID()
	vtxID1 := ids.GenerateTestID()
	vtxID2 := ids.GenerateTestID()

	bsIntf, err := New(manager, config)
	if err != nil {
		t.Fatal(err)
	}
	require.IsType(&getter{}, bsIntf)
	bs := bsIntf.(*getter)

	manager.EdgeF = func(context.Context) []ids.ID {
		return []ids.ID{
			vtxID0,
			vtxID1,
		}
	}

	var accepted []ids.ID
	sender.SendAcceptedFrontierF = func(_ context.Context, _ ids.NodeID, _ uint32, frontier []ids.ID) {
		accepted = frontier
	}

	if err := bs.GetAcceptedFrontier(context.Background(), ids.EmptyNodeID, 0); err != nil {
		t.Fatal(err)
	}

	acceptedSet := set.Set[ids.ID]{}
	acceptedSet.Add(accepted...)

	manager.EdgeF = nil

	if !acceptedSet.Contains(vtxID0) {
		t.Fatalf("Vtx should be accepted")
	}
	if !acceptedSet.Contains(vtxID1) {
		t.Fatalf("Vtx should be accepted")
	}
	if acceptedSet.Contains(vtxID2) {
		t.Fatalf("Vtx shouldn't be accepted")
	}
}

func TestFilterAccepted(t *testing.T) {
	require := require.New(t)

	manager, sender, config := testSetup(t)

	vtxID0 := ids.GenerateTestID()
	vtxID1 := ids.GenerateTestID()
	vtxID2 := ids.GenerateTestID()

	vtx0 := &odyssey.TestVertex{TestDecidable: choices.TestDecidable{
		IDV:     vtxID0,
		StatusV: choices.Accepted,
	}}
	vtx1 := &odyssey.TestVertex{TestDecidable: choices.TestDecidable{
		IDV:     vtxID1,
		StatusV: choices.Accepted,
	}}

	bsIntf, err := New(manager, config)
	if err != nil {
		t.Fatal(err)
	}
	require.IsType(&getter{}, bsIntf)
	bs := bsIntf.(*getter)

	vtxIDs := []ids.ID{vtxID0, vtxID1, vtxID2}

	manager.GetVtxF = func(_ context.Context, vtxID ids.ID) (odyssey.Vertex, error) {
		switch vtxID {
		case vtxID0:
			return vtx0, nil
		case vtxID1:
			return vtx1, nil
		case vtxID2:
			return nil, errUnknownVertex
		}
		t.Fatal(errUnknownVertex)
		return nil, errUnknownVertex
	}

	var accepted []ids.ID
	sender.SendAcceptedF = func(_ context.Context, _ ids.NodeID, _ uint32, frontier []ids.ID) {
		accepted = frontier
	}

	if err := bs.GetAccepted(context.Background(), ids.EmptyNodeID, 0, vtxIDs); err != nil {
		t.Fatal(err)
	}

	acceptedSet := set.Set[ids.ID]{}
	acceptedSet.Add(accepted...)

	manager.GetVtxF = nil

	if !acceptedSet.Contains(vtxID0) {
		t.Fatalf("Vtx should be accepted")
	}
	if !acceptedSet.Contains(vtxID1) {
		t.Fatalf("Vtx should be accepted")
	}
	if acceptedSet.Contains(vtxID2) {
		t.Fatalf("Vtx shouldn't be accepted")
	}
}