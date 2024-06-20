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
	Search      string `schema:"search"`
	User        int64  `schema:"user"`
	Page        int    `schema:"page"`
	IgnoreTypes []int  `schema:"ignoretypes"`
	R           bool   `schema:"r"`
}

type IgnoreTypeData struct {
	Value   int
	Checked bool
}

func (search *Search) MakeInitialQuery(fields string, uid int64) contentapi.Query {

	q := contentapi.NewQuery()
	q.Sql = "SELECT " + fields + " FROM content c WHERE 1"
	if search.Search != "" {
		// This should get more complicated later
		searchAny := "%" + search.Search + "%"
		q.Sql += " AND (c.name LIKE ? OR c.hash LIKE ? OR EXISTS (SELECT 1 FROM content_keywords WHERE contentId = c.id AND value LIKE ?))"
		q.AddParams(searchAny, searchAny, searchAny)
	}
	if search.User != 0 {
		q.Sql += " AND c.createUserId = ?"
		q.AddParams(search.User)
	}
	if len(search.IgnoreTypes) > 0 {
		q.Sql += " AND c.contentType NOT IN ("
		q.AddQueryParams(utils.UniqueParams(search.IgnoreTypes...)...)
		q.Sql += ")"
	}
	//q.AddParams(mainpage.Id, contentapi.ContentType_File)
	q.AndViewable("c.id", uid)

	return q
}

type CommentSearch struct {
	Search string `schema:"search"`
	User   int64  `schema:"user"`
	Page   int    `schema:"page"`
	Start  string `schema:"start"`
	Oldest bool   `schema:"oldest"`
}

func (search *CommentSearch) MakeInitialQuery(fields string, contentId int64, uid int64) contentapi.Query {
	q := contentapi.NewQuery()
	q.Sql = "SELECT " + fields + " FROM messages m WHERE m.contentId = ?"
	q.AddParams(contentId)
	q.AndCommentViewable("m")
	if search.Search != "" {
		// This should get more complicated later
		searchAny := "%" + search.Search + "%"
		q.Sql += " AND m.text LIKE ?"
		//(c.name LIKE ? OR c.hash LIKE ? OR EXISTS (SELECT 1 FROM content_keywords WHERE contentId = c.id AND value LIKE ?))"
		q.AddParams(searchAny)
	}
	if search.User != 0 {
		q.Sql += " AND m.createUserId = ?"
		q.AddParams(search.User)
	}
	if search.Start != "" {
		q.Sql += " AND m.createDate > ?"
		q.AddParams(search.Start)
	}
	//q.AddParams(mainpage.Id, contentapi.ContentType_File)
	q.AndViewable("m.contentId", uid)
	return q
}

func (gctx *GonContext) AddSearchResults(search *Search, user *UserSession, data map[string]any) error {
	// doing too much here?
	ignoretypes := make(map[string]IgnoreTypeData)
	addignoretype := func(name string, value int) {
		ignoretypes[name] = IgnoreTypeData{Value: value, Checked: slices.Contains(search.IgnoreTypes, value)}
	}
	addignoretype("Pages", contentapi.ContentType_Page)
	addignoretype("Modules", contentapi.ContentType_Module)
	addignoretype("Files", contentapi.ContentType_File)
	addignoretype("Userpages", contentapi.ContentType_Userpage)

	data["ignoretypes"] = ignoretypes

	if !search.R {
		return nil
	}

	var uid int64
	if user != nil {
		uid = int64(user.Uid)
	}

	// Figure out basic count for user info
	q := search.MakeInitialQuery("COUNT(*)", uid)
	var count int64
	err := gctx.contentdb.Get(&count, q.Sql, q.Params...)
	if err != nil {
		return err
	}

	skip := gctx.config.CommentsPerPage * search.Page

	q = search.MakeInitialQuery(contentapi.GetContentFields("c", false), uid)
	q.Order = "c.id DESC"
	q.Limit = gctx.config.CommentsPerPage // Sure, why not
	q.Skip = skip
	q.Finalize()

	results := make([]contentapi.Content, 0)
	err = gctx.contentdb.Select(&results, q.Sql, q.Params...)

	if err != nil {
		return err
	}

	data["search"] = search
	data["results"] = results
	data["resultcount"] = count
	data["resultstart"] = skip + 1

	if len(results) > 0 {
		data["resultend"] = skip + len(results)
	}

	return nil
}

func (gctx *GonContext) AddCommentData(hash string, search *CommentSearch, user *UserSession, data map[string]any) ([]contentapi.Comment, error) {
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

	// Get count of "search" results
	q := search.MakeInitialQuery("COUNT(*)", mainpage.Id, uid)
	var count int64
	err := gctx.contentdb.Get(&count, q.Sql, q.Params...)
	if err != nil {
		return nil, err
	}

	skip := gctx.config.CommentsPerPage * search.Page

	q = search.MakeInitialQuery(contentapi.GetCommentFields("m"), mainpage.Id, uid)
	q.Order = "m.id"
	if !search.Oldest {
		q.Order += " DESC"
	}
	q.Limit = gctx.config.CommentsPerPage
	q.Skip = skip
	q.Finalize()

	comments := make([]contentapi.Comment, 0)
	err = gctx.contentdb.Select(&comments, q.Sql, q.Params...)

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

	data["search"] = search
	data["mainpage"] = &mainpage // Everything expects a pointer
	data["comments"] = comments
	data["resultcount"] = count
	data["resultstart"] = skip + 1

	if len(comments) > 0 {
		data["resultend"] = skip + len(comments)
	}

	return comments, nil
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
