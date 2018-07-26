package model

import (
	"fmt"
)

func (c *MissingCoordinate) EnsureID() {
	if c.Id == "" {
		c.Id = fmt.Sprintf("%3.5f-%3.5f", c.Lat, c.Lon)
	}
}
