// +build appengine

package rev

import (
	"net/http"
	"path"
)

// A dummy Watcher interface to make handler.go compile.
type Watcher struct{}

func (w *Watcher) Notify() error { return nil }

var (
	MainRouter         *Router
	MainTemplateLoader *TemplateLoader
	MainWatcher        *Watcher
	Server             *http.Server
)

// Run the server.
func Run(port int) {
	MainRouter = NewRouter(path.Join(BasePath, "conf", "routes"))
	MainTemplateLoader = NewTemplateLoader(TemplatePaths)

	MainTemplateLoader.Refresh()
	MainRouter.Refresh()
	plugins.OnRoutesLoaded(MainRouter)
	plugins.OnAppStart()
}
