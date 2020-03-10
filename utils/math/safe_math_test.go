// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package math

import (
	"math"
	"testing"
)

const maxUint64 uint64 = math.MaxUint64

func TestAdd64(t *testing.T) {
	sum, err := Add64(0, maxUint64)
	if err != nil {
		t.Fatalf("Add64 failed unexpectedly")
	}
	if sum != maxUint64 {
		t.Fatalf("Expected %d, got %d", maxUint64, sum)
	}

	sum, err = Add64(maxUint64, 0)
	if err != nil {
		t.Fatalf("Add64 failed unexpectedly")
	}
	if sum != math.MaxUint64 {
		t.Fatalf("Expected %d, got %d", maxUint64, sum)
	}

	sum, err = Add64(1<<62, 1<<62)
	if err != nil {
		t.Fatalf("Add64 failed unexpectedly")
	}
	if sum != uint64(1<<63) {
		t.Fatalf("Expected %d, got %d", uint64(1<<63), sum)
	}

	sum, err = Add64(1, maxUint64)
	if err == nil {
		t.Fatalf("Add64 succeeded unexpectedly")
	}

	sum, err = Add64(maxUint64, 1)
	if err == nil {
		t.Fatalf("Add64 succeeded unexpectedly")
	}

	sum, err = Add64(maxUint64, maxUint64)
	if err == nil {
		t.Fatalf("Add64 succeeded unexpectedly")
	}
}

func TestMul64(t *testing.T) {
	if prod, err := Mul64(maxUint64, 0); err != nil {
		t.Fatalf("Mul64 failed unexpectedly")
	} else if prod != 0 {
		t.Fatalf("Mul64 returned wrong value")
	}

	if prod, err := Mul64(maxUint64, 1); err != nil {
		t.Fatalf("Mul64 failed unexpectedly")
	} else if prod != maxUint64 {
		t.Fatalf("Mul64 returned wrong value")
	}

	if _, err := Mul64(maxUint64-1, 2); err == nil {
		t.Fatalf("Mul64 overflowed")
	}
}
