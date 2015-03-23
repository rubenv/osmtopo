package osmtopo

import (
	"strconv"

	"github.com/omniscale/imposm3/diff/parser"
)

type Update struct {
	Store    *Store
	Filename string
}

func (u *Update) Run() error {
	changes, err := parser.Parse(u.Filename)

	for {
		c, ok := <-changes
		if !ok {
			break
		}

		err := u.process(c)
		if err != nil {
			return err
		}
	}

	if e, ok := <-err; ok && e != nil {
		return e
	}

	return nil
}

func (u *Update) process(c parser.DiffElem) error {
	if c.Del {
		if c.Node != nil {
			n, err := u.Store.GetNode(strconv.FormatInt(c.Node.Id, 10))
			if err != nil {
				return err
			}

			if n != nil {
				err = u.Store.removeNode(n)
				if err != nil {
					return err
				}
			}
		}
		if c.Way != nil {
			n, err := u.Store.GetWay(strconv.FormatInt(c.Way.Id, 10))
			if err != nil {
				return err
			}

			if n != nil {
				err = u.Store.removeWay(n)
				if err != nil {
					return err
				}
			}
		}
		if c.Rel != nil {
			n, err := u.Store.GetRelation(strconv.FormatInt(c.Rel.Id, 10))
			if err != nil {
				return err
			}

			if n != nil {
				err = u.Store.removeRelation(n)
				if err != nil {
					return err
				}
			}
		}

	}

	if c.Add {
		if c.Node != nil {
			u.Store.addNewNodes([]*Node{NodeFromEl(*c.Node)})
		}
		if c.Way != nil {
			u.Store.addNewWays([]*Way{WayFromEl(*c.Way)})
		}
		if c.Rel != nil {
			u.Store.addNewRelations([]*Relation{RelationFromEl(*c.Rel)})
		}
	}
	return nil
}
