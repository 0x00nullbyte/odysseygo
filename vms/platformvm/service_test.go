// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"encoding/json"
	"testing"
)

func TestAddDefaultSubnetValidator(t *testing.T) {
	expectedJSONString := `{"startTime":"0","endTime":"0","id":null,"destination":"","delegationFeeRate":"0","payerNonce":"0"}`
	args := AddDefaultSubnetValidatorArgs{}
	bytes, err := json.Marshal(&args)
	if err != nil {
		t.Fatal(err)
	}
	jsonString := string(bytes)
	if jsonString != expectedJSONString {
		t.Fatalf("Expected: %s\nResult: %s", expectedJSONString, jsonString)
	}
}

func TestCreateBlockchainArgsParsing(t *testing.T) {
	jsonString := `{"vmID":"lol","fxIDs":["secp256k1"], "name":"awesome", "payerNonce":5, "genesisData":"SkB92YpWm4Q2iPnLGCuDPZPgUQMxajqQQuz91oi3xD984f8r"}`
	args := CreateBlockchainArgs{}
	err := json.Unmarshal([]byte(jsonString), &args)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = json.Marshal(args.GenesisData); err != nil {
		t.Fatal(err)
	}
}

func TestExportKey(t *testing.T) {
	jsonString := `{"username":"ScoobyUser","password":"ShaggyPassword1","address":"6Y3kysjF9jnHnYkdS9yGAuoHyae2eNmeV"}`
	args := ExportKeyArgs{}
	err := json.Unmarshal([]byte(jsonString), &args)
	if err != nil {
		t.Fatal(err)
	}
}

func TestImportKey(t *testing.T) {
	jsonString := `{"username":"ScoobyUser","password":"ShaggyPassword1","privateKey":"ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"}`
	args := ImportKeyArgs{}
	err := json.Unmarshal([]byte(jsonString), &args)
	if err != nil {
		t.Fatal(err)
	}
}
