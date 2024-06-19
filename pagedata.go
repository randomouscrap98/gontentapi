package main

import (
	"database/sql"
	"fmt"
	"log"
	"slices"

	"github.com/randomouscrap98/gontentapi/contentapi"
	"github.com/randomouscrap98/gontentapi/utils"
)

type Search struct {
	Search string  `schema:"search"`
	User   int64   `schema:"user"`
	Types  []int64 `schema:"types"`
}

func (gctx *GonContext) AddSearchResults(search *Search, user *UserSession, data map[string]any) error {
	// var uid int64
	// if user != nil {
	// 	uid = int64(user.Uid)
	// }

	return nil
}

// Add all the page data (main page, subpages, etc) for
func (gctx *GonContext) AddPageData(hash string, user *UserSession, data map[string]any) error {
	var uid int64
	if user != nil {
		uid = int64(user.Uid)
	}

	var mainpage contentapi.Content

	if hash == "" {
		// This is the root page
		MakeRoot(&mainpage)
	} else {
		q := contentapi.NewQuery()
		q.Sql = "SELECT " + contentapi.GetContentFields("c", true) + " FROM content c WHERE c.hash = ?"
		q.AddParams(hash)
		q.AndViewable("c.id", uid)
		q.Finalize()
		err := gctx.contentdb.Get(&mainpage, q.Sql, q.Params...)

		if err != nil {
			if err == sql.ErrNoRows {
				return &utils.NotFound{Message: fmt.Sprintf("No content with hash %s", hash)}
			} else {
				return err
			}
		}
	}

	q := contentapi.NewQuery()
	q.Sql = "SELECT " + contentapi.GetContentFields("c", false) + " FROM content c " +
		"WHERE c.parentId = ? AND c.contentType <> ?"
	q.AddParams(mainpage.Id, contentapi.ContentType_File)
	q.AndViewable("c.id", uid)
	q.Order = "c.name"
	q.Finalize()

	subpages := make([]contentapi.Content, 0)
	err := gctx.contentdb.Select(&subpages, q.Sql, q.Params...)

	if err != nil {
		return err
	}

	breadcrumbs := make([]*contentapi.Content, 0)
	pid := mainpage.Id // Include the current page in the breadcrumbs

	// Loop to make breadcrumbs
	for pid != 0 {
		q := contentapi.NewQuery()
		q.Sql = "SELECT " + contentapi.GetContentFields("c", false) + " FROM content c " +
			"WHERE c.id = ?"
		q.AddParams(pid)
		q.AndViewable("c.id", uid)
		q.Finalize()

		var breadcrumb contentapi.Content
		err := gctx.contentdb.Get(&breadcrumb, q.Sql, q.Params...)
		if err != nil {
			if err == sql.ErrNoRows {
				break // Nothing left to do?
			} else {
				return err
			}
		}

		breadcrumbs = slices.Insert(breadcrumbs, 0, &breadcrumb)
		pid = breadcrumb.ParentId
	}

	// Always insert root?
	breadcrumbs = slices.Insert(breadcrumbs, 0, MakeRoot(nil))

	// We need to lookup users for everything
	users, err := gctx.GetUsers(mainpage.CreateUserId)
	if err != nil {
		return err
	}

	usermap := contentapi.GetMappedUsers(users)

	// Apply users to content as needed
	if mainpage.Id > 0 {
		if mainpage.ApplyUser(usermap) == nil {
			log.Printf("WARN: couldn't find user for page %s (%d)", mainpage.Name, mainpage.Id)
		}

		// Find number of comments (just for fun)
		q = contentapi.NewQuery()
		q.Sql = "SELECT COUNT(*) FROM messages WHERE contentId = ?"
		q.AddParams(mainpage.Id)
		q.AndCommentViewable("")
		q.Finalize()

		var count int64
		err = gctx.contentdb.Get(&count, q.Sql, q.Params...)
		if err != nil {
			return err
		}

		data["numcomments"] = count
	}

	// Because everything is a struct rather than a pointer, this actually gets copied in.
	// Probably bad but whatever (this is why we can't change it after the fact)
	data["title"] = mainpage.Name
	data["mainpage"] = &mainpage // Everything expects a pointer
	data["subpages"] = subpages
	data["breadcrumbs"] = breadcrumbs

	return nil
}

func (gctx *GonContext) AddCommentData(hash string, user *UserSession, page int, data map[string]any) ([]contentapi.Comment, error) {
	var uid int64
	if user != nil {
		uid = int64(user.Uid)
	}

	var mainpage contentapi.Content

	// Still need to lookup main page to make sure they have access to it
	if hash == "" {
		return nil, &utils.BadRequest{Message: "Must specify a page hash to view comments!"}
	} else {
		q := contentapi.NewQuery()
		q.Sql = "SELECT " + contentapi.GetContentFields("c", false) + " FROM content c WHERE c.hash = ?"
		q.AddParams(hash)
		q.AndViewable("c.id", uid)
		q.Finalize()
		err := gctx.contentdb.Get(&mainpage, q.Sql, q.Params...)

		if err != nil {
			if err == sql.ErrNoRows {
				return nil, &utils.NotFound{Message: fmt.Sprintf("No content with hash %s", hash)}
			} else {
				return nil, err
			}
		}
	}

	// Get comments
	q := contentapi.NewQuery()
	q.Sql = "SELECT " + contentapi.GetCommentFields("m") + " FROM messages m WHERE m.contentId = ?"
	q.AddParams(mainpage.Id)
	q.AndCommentViewable("m")
	q.Order = "m.id DESC"
	q.Limit = gctx.config.CommentsPerPage
	q.Skip = gctx.config.CommentsPerPage * page
	q.Finalize()

	//log.Printf("Final comment query: " + q.Sql)
	//log.Printf(fmt.Sprintf("Final params: %v", q.Params))

	comments := make([]contentapi.Comment, 0)
	err := gctx.contentdb.Select(&comments, q.Sql, q.Params...)

	if err != nil {
		return nil, err
	}

	// Have to pull out only uids (maybe there's a better way, who knows)
	commentUids := make([]int64, len(comments)+1)
	for i := range comments {
		commentUids[i] = comments[i].CreateUserId
	}
	commentUids[len(comments)] = mainpage.CreateUserId

	// Need to look up users for each comment
	users, err := gctx.GetUsers(commentUids...)
	if err != nil {
		return nil, err
	}

	usermap := contentapi.GetMappedUsers(users)

	// Might as well apply the thing here (though I might remove it)
	if mainpage.ApplyUser(usermap) == nil {
		log.Printf("WARN: couldn't find user for page %s (%d)", mainpage.Name, mainpage.Id)
	}

	// Apply user for every comment. It's fine if they don't exist
	for i := range comments {
		if comments[i].ApplyUser(usermap) == nil {
			log.Printf("WARN: couldn't find user for page %s (%d)", mainpage.Name, mainpage.Id)
		}
	}

	data["mainpage"] = &mainpage // Everything expects a pointer
	data["comments"] = comments

	return comments, nil
}
