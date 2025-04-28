package evm

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// Signer wraps an ECDSA private key to provide signing capabilities for EVM transactions and messages.
type Signer struct {
	privateKey *ecdsa.PrivateKey
	address    common.Address
}

// NewSigner creates a new Signer instance from an ECDSA private key.
func NewSigner(pk *ecdsa.PrivateKey) *Signer {
	if pk == nil {
		panic("private key cannot be nil") // Or return an error
	}
	address := crypto.PubkeyToAddress(pk.PublicKey)
	return &Signer{
		privateKey: pk,
		address:    address,
	}
}

// Address returns the Ethereum address associated with the Signer's private key.
func (s *Signer) Address() common.Address {
	return s.address
}

// SignTx signs the given Ethereum transaction using the Signer's private key and the provided chain ID.
func (s *Signer) SignTx(tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainID), s.privateKey)
	if err != nil {
		// Логгер здесь не используется, т.к. пакет evm не импортирует logger,
		// но ошибка возвращается для обработки вызывающей стороной.
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}
	return signedTx, nil
}

// SignPersonalMessage signs the given message according to the EIP-191 standard (`personal_sign`).
func (s *Signer) SignPersonalMessage(_ context.Context, message []byte) ([]byte, error) {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	dataToSign := append([]byte(prefix), message...)
	msgHash := crypto.Keccak256Hash(dataToSign)

	sig, err := crypto.Sign(msgHash.Bytes(), s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message hash: %w", err)
	}

	sig[crypto.RecoveryIDOffset] += 27

	return sig, nil
}
