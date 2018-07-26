package osmtopo

import (
	"encoding/json"
	"io"

	"github.com/rubenv/osmtopo/osmtopo/model"
)

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
