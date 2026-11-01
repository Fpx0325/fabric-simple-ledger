package main

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"regexp"
	"strings"
)

type Contract struct{ contractapi.Contract }
type Evidence struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Hash      string `json:"hash"`
	OwnerMSP  string `json:"owner_msp"`
	Creator   string `json:"creator"`
	TxID      string `json:"tx_id"`
	CreatedAt string `json:"created_at"`
}

var validID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var validHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func authorize(ctx contractapi.TransactionContextInterface, write bool) (string, error) {
	msp, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return "", err
	}
	if msp != "Org1MSP" && msp != "Org2MSP" {
		return "", fmt.Errorf("FORBIDDEN: unknown organization")
	}
	if write && msp != "Org1MSP" {
		return "", fmt.Errorf("FORBIDDEN: only Org1MSP can write")
	}
	return msp, nil
}
func (c *Contract) CreateEvidence(ctx contractapi.TransactionContextInterface, id, title, hash string) (*Evidence, error) {
	msp, err := authorize(ctx, true)
	if err != nil {
		return nil, err
	}
	if !validID.MatchString(id) || !validHash.MatchString(hash) || len(strings.TrimSpace(title)) == 0 || len(title) > 240 {
		return nil, fmt.Errorf("INVALID: id, title or SHA-256")
	}
	key := "evidence:" + id
	old, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, err
	}
	if old != nil {
		return nil, fmt.Errorf("EXISTS: evidence is immutable")
	}
	creator, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return nil, err
	}
	ts, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return nil, err
	}
	e := &Evidence{id, title, hash, msp, creator, ctx.GetStub().GetTxID(), fmt.Sprintf("%d.%09d", ts.Seconds, ts.Nanos)}
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	if err = ctx.GetStub().PutState(key, data); err != nil {
		return nil, err
	}
	return e, nil
}
func (c *Contract) ReadEvidence(ctx contractapi.TransactionContextInterface, id string) (*Evidence, error) {
	if _, err := authorize(ctx, false); err != nil {
		return nil, err
	}
	if !validID.MatchString(id) {
		return nil, fmt.Errorf("INVALID: id")
	}
	data, err := ctx.GetStub().GetState("evidence:" + id)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("NOT_FOUND: evidence")
	}
	var e Evidence
	if err = json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}
func main() {
	cc, err := contractapi.NewChaincode(&Contract{})
	if err != nil {
		panic(err)
	}
	if err = cc.Start(); err != nil {
		panic(err)
	}
}
