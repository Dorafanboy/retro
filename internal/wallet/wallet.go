package wallet

import (
	"bufio"
	"crypto/ecdsa"
	"fmt"
	"os"
	"strings"

	"retro_template/internal/logger"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Wallet stores the private key and address
type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	Address    common.Address
}

// LoadWallets reads private keys from a file and returns Wallet objects
func LoadWallets(path string) ([]*Wallet, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open wallets file %s: %w", path, err)
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
			logger.Warn("Не удалось распознать приватный ключ", "line", lineNumber, "error", err)
			continue
		}

		publicKey := privateKey.Public()
		publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			logger.Warn("Не удалось получить публичный ключ из приватного", "line", lineNumber)
			continue
		}

		address := crypto.PubkeyToAddress(*publicKeyECDSA)
		wallets = append(wallets, &Wallet{
			PrivateKey: privateKey,
			Address:    address,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning wallets file %s: %w", path, err)
	}

	if len(wallets) == 0 {
		logger.Error("Не найдено валидных приватных ключей в файле", "file", path)
		return nil, fmt.Errorf("no valid private keys found in %s", path)
	}

	return wallets, nil
}
