package model

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
