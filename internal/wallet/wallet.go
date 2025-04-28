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
	ErrWalletsFileNotFound       = errors.New("wallets file not found")
	ErrWalletFileReadFailed      = errors.New("failed to read wallets file")
	ErrWalletInvalidKey          = errors.New("invalid private key format")
	ErrPublicKeyExtractionFailed = errors.New("failed to extract public key")
	ErrNoValidKeysFound          = errors.New("no valid private keys found in the file")
)

// Wallet stores the private key and address
type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	Address    common.Address
}

// LoadWallets reads private keys from a file and returns a slice of Wallet pointers.
func LoadWallets(path string, log logger.Logger) ([]*Wallet, error) {
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

		wallet, _ := parsePrivateKey(privateKeyHex, lineNumber, path, log)
		if wallet != nil {
			wallets = append(wallets, wallet)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("сканирование файла кошельков '%s': %w: %w", path, ErrWalletFileReadFailed, err)
	}

	if len(wallets) == 0 {
		log.Error("В файле не найдено валидных приватных ключей", "file", path)
		return nil, fmt.Errorf("%w в файле '%s'", ErrNoValidKeysFound, path)
	}

	return wallets, nil
}

// parsePrivateKey converts a hex private key string into a Wallet struct.
func parsePrivateKey(privateKeyHex string, lineNumber int, filePath string, log logger.Logger) (*Wallet, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		log.Warn("Неверный формат приватного ключа",
			"line", lineNumber, "file", filePath, "error", ErrWalletInvalidKey)
		return nil, nil
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Warn("Не удалось извлечь публичный ключ ECDSA",
			"line", lineNumber, "file", filePath, "error", ErrPublicKeyExtractionFailed)
		return nil, nil
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	return &Wallet{
		PrivateKey: privateKey,
		Address:    address,
	}, nil
}
