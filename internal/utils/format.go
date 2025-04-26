package utils

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/params"
)

var (
	// Precomputed powers of 10 for conversion
	pow10_9  = big.NewInt(params.GWei)
	pow10_18 = big.NewInt(params.Ether)

	// Use big.Float for more accurate scaling
	etherScale = new(big.Float).SetInt(pow10_18)
	gweiScale  = new(big.Float).SetInt(pow10_9)
)

// ToWei converts a decimal string (representing Ether) to *big.Int (Wei).
func ToWei(decimalAmount string) (*big.Int, error) {
	amountFloat, _, err := big.ParseFloat(decimalAmount, 10, 256, big.ToNearestEven)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга строки '%s' в число: %w", decimalAmount, err)
	}

	amountFloat.Mul(amountFloat, etherScale)

	weiAmount := new(big.Int)
	amountFloat.Int(weiAmount)

	return weiAmount, nil
}

// ToGwei converts a decimal string (representing Ether) to *big.Int (Gwei).
func ToGwei(decimalAmount string) (*big.Int, error) {
	amountFloat, _, err := big.ParseFloat(decimalAmount, 10, 256, big.ToNearestEven)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга строки '%s' в число: %w", decimalAmount, err)
	}

	amountFloat.Mul(amountFloat, gweiScale)

	gweiAmount := new(big.Int)
	amountFloat.Int(gweiAmount)

	return gweiAmount, nil
}

// FromWei converts a *big.Int (Wei) to a decimal string (Ether).
func FromWei(weiAmount *big.Int) string {
	if weiAmount == nil {
		return "0"
	}
	amountFloat := new(big.Float).SetInt(weiAmount)
	amountFloat.Quo(amountFloat, etherScale)
	return strings.TrimRight(strings.TrimRight(amountFloat.Text('f', 18), "0"), ".")
}

// FromGwei converts a *big.Int (Gwei) to a decimal string (Ether).
func FromGwei(gweiAmount *big.Int) string {
	if gweiAmount == nil {
		return "0"
	}
	amountFloat := new(big.Float).SetInt(gweiAmount)
	amountFloat.Quo(amountFloat, gweiScale)
	return strings.TrimRight(strings.TrimRight(amountFloat.Text('f', 9), "0"), ".")
}
