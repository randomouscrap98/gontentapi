package utils

// Condense given set of ids down to a minimal set (distinct only)
// and turn it into an any array. This is useful for large IN queries
// where you know a lot of the ids are probably repeats and don't want
// 1000 params in your query.
func UniqueParams[k comparable](ids ...k) []any {
	uidset := make(map[k]struct{})
	params := make([]any, 0, len(ids))
	var s struct{}
	for _, uid := range ids {
		_, ok := uidset[uid]
		if !ok {
			uidset[uid] = s
			params = append(params, uid)
		}
	}
	return params
}
