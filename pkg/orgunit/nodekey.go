package orgunit

import (
	"errors"
	"regexp"
	"strings"
)

const (
	OrgNodeKeyLength = 8

	orgNodeKeyGuardAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	orgNodeKeyBodyAlphabet  = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	orgNodeKeyBodyWidth     = 7
)

var (
	ErrOrgNodeKeyInvalid  = errors.New("org_node_key_invalid")
	ErrOrgNodeKeyNotFound = errors.New("org_node_key_not_found")

	orgNodeKeyPattern = regexp.MustCompile(`^[ABCDEFGHJKLMNPQRSTUVWXYZ][ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{7}$`)
	orgNodeKeyIndex   = buildAlphabetIndex(orgNodeKeyBodyAlphabet)
)

func NormalizeOrgNodeKey(input string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(input))
	if !orgNodeKeyPattern.MatchString(normalized) {
		return "", ErrOrgNodeKeyInvalid
	}
	return normalized, nil
}

func EncodeOrgNodeKey(seq int64) (string, error) {
	if seq <= 0 {
		return "", ErrOrgNodeKeyInvalid
	}

	base := int64(len(orgNodeKeyBodyAlphabet))
	capacityPerGuard := powInt64(base, orgNodeKeyBodyWidth)
	guardIndex := (seq / capacityPerGuard) % int64(len(orgNodeKeyGuardAlphabet))
	bodyValue := seq % capacityPerGuard

	body := make([]byte, orgNodeKeyBodyWidth)
	for i := orgNodeKeyBodyWidth - 1; i >= 0; i-- {
		body[i] = orgNodeKeyBodyAlphabet[bodyValue%base]
		bodyValue /= base
	}

	return string(orgNodeKeyGuardAlphabet[guardIndex]) + string(body), nil
}

func DecodeOrgNodeKey(input string) (int64, error) {
	key, err := NormalizeOrgNodeKey(input)
	if err != nil {
		return 0, err
	}

	base := int64(len(orgNodeKeyBodyAlphabet))
	capacityPerGuard := powInt64(base, orgNodeKeyBodyWidth)
	guardIndex := strings.IndexByte(orgNodeKeyGuardAlphabet, key[0])
	if guardIndex < 0 {
		return 0, ErrOrgNodeKeyInvalid
	}

	var bodyValue int64
	for i := 1; i < len(key); i++ {
		digit, ok := orgNodeKeyIndex[key[i]]
		if !ok {
			return 0, ErrOrgNodeKeyInvalid
		}
		bodyValue = bodyValue*base + int64(digit)
	}

	return int64(guardIndex)*capacityPerGuard + bodyValue, nil
}

func buildAlphabetIndex(alphabet string) map[byte]int {
	index := make(map[byte]int, len(alphabet))
	for i := range alphabet {
		index[alphabet[i]] = i
	}
	return index
}

func powInt64(base int64, exponent int) int64 {
	result := int64(1)
	for range exponent {
		result *= base
	}
	return result
}
