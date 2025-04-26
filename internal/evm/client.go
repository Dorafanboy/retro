package evm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"retro_template/internal/logger"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	// ErrNoRpcUrlsProvided indicates that no RPC URLs were provided for client creation.
	ErrNoRpcUrlsProvided = errors.New("no RPC URLs provided")
	// ErrEvmClientCreationFailed indicates that the client failed to connect to any of the provided RPC URLs.
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
	ethClient *ethclient.Client
	chainID   *big.Int
}

// Ensure Client implements EVMClient interface at compile time.
var _ EVMClient = (*Client)(nil)

// NewClient creates a new EVM client, trying multiple RPC URLs if provided.
func NewClient(ctx context.Context, rpcUrls []string) (*Client, error) {
	if len(rpcUrls) == 0 {
		return nil, ErrNoRpcUrlsProvided
	}

	logger.Info("Подключение к EVM узлу...", "rpc_count", len(rpcUrls))
	var lastErr error

	for i, url := range rpcUrls {
		logger.Debug("Попытка подключения", "rpc_url", url, "attempt", i+1)

		dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
		client, err := ethclient.DialContext(dialCtx, url)
		dialCancel()
		if err == nil {
			chainCtx, chainCancel := context.WithTimeout(ctx, 5*time.Second)
			chainID, err := client.ChainID(chainCtx)
			chainCancel()
			if err == nil {
				logger.Success("Подключено к EVM узлу", "url", url, "chain_id", chainID.String())
				return &Client{ethClient: client, chainID: chainID}, nil
			}
			logger.Warn("Подключено, но не удалось получить ChainID", "url", url, "error", err)
			client.Close()
			lastErr = err
		} else {
			logger.Warn("Не удалось подключиться к EVM узлу", "url", url, "error", err)
			lastErr = err
			if errors.Is(err, context.DeadlineExceeded) && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				logger.Warn("Отмена операции подключения из-за таймаута родительского контекста")
				return nil, ctx.Err()
			}
			if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
				logger.Warn("Отмена операции подключения из-за отмены родительского контекста")
				return nil, ctx.Err()
			}
		}
	}

	logger.Error("Не удалось подключиться ни к одному из указанных EVM узлов", "last_error", lastErr)
	return nil, fmt.Errorf("%w: %w", ErrEvmClientCreationFailed, lastErr)
}

// Close terminates the underlying RPC connection
func (c *Client) Close() {
	if c.ethClient != nil {
		logger.Debug("Закрытие соединения с EVM клиентом")
		c.ethClient.Close()
	}
}

// GetChainID returns the chain ID associated with the client connection
func (c *Client) GetChainID() *big.Int {
	return c.chainID
}

// GetBalance retrieves the native token balance for a given address
func (c *Client) GetBalance(ctx context.Context, address common.Address) (*big.Int, error) {
	return c.ethClient.BalanceAt(ctx, address, nil)
}

// GetNonce retrieves the next nonce for an account
func (c *Client) GetNonce(ctx context.Context, address common.Address) (uint64, error) {
	return c.ethClient.PendingNonceAt(ctx, address)
}

// SuggestGasPrice suggests a gas price for legacy transactions
func (c *Client) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return c.ethClient.SuggestGasPrice(ctx)
}

// SuggestGasTipCap suggests a gas tip cap for EIP-1559 transactions
func (c *Client) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return c.ethClient.SuggestGasTipCap(ctx)
}

// EstimateGasLimit estimates the gas needed for a transaction
func (c *Client) EstimateGasLimit(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	return c.ethClient.EstimateGas(ctx, msg)
}

// SendRawTransaction sends a signed transaction to the network
func (c *Client) SendRawTransaction(ctx context.Context, tx *types.Transaction) error {
	logger.Debug("Отправка подписанной транзакции", "tx_hash", tx.Hash().Hex())
	err := c.ethClient.SendTransaction(ctx, tx)
	if err != nil {
		logger.Error("Не удалось отправить транзакцию", "tx_hash", tx.Hash().Hex(), "error", err)
		return fmt.Errorf("sending transaction failed: %w", err)
	}
	logger.Info("Транзакция успешно отправлена", "tx_hash", tx.Hash().Hex())
	return nil
}

// WaitForReceipt waits for a transaction receipt, polling the network
func (c *Client) WaitForReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	logger.Debug("Ожидание квитанции транзакции", "tx_hash", txHash.Hex())
	for {
		receipt, err := c.ethClient.TransactionReceipt(ctx, txHash)
		if err == nil && receipt != nil {
			logger.Info("Квитанция транзакции получена", "tx_hash", txHash.Hex(), "status", receipt.Status)
			return receipt, nil
		}
		if err != nil && !errors.Is(err, ethereum.NotFound) {
			logger.Warn("Ошибка при проверке квитанции транзакции", "tx_hash", txHash.Hex(), "error", err)
			return nil, fmt.Errorf("error fetching receipt: %w", err)
		}

		select {
		case <-time.After(5 * time.Second):
			continue
		case <-ctx.Done():
			logger.Warn("Контекст отменен во время ожидания квитанции", "tx_hash", txHash.Hex())
			return nil, ctx.Err()
		}
	}
}
