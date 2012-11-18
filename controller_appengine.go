// +build appengine

package rev

import (
	"fmt"
	"reflect"
)

// Redirect to an action or to a URL.
//   c.Redirect("/controller/action")
//   c.Redirect("/controller/%d/action", id)
func (c *Controller) Redirect(url string, args ...interface{}) Result {
	if len(args) == 0 {
		return &RedirectToUrlResult{url}
	}
	return &RedirectToUrlResult{fmt.Sprintf(url, args...)}
}

// Return the reflect.Method, given a Receiver type and Func value.
func FindMethod(recvType reflect.Type, funcVal reflect.Value) *reflect.Method {
	panic("unimplementable on appengine")
	return nil
}
