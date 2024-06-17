package contentapi

import ()

type Query struct {
	Sql    string // The actual SQL for the query
	Params []any  // The parameters added in the right order
	Limit  int    // limit the results by this amount
	Skip   int    // Skip this many results
	Order  string // The order clause. Add desc yourself, but not ORDER BY
}

// Create a new query
func NewQuery() Query {
	return Query{
		Params: make([]any, 0),
		Limit:  -1,
		Skip:   -1,
	}
}

func (q *Query) AddParams(params ...any) {
	q.Params = append(q.Params, params...)
}

// Add the query for viewable. Make sure you already have a where clause
func (q *Query) AndViewable(cidField string, user int64) {
	q.Sql += " AND deleted = 0 AND " + cidField +
		" IN (SELECT contentId FROM content_permissions WHERE read = 1 AND userId IN (?, ?))"
	q.Params = append(q.Params, 0, user)
}

// Add the finishing touches (limit, skip, etc)
func (q *Query) Finalize() {
	if q.Order != "" {
		q.Sql += " ORDER BY " + q.Order
	}
	if q.Limit >= 0 {
		q.Sql += " LIMIT ?"
		q.Params = append(q.Params, q.Limit)
	}
	if q.Skip >= 0 {
		q.Sql += " OFFSET ?"
		q.Params = append(q.Params, q.Skip)
	}
}
