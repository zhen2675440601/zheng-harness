package store

import (
	"database/sql"
	"encoding/json"

	"zheng-harness/internal/domain"
)

func marshalProvenance(provenance *domain.Provenance) (string, error) {
	if provenance == nil {
		return "", nil
	}
	normalized := provenance.Normalize()
	if err := normalized.Validate(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(provenance)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func parseProvenance(raw sql.NullString) (*domain.Provenance, error) {
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}
	var provenance domain.Provenance
	if err := json.Unmarshal([]byte(raw.String), &provenance); err != nil {
		return nil, err
	}
	normalized := provenance.Normalize()
	return &normalized, nil
}

func mergeStoredProvenance(existing, incoming *domain.Provenance) *domain.Provenance {
	if existing == nil {
		if incoming == nil {
			return nil
		}
		normalized := incoming.Normalize()
		if err := normalized.Validate(); err != nil {
			return nil
		}
		return &normalized
	}
	if incoming == nil {
		normalized := existing.Normalize()
		if err := normalized.Validate(); err != nil {
			return nil
		}
		return &normalized
	}
	merged := existing.Normalize()
	for _, plugin := range incoming.Normalize().Plugins {
		duplicate := false
		for _, current := range merged.Plugins {
			if current.Family == plugin.Family && current.LogicalID == plugin.LogicalID {
				duplicate = true
				break
			}
		}
		if !duplicate {
			merged.Plugins = append(merged.Plugins, plugin)
		}
	}
	merged = merged.Normalize()
	if len(merged.Plugins) == 0 {
		return nil
	}
	if err := merged.Validate(); err != nil {
		return nil
	}
	return &merged
}
