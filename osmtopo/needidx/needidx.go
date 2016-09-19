package needidx

type needTopLevel [256]*needLevel7
type needLevel7 [256]*needLevel6
type needLevel6 [256]*needLevel5
type needLevel5 [256]*needLevel4
type needLevel4 [256]*needLevel3
type needLevel3 [256]*needLevel2
type needLevel2 [256]*needLeaf
type needLeaf [256]bool

type NeedIdx struct {
	entries needTopLevel
}

func New() *NeedIdx {
	return &NeedIdx{}
}

func (n *NeedIdx) MarkNeeded(id int64) {
	k := byte(id >> 56)
	v8 := n.entries[k]
	if v8 == nil {
		v8 = &needLevel7{}
		n.entries[k] = v8
	}

	k = byte(id >> 48)
	v7 := v8[k]
	if v7 == nil {
		v7 = &needLevel6{}
		v8[k] = v7
	}

	k = byte(id >> 40)
	v6 := v7[k]
	if v6 == nil {
		v6 = &needLevel5{}
		v7[k] = v6
	}

	k = byte(id >> 32)
	v5 := v6[k]
	if v5 == nil {
		v5 = &needLevel4{}
		v6[k] = v5
	}

	k = byte(id >> 24)
	v4 := v5[k]
	if v4 == nil {
		v4 = &needLevel3{}
		v5[k] = v4
	}

	k = byte(id >> 16)
	v3 := v4[k]
	if v3 == nil {
		v3 = &needLevel2{}
		v4[k] = v3
	}

	k = byte(id >> 8)
	v2 := v3[k]
	if v2 == nil {
		v2 = &needLeaf{}
		v3[k] = v2
	}

	v2[byte(id)] = true
}

func (n *NeedIdx) IsNeeded(id int64) bool {
	k := byte(id >> 56)
	v8 := n.entries[k]
	if v8 == nil {
		return false
	}

	k = byte(id >> 48)
	v7 := v8[k]
	if v7 == nil {
		return false
	}

	k = byte(id >> 40)
	v6 := v7[k]
	if v6 == nil {
		return false
	}

	k = byte(id >> 32)
	v5 := v6[k]
	if v5 == nil {
		return false
	}

	k = byte(id >> 24)
	v4 := v5[k]
	if v4 == nil {
		return false
	}

	k = byte(id >> 16)
	v3 := v4[k]
	if v3 == nil {
		return false
	}

	k = byte(id >> 8)
	v2 := v3[k]
	if v2 == nil {
		return false
	}

	return v2[byte(id)]
}
