package controllers

import (
	"appengine/datastore"
	"code.google.com/p/go.crypto/bcrypt"
	"fmt"
	"github.com/robfig/revel"
	"github.com/robfig/revel/samples/booking-appengine/app/models"
	"strings"
)

type Hotels struct {
	AppEngineController
}

func (c Hotels) CheckUser() rev.Result {
	if user := c.Connected(); user == nil {
		c.Flash.Error("Please log in first")
		return c.Redirect("/")
	}
	return nil
}

func (c Hotels) Index() rev.Result {
	ctx := c.Context()
	t := datastore.NewQuery("Booking").
		Filter("Username =", c.Connected().Username).
		Run(ctx)

	var bookings []*models.Booking
	for {
		var booking models.Booking
		_, err := t.Next(&booking)
		if err == datastore.Done {
			break
		}
		if err != nil {
			panic(err)
		}

		bookings = append(bookings, &booking)
	}

	for _, b := range bookings {
		b.Hotel = c.loadHotelById(b.HotelId)
	}

	return c.Render(bookings)
}

func (c Hotels) List(search string, size, page int) rev.Result {
	if page == 0 {
		page = 1
	}
	nextPage := page + 1
	search = strings.TrimSpace(search)
	search = strings.ToLower(search)

	query := datastore.NewQuery("Hotel").
		Limit(size).
		Offset((page - 1) * size)
	if search != "" {
		query = query.Filter("Name = ", search)
	}

	ctx := c.Context()
	var hotels []*models.Hotel
	for t := query.Run(ctx); ; {
		var hotel models.Hotel
		_, err := t.Next(&hotel)
		if err == datastore.Done {
			break
		}
		if err != nil {
			panic(err)
		}

		hotels = append(hotels, &hotel)
	}

	return c.Render(hotels, search, size, page, nextPage)
}

func (c Hotels) loadHotelById(id int64) *models.Hotel {
	ctx := c.Context()
	k := datastore.NewKey(ctx, "Hotel", "", id, nil)
	var hotel models.Hotel
	if err := datastore.Get(ctx, k, &hotel); err != nil {
		panic(err)
	}
	return &hotel
}

func (c Hotels) Show(id int64) rev.Result {
	var title string
	hotel := c.loadHotelById(id)
	if hotel == nil {
		title = "Not found"
		// 	TODO: return c.NotFound("Hotel does not exist")
	} else {
		title = hotel.Name
	}
	return c.Render(title, hotel)
}

func (c Hotels) Settings() rev.Result {
	return c.Render()
}

func (c Hotels) SaveSettings(password, verifyPassword string) rev.Result {
	models.ValidatePassword(c.Validation, password)
	c.Validation.Required(verifyPassword).
		Message("Please verify your password")
	c.Validation.Required(verifyPassword == password).
		Message("Your password doesn't match")
	if c.Validation.HasErrors() {
		c.Validation.Keep()
		return c.Redirect("/hotels/settings")
	}

	user := c.Connected()
	user.HashedPassword, _ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	ctx := c.Context()
	key := datastore.NewKey(ctx, "User", user.Username, 0, nil)
	if _, err := datastore.Put(ctx, key, &user); err != nil {
		panic(err)
	}

	c.Flash.Success("Password updated")
	return c.Redirect("/hotels")
}

func (c Hotels) ConfirmBooking(id int64, booking models.Booking) rev.Result {
	hotel := c.loadHotelById(id)
	title := fmt.Sprintf("Confirm %s booking", hotel.Name)
	booking.Hotel = hotel
	booking.User = c.Connected()
	booking.Validate(c.Validation)

	if c.Validation.HasErrors() || c.Params.Get("revise") != "" {
		c.Validation.Keep()
		c.FlashParams()
		return c.Redirect("/hotels/%d/booking", id)
	}

	if c.Params.Get("confirm") != "" {
		ctx := c.Context()
		key := datastore.NewKey(ctx, "Booking", "", 0, nil)
		key, err := datastore.Put(ctx, key, &booking)
		if err != nil {
			panic(err)
		}

		bookingId := key.IntID()
		c.Flash.Success("Thank you, %s, your confirmation number for %s is %d",
			booking.User.Name, hotel.Name, bookingId)
		return c.Redirect("/hotels")
	}

	return c.Render(title, hotel, booking)
}

func (c Hotels) CancelBooking(id int64) rev.Result {
	ctx := c.Context()
	key := datastore.NewKey(ctx, "Booking", "", id, nil)
	if err := datastore.Delete(ctx, key); err != nil {
		panic(err)
	}
	c.Flash.Success(fmt.Sprintln("Booking cancelled for confirmation number", id))
	return c.Redirect("/hotels")
}

func (c Hotels) Book(id int64) rev.Result {
	hotel := c.loadHotelById(id)
	title := "Book " + hotel.Name
	// if hotel == nil {
	// 	return c.NotFound("Hotel does not exist")
	// }
	return c.Render(title, hotel)
}

func init() {
	rev.InterceptMethod(Hotels.CheckUser, rev.BEFORE)
}
