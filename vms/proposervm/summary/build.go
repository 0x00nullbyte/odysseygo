// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package summary

import (
	"fmt"

	"github.com/ava-labs/avalanchego/utils/hashing"
)

func Build(
	block []byte,
	coreSummary []byte,
) (StateSummary, error) {
	summary := stateSummary{
		Block:        block,
		InnerSummary: coreSummary,
	}

	bytes, err := c.Marshal(codecVersion, &summary)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal proposer summary due to: %w", err)
	}

	summary.id = hashing.ComputeHash256Array(bytes)
	summary.bytes = bytes
	return &summary, nil
}
