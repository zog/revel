package controllers

import (
	"appengine/datastore"
	"code.google.com/p/go.crypto/bcrypt"
	"github.com/robfig/revel"
	"github.com/robfig/revel/samples/booking-appengine/app/models"
)

type Application struct {
	AppEngineController
}

func (c Application) Index() rev.Result {
	if c.Connected() != nil {
		return c.Redirect("/hotels")
	}
	return c.Render()
}

func (c Application) Register() rev.Result {
	return c.Render()
}

func (c Application) SaveUser(user models.User, verifyPassword string) rev.Result {
	c.Validation.Required(verifyPassword)
	c.Validation.Required(verifyPassword == user.Password).
		Message("Password does not match")
	user.Validate(c.Validation)

	if c.Validation.HasErrors() {
		c.Validation.Keep()
		c.FlashParams()
		return c.Redirect("/register")
	}

	user.HashedPassword, _ = bcrypt.GenerateFromPassword(
		[]byte(user.Password), bcrypt.DefaultCost)

	ctx := c.Context()
	k := datastore.NewKey(ctx, "User", user.Username, 0, nil)
	if _, err := datastore.Put(ctx, k, user); err != nil {
		panic(err)
	}

	c.Session["user"] = user.Username
	c.Flash.Success("Welcome, " + user.Name)
	return c.Redirect("/hotels")
}

func (c Application) Login(username, password string) rev.Result {
	user := c.GetUser(username)
	if user != nil {
		err := bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(password))
		if err == nil {
			c.Session["user"] = username
			c.Flash.Success("Welcome, " + username)
			return c.Redirect("/hotels")
		}
	}

	c.Flash.Out["username"] = username
	c.Flash.Error("Login failed")
	return c.Redirect("/")
}

func (c Application) Logout() rev.Result {
	for k := range c.Session {
		delete(c.Session, k)
	}
	return c.Redirect("/")
}
