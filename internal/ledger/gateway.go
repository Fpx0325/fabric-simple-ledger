package ledger

import (
	"context"
	"crypto/x509"
	"fmt"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
	"path/filepath"
	"time"
)

type Gateway struct {
	Contract *client.Contract
	gateway  *client.Gateway
	conn     *grpc.ClientConn
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func Open() (*Gateway, error) {
	root := env("FABRIC_ORG_PATH", "network/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com")
	certPEM, err := os.ReadFile(filepath.Join(root, "users/User1@org1.example.com/msp/signcerts/cert.pem"))
	if err != nil {
		return nil, err
	}
	cert, err := identity.CertificateFromPEM(certPEM)
	if err != nil {
		return nil, err
	}
	id, err := identity.NewX509Identity("Org1MSP", cert)
	if err != nil {
		return nil, err
	}
	keys, err := filepath.Glob(filepath.Join(root, "users/User1@org1.example.com/msp/keystore/*"))
	if err != nil || len(keys) != 1 {
		return nil, fmt.Errorf("expected exactly one private key")
	}
	keyPEM, err := os.ReadFile(keys[0])
	if err != nil {
		return nil, err
	}
	key, err := identity.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, err
	}
	sign, err := identity.NewPrivateKeySign(key)
	if err != nil {
		return nil, err
	}
	tlsPEM, err := os.ReadFile(filepath.Join(root, "peers/peer0.org1.example.com/tls/ca.crt"))
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(tlsPEM) {
		return nil, fmt.Errorf("invalid TLS CA")
	}
	conn, err := grpc.NewClient(env("FABRIC_PEER_ENDPOINT", "localhost:7051"), grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(pool, "peer0.org1.example.com")))
	if err != nil {
		return nil, err
	}
	gw, err := client.Connect(id, client.WithSign(sign), client.WithClientConnection(conn), client.WithEvaluateTimeout(5*time.Second), client.WithEndorseTimeout(15*time.Second), client.WithSubmitTimeout(5*time.Second), client.WithCommitStatusTimeout(30*time.Second))
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &Gateway{gw.GetNetwork(env("FABRIC_CHANNEL", "mychannel")).GetContract("evidence"), gw, conn}, nil
}
func (g *Gateway) Close() { g.gateway.Close(); g.conn.Close() }
func (g *Gateway) Create(ctx context.Context, id, title, hash string) ([]byte, error) {
	return g.Contract.SubmitWithContext(ctx, "CreateEvidence", client.WithArguments(id, title, hash))
}
func (g *Gateway) Read(ctx context.Context, id string) ([]byte, error) {
	return g.Contract.EvaluateWithContext(ctx, "ReadEvidence", client.WithArguments(id))
}
