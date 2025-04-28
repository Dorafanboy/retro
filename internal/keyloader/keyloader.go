package keyloader

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
	ErrWalletsFileNotFound       = errors.New("key file not found")
	ErrWalletFileReadFailed      = errors.New("failed to read key file")
	ErrWalletInvalidKey          = errors.New("invalid private key format")
	ErrPublicKeyExtractionFailed = errors.New("failed to extract public key")
	ErrNoValidKeysFound          = errors.New("no valid private keys found in the file")
)

// LoadedKey stores the private key and address loaded from a source.
// It does not provide any signing capabilities itself.
type LoadedKey struct {
	PrivateKey *ecdsa.PrivateKey
	Address    common.Address
}

// LoadKeys reads private keys from a file and returns a slice of LoadedKey pointers.
// It expects one private key per line, optionally prefixed with "0x".
// Lines starting with '#' or empty lines are ignored.
func LoadKeys(path string, log logger.Logger) ([]*LoadedKey, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("файл ключей '%s': %w", path, ErrWalletsFileNotFound)
		}
		return nil, fmt.Errorf("чтение файла ключей '%s': %w: %w", path, ErrWalletFileReadFailed, err)
	}
	defer file.Close()

	var loadedKeys []*LoadedKey
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		privateKeyHex := strings.TrimPrefix(line, "0x")

		keyData, _ := parsePrivateKeyToLoadedKey(privateKeyHex, lineNumber, path, log)
		if keyData != nil {
			loadedKeys = append(loadedKeys, keyData)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("сканирование файла ключей '%s': %w: %w", path, ErrWalletFileReadFailed, err)
	}

	if len(loadedKeys) == 0 {
		log.Error("В файле не найдено валидных приватных ключей", "file", path)
		return nil, fmt.Errorf("%w в файле '%s'", ErrNoValidKeysFound, path)
	}

	return loadedKeys, nil
}

// parsePrivateKeyToLoadedKey converts a hex private key string into a LoadedKey struct.
// Returns nil if the key is invalid or public key extraction fails, logging a warning.
func parsePrivateKeyToLoadedKey(privateKeyHex string, lineNumber int, filePath string, log logger.Logger) (*LoadedKey, error) {
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
	return &LoadedKey{
		PrivateKey: privateKey,
		Address:    address,
	}, nil
}
