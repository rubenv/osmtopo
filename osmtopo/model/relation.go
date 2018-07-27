package model

import "strconv"

func (r *Relation) GetTag(key string) (string, bool) {
	if r.Tags == nil {
		return "", false
	}

	for _, e := range r.Tags {
		if e.Key == key {
			return e.Value, true
		}
	}

	return "", false
}

func (r *Relation) GetAdminLevel() int {
	t, ok := r.GetTag("admin_level")
	if !ok {
		return 0
	}

	al, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return 0
	}
	return int(al)
}
