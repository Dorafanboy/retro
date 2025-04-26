package wallet

import (
	"bufio"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"strings"

	"retro/internal/logger"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	// ErrWalletsFileNotFound indicates that the wallets file was not found.
	ErrWalletsFileNotFound = errors.New("wallets file not found")
	// ErrWalletFileReadFailed indicates an error occurred while reading the wallets file.
	ErrWalletFileReadFailed = errors.New("failed to read wallets file")
	// ErrWalletInvalidKey indicates an invalid private key format was encountered.
	ErrWalletInvalidKey = errors.New("invalid private key format")
	// ErrPublicKeyExtractionFailed indicates failure to extract public key from private key.
	ErrPublicKeyExtractionFailed = errors.New("failed to extract public key")
	// ErrNoValidKeysFound indicates that no valid private keys were found in the file.
	ErrNoValidKeysFound = errors.New("no valid private keys found in the file")
)

// Wallet stores the private key and address
type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	Address    common.Address
}

// LoadWallets reads private keys from a file and returns a slice of Wallet pointers.
func LoadWallets(path string) ([]*Wallet, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("файл кошельков '%s': %w", path, ErrWalletsFileNotFound)
		}
		return nil, fmt.Errorf("чтение файла кошельков '%s': %w: %w", path, ErrWalletFileReadFailed, err)
	}
	defer file.Close()

	var wallets []*Wallet
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		privateKeyHex := strings.TrimPrefix(line, "0x")

		privateKey, err := crypto.HexToECDSA(privateKeyHex)
		if err != nil {
			logger.Warn("Неверный формат приватного ключа", "line", lineNumber, "file", path, "error", ErrWalletInvalidKey)
			continue
		}

		publicKey := privateKey.Public()
		publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			logger.Warn("Не удалось извлечь публичный ключ ECDSA", "line", lineNumber, "file", path, "error", ErrPublicKeyExtractionFailed)
			continue
		}

		address := crypto.PubkeyToAddress(*publicKeyECDSA)
		wallets = append(wallets, &Wallet{
			PrivateKey: privateKey,
			Address:    address,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("сканирование файла кошельков '%s': %w: %w", path, ErrWalletFileReadFailed, err)
	}

	if len(wallets) == 0 {
		logger.Error("В файле не найдено валидных приватных ключей", "file", path)
		return nil, fmt.Errorf("%w в файле '%s'", ErrNoValidKeysFound, path)
	}

	return wallets, nil
}
