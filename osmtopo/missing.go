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
	Matched     map[string]bool                  `json:"matched"`
	MatchName   map[string]string                `json:"matchnames"`
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

	toAdd := make([]*model.MissingCoordinate, 0)
	for _, m := range missing {
		complete := true
		for _, layer := range e.config.Layers {
			matches := e.topologies.Query(m.Lat, m.Lon, layer.ID)
			if len(matches) == 0 {
				complete = false
			}
		}
		if !complete {
			toAdd = append(toAdd, m)
		}
	}

	err = e.addMissing(toAdd)
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
		Matched:     make(map[string]bool),
		MatchName:   make(map[string]string),
	}

	complete := true
	for _, layer := range e.config.Layers {
		matches := e.topologies.Query(c.Lat, c.Lon, layer.ID)
		if len(matches) == 0 {
			complete = false

			suggestions := make([]*RelationSuggestion, 0)
			matches := e.lookup.Query(c.Lat, c.Lon, layer.ID)
			for _, match := range matches {
				rel, err := e.GetRelation(match)
				if err != nil {
					return nil, err
				}
				if rel == nil {
					return nil, fmt.Errorf("Cannot find relation for match %d", match)
				}

				name, _ := rel.GetTag("name")
				admin_level := rel.GetAdminLevel()
				suggestions = append(suggestions, &RelationSuggestion{
					ID:         match,
					Name:       name,
					AdminLevel: admin_level,
				})
			}
			info.Suggestions[layer.ID] = suggestions
		} else {
			rel, err := e.GetRelation(matches[0])
			if err != nil {
				return nil, err
			}
			if rel == nil {
				return nil, fmt.Errorf("Cannot find relation for match %d", matches[0])
			}

			name, _ := rel.GetTag("name")
			info.Matched[layer.ID] = true
			info.MatchName[layer.ID] = name
		}
	}
	if complete {
		err = e.removeMissing(c)
		if err != nil {
			return nil, err
		}
		e.Status.Missing--

		return e.getMissingCoordinate()
	}

	return info, nil
}
