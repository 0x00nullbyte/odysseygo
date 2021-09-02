// (c) 2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package block

import (
	"crypto/x509"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/utils/hashing"
)

func Parse(bytes []byte) (Block, error) {
	block := statelessBlock{
		id:    hashing.ComputeHash256Array(bytes),
		bytes: bytes,
	}
	parsedVersion, err := c.Unmarshal(bytes, &block)
	if err != nil {
		return nil, err
	}
	if parsedVersion != version {
		return nil, fmt.Errorf("expected codec version %d but got %d", version, parsedVersion)
	}

	block.timestamp = time.Unix(block.StatelessBlock.Timestamp, 0)
	if len(block.StatelessBlock.Certificate) == 0 {
		return &block, nil
	}

	cert, err := x509.ParseCertificate(block.StatelessBlock.Certificate)
	if err != nil {
		return nil, err
	}
	block.cert = cert
	block.proposer = hashing.ComputeHash160Array(hashing.ComputeHash256(cert.Raw))
	return &block, nil
}
