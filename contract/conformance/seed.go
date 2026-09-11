package conformance

import (
	"encoding/json"
	"os"
)

// LoadSeedIDs collects every id declared in contract/seed.json. Ids in
// this set are stable across apps and must not be normalized; anything
// else was minted at runtime.
func LoadSeedIDs(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for key, val := range doc {
		if key == "_notes" {
			continue
		}
		var rows []map[string]any
		if err := json.Unmarshal(val, &rows); err != nil {
			continue
		}
		for _, row := range rows {
			if id, ok := row["id"].(string); ok {
				ids[id] = true
			}
		}
	}
	return ids, nil
}
