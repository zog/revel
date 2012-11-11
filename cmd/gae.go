package main

import (
	"fmt"
	"github.com/robfig/revel"
	"github.com/robfig/revel/harness"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var cmdGae = &Command{
	UsageLine: "gae [import path] [destination path] [run mode]",
	Short:     "package a Revel application for Google AppEngine",
	Long: `
Copy the specified Revel app, its modules, and its transitive dependencies
to the given destination path, which should be the directory of your app.yaml 
configuration file.

For example:

    revel gae github.com/robfig/revel/samples/chat ./gae

If a run mode is not specified, "prod" is assumed.
`,
}

func init() {
	cmdGae.Run = gaeApp
}

func gaeApp(args []string) {
	if len(args) < 2 || len(args) > 3 {
		tmpl(os.Stderr, helpTemplate, cmdGae)
		return
	}

	appImportPath, destPath, runMode := args[0], args[1], "prod"
	if len(args) == 3 {
		runMode = args[2]
	}
	rev.Init(runMode, appImportPath, "")

	_, reverr := harness.Build()
	panicOnError(reverr, "Failed to build")

	// Prepare a build context.
	ctx := build.Default
	ctx.BuildTags = append(ctx.BuildTags, "appengine")

	// Get the set of transitive dependencies.
	type Dep struct{ ImportPath, SrcPath string }
	deps := []Dep{}
	queue := []Dep{{rev.ImportPath + "/app/tmp", path.Join(rev.AppPath, "tmp")}}
	importPathSet := map[string]struct{}{}
	for len(queue) > 0 {
		var dep Dep
		dep, queue = queue[0], queue[1:]

		pkg, err := ctx.Import(dep.ImportPath, dep.SrcPath, 0)
		if err != nil {
			errorf("Failed to import package: %s", err)
		}

		// Skip builtin packages.
		if pkg.Goroot {
			continue
		}

		deps = append(deps, dep)

		// Add this package's imports to the search queue.
		for _, importPath := range pkg.Imports {
			// Have we seen this import already?
			if _, ok := importPathSet[importPath]; ok {
				continue
			}

			importPathSet[importPath] = struct{}{}

			// Run an import in FindOnly to get the source directory.
			depPkg, err := ctx.Import(importPath, "", build.FindOnly)
			if err != nil {
				errorf("Failed to import %s: %s", importPath, err)
			}

			queue = append(queue, Dep{importPath, depPkg.Dir})
		}
	}

	// Copy the app, modules, and all of the dependencies to the destination.
	mustCopyDir(path.Join(destPath, filepath.FromSlash(appImportPath)), rev.BasePath, nil)
	for _, module := range rev.Modules {
		mustCopyDir(path.Join(destPath, filepath.FromSlash(module.ImportPath)), module.Path, nil)
	}
	for _, dep := range deps {
		if !strings.HasPrefix(dep.SrcPath, rev.BasePath) {
			mustCopyDirContents(path.Join(destPath, filepath.FromSlash(dep.ImportPath)), dep.SrcPath)
		}
	}

	// Additionally, copy some non-code stuff
	gaeRevelPath := path.Join(destPath, filepath.FromSlash(rev.REVEL_IMPORT_PATH))
	mustCopyDir(path.Join(gaeRevelPath, "conf"), path.Join(rev.RevelPath, "conf"), nil)
	mustCopyDir(path.Join(gaeRevelPath, "templates"), path.Join(rev.RevelPath, "templates"), nil)

	fmt.Println("Your GAE code is ready.")
}
