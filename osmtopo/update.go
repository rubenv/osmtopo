package osmtopo

import (
	"github.com/omniscale/imposm3/diff/parser"
	"github.com/rubenv/osmtopo/osmtopo/model"
)

type Update struct {
	Store    *Store
	Filename string
}

func (u *Update) Run() error {
	changes, err := parser.Parse(u.Filename)

	for {
		select {
		case c, ok := <-changes:
			if !ok {
				break
			}

			err := u.process(c)
			if err != nil {
				return err
			}
		case e, ok := <-err:
			if !ok {
				err = nil
			}
			return e
		}
	}

	return nil

}

func (u *Update) process(c parser.DiffElem) error {
	if c.Del {
		if c.Node != nil {
			n, err := u.Store.GetNode(c.Node.Id)
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
			n, err := u.Store.GetWay(c.Way.Id)
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
			n, err := u.Store.GetRelation(c.Rel.Id)
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
			u.Store.addNewNodes([]*model.Node{NodeFromEl(*c.Node)})
		}
		if c.Way != nil {
			u.Store.addNewWays([]*model.Way{WayFromEl(*c.Way)})
		}
		if c.Rel != nil {
			u.Store.addNewRelations([]*model.Relation{RelationFromEl(*c.Rel)})
		}
	}
	return nil
}
