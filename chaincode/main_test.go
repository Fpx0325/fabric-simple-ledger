package main

import (
	"crypto/x509"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"strings"
	"testing"
)

type identityStub struct{ msp string }

func (i identityStub) GetID() (string, error)                         { return "test-user", nil }
func (i identityStub) GetMSPID() (string, error)                      { return i.msp, nil }
func (i identityStub) GetAttributeValue(string) (string, bool, error) { return "", false, nil }
func (i identityStub) AssertAttributeValue(string, string) error      { return nil }
func (i identityStub) GetX509Certificate() (*x509.Certificate, error) { return nil, nil }
func testContext(msp string) (*contractapi.TransactionContext, *shimtest.MockStub) {
	s := shimtest.NewMockStub("evidence", nil)
	c := new(contractapi.TransactionContext)
	c.SetStub(s)
	c.SetClientIdentity(identityStub{msp})
	s.MockTransactionStart("tx-1")
	return c, s
}
func TestEvidenceLifecycle(t *testing.T) {
	ctx, s := testContext("Org1MSP")
	defer s.MockTransactionEnd("tx-1")
	c := &Contract{}
	hash := strings.Repeat("a", 64)
	e, err := c.CreateEvidence(ctx, "DOC-1", "测试合同", hash)
	if err != nil {
		t.Fatal(err)
	}
	if e.OwnerMSP != "Org1MSP" || e.Hash != hash {
		t.Fatal(e)
	}
	if _, err = c.CreateEvidence(ctx, "DOC-1", "覆盖", hash); err == nil {
		t.Fatal("overwrite accepted")
	}
	got, err := c.ReadEvidence(ctx, "DOC-1")
	if err != nil || got.Title != "测试合同" {
		t.Fatal(got, err)
	}
	ctx.SetClientIdentity(identityStub{"Org2MSP"})
	if _, err = c.ReadEvidence(ctx, "DOC-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = c.CreateEvidence(ctx, "DOC-2", "标题", hash); err == nil {
		t.Fatal("Org2 write accepted")
	}
	ctx.SetClientIdentity(identityStub{"UnknownMSP"})
	if _, err = c.ReadEvidence(ctx, "DOC-1"); err == nil {
		t.Fatal("unknown org read accepted")
	}
}
func TestInvalidAndMissing(t *testing.T) {
	ctx, s := testContext("Org1MSP")
	defer s.MockTransactionEnd("tx-1")
	c := &Contract{}
	for _, id := range []string{"", "bad id", "../x"} {
		if _, err := c.CreateEvidence(ctx, id, "标题", strings.Repeat("a", 64)); err == nil {
			t.Fatal("invalid ID accepted")
		}
	}
	if _, err := c.CreateEvidence(ctx, "ok", "标题", "bad-hash"); err == nil {
		t.Fatal("invalid hash accepted")
	}
	if _, err := c.ReadEvidence(ctx, "missing"); err == nil {
		t.Fatal("missing record accepted")
	}
}
func TestChaincodeConstruction(t *testing.T) {
	if _, err := contractapi.NewChaincode(&Contract{}); err != nil {
		t.Fatal(err)
	}
}
