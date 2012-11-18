package controllers

import (
	"appengine"
	"appengine/datastore"
	"github.com/robfig/revel"
	"github.com/robfig/revel/samples/booking-appengine/app/models"
)

type AppEngineController struct {
	*rev.Controller
}

func (c AppEngineController) Context() appengine.Context {
	const key = "appengine.Context"
	if ctx, ok := c.Args[key]; ok {
		return ctx.(appengine.Context)
	}
	ctx := appengine.NewContext(c.Request.Request)
	c.Args[key] = ctx
	return ctx
}

func (c AppEngineController) AddUser() rev.Result {
	if user := c.Connected(); user != nil {
		c.RenderArgs["user"] = user
	}
	return nil
}

func (c AppEngineController) Connected() *models.User {
	if c.RenderArgs["user"] != nil {
		return c.RenderArgs["user"].(*models.User)
	}
	if username, ok := c.Session["user"]; ok {
		return c.GetUser(username)
	}
	return nil
}

func (c AppEngineController) GetUser(username string) *models.User {
	// k := datastore.NewKey(ctx, "User", "", 0, nil)
	query := datastore.NewQuery("User").
		Filter("Username = ", username).
		Limit(1)
	t := query.Run(c.Context())

	var user models.User
	_, err := t.Next(&user)
	if err != nil {
		if err == datastore.Done {
			return nil
		}
		panic(err)
	}

	// if err := datastore.Get(ctx, k, &user); err != nil {
	// 	rev.INFO.Println("Couldn't get:", err)
	// 	return nil
	// }
	return &user
}

func init() {
	rev.InterceptMethod(AppEngineController.AddUser, rev.BEFORE)
}
