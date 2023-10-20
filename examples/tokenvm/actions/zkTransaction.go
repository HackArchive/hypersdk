package actions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*zkTransaction)(nil)

var assetCommitment [32]byte

type zkTransaction struct {
	From ed25519.PublicKey `json:"from"`

	To ed25519.PublicKey `json:"to"`

	Asset [32]byte `json:"asset"`
}

func (t *zkTransaction) StateKeys(rauth chain.Auth, _ ids.ID) []string {
	return []string{
		string(storage.BalanceKey(auth.GetActor(rauth), assetCommitment)),
		string(storage.BalanceKey(ed25519.PublicKey(t.To), assetCommitment)),
	}
}
func (*zkTransaction) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.BalanceChunks}
}

func (*zkTransaction) GetTypeID() uint8 {
	return zkTransactionID
}

func (t *zkTransaction) Execute(context.Context, chain.Rules, state.Mutable, int64, chain.Auth,
	ids.ID, bool) (bool, uint64, []byte, *warp.UnsignedMessage, error) {

	field := big.NewInt(256)

	coefs := make([]*big.Int, 4)
	for i := range coefs {
		coef, err := rand.Int(rand.Reader, field)
		if err != nil {
			fmt.Println(err)
			return false, zkTransactionComputeUnits, nil, nil, err
		}
		coefs[i] = coef
	}

	polynomial := NewPolynomial(coefs...)

	// Generate key pair
	senderPrivateKey, err := ed25519.GeneratePrivateKey()
	if err != nil {
		fmt.Println(err)
		return false, zkTransactionComputeUnits, nil, nil, err
	}
	senderPublicKey := senderPrivateKey.PublicKey()

	receiverPrivateKey, err := ed25519.GeneratePrivateKey()
	if err != nil {
		fmt.Println(err)
		return false, zkTransactionComputeUnits, nil, nil, err
	}
	receiverPublicKey := receiverPrivateKey.PublicKey()

	assetValue := new(big.Int).SetInt64(100)
	assetCommitment := commitAsset(assetValue)

	proof := NewCubicZKProof(field, polynomial)

	challenge, response, err := proof.GenerateProof(senderPrivateKey[:], receiverPublicKey[:], assetCommitment)
	if err != nil {
		fmt.Println(err)
		return false, zkTransactionComputeUnits, nil, nil, err
	}

	err = proof.VerifyProof(senderPublicKey[:], receiverPublicKey[:], assetCommitment, challenge, response)
	if err != nil {
		fmt.Println(err)
		return false, zkTransactionComputeUnits, nil, nil, err
	}

	fmt.Println("The proof is valid for the sender's private key, receiver's public key, and the asset commitment.")
	return true, zkTransactionComputeUnits, nil, nil, err
}

func (*zkTransaction) OutputsWarpMessage() bool {
	return false
}

func (t *zkTransaction) Size() int {
	return ed25519.PublicKeyLen + ed25519.PublicKeyLen + consts.IDLen + consts.Uint64Len
}

func (*zkTransaction) MaxComputeUnits(chain.Rules) uint64 {
	return zkTransactionComputeUnits
}

func (t *zkTransaction) Marshal(p *codec.Packer) {
	p.PackPublicKey(t.From)
	p.PackPublicKey(t.To)
	p.PackID(assetCommitment)
}

func (*zkTransaction) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func commitAsset(assetValue *big.Int) *big.Int {
	commitment := sha256.Sum256(assetValue.Bytes())
	return new(big.Int).SetBytes(commitment[:])
}
