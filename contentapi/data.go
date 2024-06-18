package contentapi

import (
	"fmt"
	//"log"
	// "time"
)

// Basic user data from database (not all)
type User struct {
	Id       int64  `db:"id"`
	Username string `db:"username"`
	Avatar   string `db:"avatar"`
	Created  string `db:"createDate"`
	Super    bool   `db:"super"`
}

// Retrieve the list of user fields based on the table name
func GetUserFields(table string) string {
	if table != "" {
		table += "."
	}
	return fmt.Sprintf("%[1]sid,%[1]susername,%[1]savatar,%[1]screateDate,%[1]ssuper", table)
}

// Basic content data from database (not all)
type Content struct {
	Id           int64  `db:"id"`
	Name         string `db:"name"`
	Hash         string `db:"hash"`
	Text         string `db:"text"`
	ParentId     int64  `db:"parentId"`
	Created      string `db:"createDate"`
	ContentType  int    `db:"contentType"`
	CreateUserId int64  `db:"createUserId"`

	Private bool

	Values     map[string]string
	CreateUser *User
}

// Retrieve the list of content fields based on table name (REQUIRED).
// If you don't specify all fields, some larger fields are removed (useful
// for retrieving surface information for lists)
func GetContentFields(table string, allFields bool) string {
	// This is fine being a panic, you're using it wrong
	if table == "" {
		panic("You MUST set a table name!")
	}
	table += "."
	var bigFields string
	if allFields {
		bigFields = table + "text"
	} else {
		bigFields = "'' AS text"
	}
	privateQuery := "SELECT 1 FROM content_permissions WHERE contentId = %[1]sid AND userId = 0 AND read = 1"
	return fmt.Sprintf("%[1]sid,%[1]sname,%[1]shash,%[2]s,%[1]sparentId,%[1]scontentType,%[1]screateDate,%[1]screateUserId,NOT EXISTS ("+privateQuery+") AS private",
		table, bigFields)
}

// Basic COMMENT data from database (no modules in this system)
type Comment struct {
	Id           int64  `db:"id"`
	ContentId    int64  `db:"contentId"`
	Created      string `db:"createDate"`
	Text         string `db:"text"`
	CreateUserId int64  `db:"createUserId"`

	Values     map[string]string
	CreateUser *User
}

// Return all fields for SELECT for comment query
func GetCommentFields(table string) string {
	if table != "" {
		table += "."
	}
	return fmt.Sprintf("%[1]sid,%[1]scontentId,%[1]screateDate,%[1]stext,%[1]screateUserId", table)
}
