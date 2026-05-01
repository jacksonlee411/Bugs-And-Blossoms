package authz

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

type PolicyGrant struct {
	Subject string
	Domain  string
	Object  string
	Action  string
}

func ReadPolicyGrants(path string) ([]PolicyGrant, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParsePolicyGrants(f)
}

func ParsePolicyGrants(r io.Reader) ([]PolicyGrant, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	var grants []PolicyGrant
	for line := 1; ; line++ {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("policy line %d: %w", line, err)
		}
		if len(record) == 0 || blankCSVRecord(record) {
			continue
		}
		kind := strings.TrimSpace(record[0])
		switch kind {
		case "p":
			if len(record) != 5 {
				return nil, fmt.Errorf("policy line %d: p record must have 5 fields", line)
			}
			grant := PolicyGrant{
				Subject: strings.TrimSpace(record[1]),
				Domain:  strings.TrimSpace(record[2]),
				Object:  strings.TrimSpace(record[3]),
				Action:  strings.TrimSpace(record[4]),
			}
			if grant.Subject == "" || grant.Domain == "" || grant.Object == "" || grant.Action == "" {
				return nil, fmt.Errorf("policy line %d: %w", line, ErrEmptyPolicyGrant)
			}
			grants = append(grants, grant)
		case "g":
			continue
		default:
			return nil, fmt.Errorf("policy line %d: invalid record kind %q", line, kind)
		}
	}
	return grants, nil
}

func blankCSVRecord(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}
