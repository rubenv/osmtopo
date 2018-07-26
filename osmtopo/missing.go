package osmtopo

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/rubenv/osmtopo/osmtopo/model"
)

type CoordinateInfo struct {
	Coordinate  *model.MissingCoordinate         `json:"coordinate"`
	Suggestions map[string][]*RelationSuggestion `json:"suggestions"`
}

type RelationSuggestion struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	AdminLevel int    `json:"admin_level"`
}

func (e *Env) importMissing(in io.Reader) error {
	missing := make([]*model.MissingCoordinate, 0)
	err := json.NewDecoder(in).Decode(&missing)
	if err != nil {
		return err
	}

	err = e.addMissing(missing)
	if err != nil {
		return err
	}

	c, err := e.countMissing()
	if err != nil {
		return err
	}

	e.Status.Missing = c
	return nil
}

func (e *Env) getMissingCoordinate() (*CoordinateInfo, error) {
	c, err := e.getMissing()
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, nil
	}

	info := &CoordinateInfo{
		Coordinate:  c,
		Suggestions: make(map[string][]*RelationSuggestion),
	}

	for _, layer := range e.config.Layers {
		suggestions := make([]*RelationSuggestion, 0)
		for _, admin := range layer.AdminLevels {
			matches := e.lookup.query(c.Lat, c.Lon, admin)

			for _, match := range matches {
				rel, err := e.GetRelation(match)
				if err != nil {
					return nil, err
				}
				if rel == nil {
					return nil, fmt.Errorf("Cannot find relation for match %d", match)
				}

				name, _ := rel.GetTag("name")
				suggestions = append(suggestions, &RelationSuggestion{
					ID:         match,
					Name:       name,
					AdminLevel: admin,
				})
			}
		}
		info.Suggestions[layer.ID] = suggestions
	}

	return info, nil
}
