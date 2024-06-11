package contentapi

import (
	"time"
)

// Basic user data from database (not all)
type User struct {
	Id       int64     `db:"id"`
	Username string    `db:"username"`
	Avatar   string    `db:"avatar"`
	Created  time.Time `db:"createDate"`
	Super    bool      `db:"super"`
}

// Basic content data from database (not all)
type Content struct {
	Id       int64     `db:"id"`
	Name     string    `db:"name"`
	Hash     string    `db:"hash"`
	Text     string    `db:"text"`
	ParentId int64     `db:"parentId"`
	Created  time.Time `db:"createDate"`

	Values     map[string]string
	CreateUser *User
}

// Basic COMMENT data from database (no modules in this system)
type Comment struct {
	Id        int64     `db:"id"`
	ContentId int64     `db:"contentId"`
	Created   time.Time `db:"createDate"`
	Text      string    `db:"text"`

	Values     map[string]string
	CreateUser *User
}
