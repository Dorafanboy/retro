package evm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"retro/internal/logger"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	ErrNoRpcUrlsProvided       = errors.New("no RPC URLs provided")
	ErrEvmClientCreationFailed = errors.New("failed to connect to any provided EVM node")
)

// EVMClient defines the interface for interacting with an EVM compatible blockchain.
type EVMClient interface {
	Close()
	GetChainID() *big.Int
	GetBalance(ctx context.Context, address common.Address) (*big.Int, error)
	GetNonce(ctx context.Context, address common.Address) (uint64, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	SuggestGasTipCap(ctx context.Context) (*big.Int, error)
	EstimateGasLimit(ctx context.Context, msg ethereum.CallMsg) (uint64, error)
	SendRawTransaction(ctx context.Context, tx *types.Transaction) error
	WaitForReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

// Client wraps the go-ethereum client and provides helper methods.
type Client struct {
	*ethclient.Client
	chainID *big.Int
	log     logger.Logger
}

// Ensure Client implements EVMClient interface at compile time.
var _ EVMClient = (*Client)(nil)

// NewClient creates a new EVM client, trying multiple RPC URLs if provided.
func NewClient(ctx context.Context, log logger.Logger, rpcUrls []string) (*Client, error) {
	if len(rpcUrls) == 0 {
		return nil, ErrNoRpcUrlsProvided
	}

	rpcUrl := rpcUrls[rand.Intn(len(rpcUrls))]
	log.Debug("Подключение к EVM ноде...", "url", rpcUrl)

	ethClient, err := ethclient.DialContext(ctx, rpcUrl)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEvmClientCreationFailed, err)
	}
	log.Debug("Успешное подключение к EVM ноде", "url", rpcUrl)

	chainCtx, chainCancel := context.WithTimeout(ctx, 5*time.Second)
	chainID, err := ethClient.ChainID(chainCtx)
	chainCancel()
	if err == nil {
		log.Success("Подключено к EVM узлу", "url", rpcUrl, "chain_id", chainID.String())
		return &Client{Client: ethClient, chainID: chainID, log: log}, nil
	}
	log.Warn("Подключено, но не удалось получить ChainID", "url", rpcUrl, "error", err)
	ethClient.Close()
	return nil, fmt.Errorf("%w: %w", ErrEvmClientCreationFailed, err)
}

// Close terminates the underlying RPC connection
func (c *Client) Close() {
	c.log.Debug("Закрытие соединения с EVM нодой...")
	c.Client.Close()
}

// GetChainID returns the chain ID associated with the client connection
func (c *Client) GetChainID() *big.Int {
	return c.chainID
}

// GetBalance retrieves the native token balance for a given address
func (c *Client) GetBalance(ctx context.Context, address common.Address) (*big.Int, error) {
	c.log.Debug("Запрос баланса...", "address", address.Hex())
	balance, err := c.Client.BalanceAt(ctx, address, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения баланса для %s: %w", address.Hex(), err)
	}
	return balance, nil
}

// GetNonce retrieves the next nonce for an account
func (c *Client) GetNonce(ctx context.Context, address common.Address) (uint64, error) {
	return c.Client.PendingNonceAt(ctx, address)
}

// SuggestGasPrice suggests a gas price for legacy transactions
func (c *Client) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return c.Client.SuggestGasPrice(ctx)
}

// SuggestGasTipCap suggests a gas tip cap for EIP-1559 transactions
func (c *Client) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return c.Client.SuggestGasTipCap(ctx)
}

// EstimateGasLimit estimates the gas needed for a transaction
func (c *Client) EstimateGasLimit(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	return c.Client.EstimateGas(ctx, msg)
}

// SendRawTransaction sends a signed transaction to the network
func (c *Client) SendRawTransaction(ctx context.Context, tx *types.Transaction) error {
	c.log.Debug("Отправка подписанной транзакции", "tx_hash", tx.Hash().Hex())
	err := c.Client.SendTransaction(ctx, tx)
	if err != nil {
		c.log.Error("Не удалось отправить транзакцию", "tx_hash", tx.Hash().Hex(), "error", err)
		return fmt.Errorf("sending transaction failed: %w", err)
	}
	c.log.Info("Транзакция успешно отправлена", "tx_hash", tx.Hash().Hex())
	return nil
}

// WaitForReceipt waits for a transaction receipt, polling the network
func (c *Client) WaitForReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	c.log.Debug("Ожидание квитанции транзакции", "tx_hash", txHash.Hex())
	for {
		receipt, err := c.Client.TransactionReceipt(ctx, txHash)
		if err == nil && receipt != nil {
			c.log.Info("Квитанция транзакции получена", "tx_hash", txHash.Hex(), "status", receipt.Status)
			return receipt, nil
		}
		if err != nil && !errors.Is(err, ethereum.NotFound) {
			c.log.Warn("Ошибка при проверке квитанции транзакции", "tx_hash", txHash.Hex(), "error", err)
			return nil, fmt.Errorf("error fetching receipt: %w", err)
		}

		select {
		case <-time.After(5 * time.Second):
			continue
		case <-ctx.Done():
			c.log.Warn("Контекст отменен во время ожидания квитанции", "tx_hash", txHash.Hex())
			return nil, ctx.Err()
		}
	}
}
