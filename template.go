package revel

import (
	"fmt"
	"html"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"bytes"
	"github.com/realistschuckle/gohaml"
)

var ERROR_CLASS = "hasError"

// This object handles loading and parsing of templates.
// Everything below the application's views directory is treated as a template.
type TemplateLoader struct {
	// This is the set of all templates under views
	templateSet *template.Template
	// If an error was encountered parsing the templates, it is stored here.
	compileError *Error
	// Paths to search for templates, in priority order.
	paths []string
	// Map from template name to the path from whence it was loaded.
	templatePaths map[string]string
	// templateNames is a map from lower case template name to the real template name.
	templateNames map[string]string
}

type Template interface {
	Name() string
	Content() []string
	Render(wr io.Writer, arg interface{}) error
}

var invalidSlugPattern = regexp.MustCompile(`[^a-z0-9 _-]`)
var whiteSpacePattern = regexp.MustCompile(`\s+`)

var (
	// The functions available for use in the templates.
	TemplateFuncs = map[string]interface{}{
		"url": ReverseUrl,
		"set": func(renderArgs map[string]interface{}, key string, value interface{}) template.JS {
			renderArgs[key] = value
			return template.JS("")
		},
		"append": func(renderArgs map[string]interface{}, key string, value interface{}) template.JS {
			if renderArgs[key] == nil {
				renderArgs[key] = []interface{}{value}
			} else {
				renderArgs[key] = append(renderArgs[key].([]interface{}), value)
			}
			return template.JS("")
		},
		"field": NewField,
		"firstof": func(args ...interface{}) interface{} {
			for _, val := range args {
				switch val.(type) {
				case nil:
					continue
				case string:
					if val == "" {
						continue
					}
					return val
				default:
					return val
				}
			}
			return nil
		},
		"option": func(f *Field, val interface{}, label string) template.HTML {
			selected := ""
			if f.Flash() == val || (f.Flash() == "" && f.Value() == val) {
				selected = " selected"
			}

			return template.HTML(fmt.Sprintf(`<option value="%s"%s>%s</option>`,
				html.EscapeString(fmt.Sprintf("%v", val)), selected, html.EscapeString(label)))
		},
		"radio": func(f *Field, val string) template.HTML {
			checked := ""
			if f.Flash() == val {
				checked = " checked"
			}
			return template.HTML(fmt.Sprintf(`<input type="radio" name="%s" value="%s"%s>`,
				html.EscapeString(f.Name), html.EscapeString(val), checked))
		},
		"checkbox": func(f *Field, val string) template.HTML {
			checked := ""
			if f.Flash() == val {
				checked = " checked"
			}
			return template.HTML(fmt.Sprintf(`<input type="checkbox" name="%s" value="%s"%s>`,
				html.EscapeString(f.Name), html.EscapeString(val), checked))
		},
		// Pads the given string with &nbsp;'s up to the given width.
		"pad": func(str string, width int) template.HTML {
			if len(str) >= width {
				return template.HTML(html.EscapeString(str))
			}
			return template.HTML(html.EscapeString(str) + strings.Repeat("&nbsp;", width-len(str)))
		},

		"errorClass": func(name string, renderArgs map[string]interface{}) template.HTML {
			errorMap, ok := renderArgs["errors"].(map[string]*ValidationError)
			if !ok || errorMap == nil {
				WARN.Println("Called 'errorClass' without 'errors' in the render args.")
				return template.HTML("")
			}
			valError, ok := errorMap[name]
			if !ok || valError == nil {
				return template.HTML("")
			}
			return template.HTML(ERROR_CLASS)
		},

		"msg": func(renderArgs map[string]interface{}, message string, args ...interface{}) template.HTML {
			str, ok := renderArgs[CurrentLocaleRenderArg].(string)
			if !ok {
				return ""
			}
			return template.HTML(Message(str, message, args...))
		},

		// Replaces newlines with <br>
		"nl2br": func(text string) template.HTML {
			return template.HTML(strings.Replace(template.HTMLEscapeString(text), "\n", "<br>", -1))
		},

		// Skips sanitation on the parameter.  Do not use with dynamic data.
		"raw": func(text string) template.HTML {
			return template.HTML(text)
		},

		// Pluralize, a helper for pluralizing words to correspond to data of dynamic length.
		// items - a slice of items, or an integer indicating how many items there are.
		// pluralOverrides - optional arguments specifying the output in the
		//     singular and plural cases.  by default "" and "s"
		"pluralize": func(items interface{}, pluralOverrides ...string) string {
			singular, plural := "", "s"
			if len(pluralOverrides) >= 1 {
				singular = pluralOverrides[0]
				if len(pluralOverrides) == 2 {
					plural = pluralOverrides[1]
				}
			}

			switch v := reflect.ValueOf(items); v.Kind() {
			case reflect.Int:
				if items.(int) != 1 {
					return plural
				}
			case reflect.Slice:
				if v.Len() != 1 {
					return plural
				}
			default:
				ERROR.Println("pluralize: unexpected type: ", v)
			}
			return singular
		},

		// Format a date according to the application's default date(time) format.
		"date": func(date time.Time) string {
			return date.Format(DateFormat)
		},
		"datetime": func(date time.Time) string {
			return date.Format(DateTimeFormat)
		},
		"slug": Slug,
		"even": func(a int) bool { return (a % 2) == 0 },
		"content": func(renderArgs map[string]interface{}) template.HTML {
			name := renderArgs["ContentTemplate"].(string)
			layoutName := renderArgs["LayoutTemplate"].(string)

			layout, err := MainTemplateLoader.Template("layouts/" + layoutName + ".haml")
			if err != nil {
				layout, err = MainTemplateLoader.Template("layouts/" + layoutName + ".html")
			}

			var tpl *template.Template
			for _, t := range layout.(GoTemplate).Templates(){
				if(t.Name() == name){
					tpl = t
				}
			}

			if(tpl == nil){
				ERROR.Println("Missing template: ", name)
				// error := ErrorResult{fmt.Errorf("Server Error:\n\n"+
				// 	"Missing layout: \n%s", name)}
				var out bytes.Buffer
				tmpl, _ := MainTemplateLoader.Template("errors/500.html")
				revelError := &Error{
					Title:       "Server Error",
					Description: fmt.Sprintf("Missing template: %s \n Present templates: %s", name, MainTemplateLoader),
				}
				renderArgs["Error"] = revelError
				tmpl.Render(&out, renderArgs)
				return template.HTML(out.String())
			}

			var out bytes.Buffer
			err = GoTemplate{tpl, MainTemplateLoader}.Render(&out, renderArgs)
			if err != nil {
				var out bytes.Buffer
				fmt.Println(err)

				tmpl, _ := MainTemplateLoader.Template("errors/500.html")
				revelError := &Error{
					Title:       "Server Error",
					Description: err.Error(),
				}
				renderArgs["Error"] = revelError
				tmpl.Render(&out, renderArgs)
				return template.HTML(out.String())
			}
			res := out.String()
			return template.HTML(res)
		},

		"layout": func(name string, contentName string, renderArgs map[string]interface{}) template.HTML {
			renderArgs["ContentTemplate"] = contentName
			renderArgs["LayoutTemplate"] = name
			layout, err := MainTemplateLoader.Template("layouts/" + name + ".haml")
			if err != nil {
				layout, err = MainTemplateLoader.Template("layouts/" + name + ".html")
			}
			if err != nil {
				var out bytes.Buffer

				ERROR.Println("Missing layout: ", name)
				// error := ErrorResult{fmt.Errorf("Server Error:\n\n"+
				// 	"Missing layout: \n%s", name)}
				tmpl, _ := MainTemplateLoader.Template("errors/500.html")
				revelError := &Error{
					Title:       "Server Error",
					Description: fmt.Sprintf("Missing layout: %s", name),
				}
				renderArgs["Error"] = revelError
				tmpl.Render(&out, renderArgs)
				return template.HTML(out.String())
			}

			var out bytes.Buffer
			err = layout.Render(&out, renderArgs)
			if err != nil {
				var out bytes.Buffer
				tmpl, _ := MainTemplateLoader.Template("errors/500.html")
				revelError := &Error{
					Title:       "Server Error",
					Description: err.Error(),
				}
				renderArgs["Error"] = revelError
				tmpl.Render(&out, renderArgs)
				return template.HTML(out.String())
			}

			res := out.String()
			return template.HTML(res)
		},
	}
)

func NewTemplateLoader(paths []string) *TemplateLoader {
	loader := &TemplateLoader{
		paths: paths,
	}
	return loader
}

// This scans the views directory and parses all templates as Go Templates.
// If a template fails to parse, the error is set on the loader.
// (It's awkward to refresh a single Go Template)
func (loader *TemplateLoader) Refresh() *Error {
	TRACE.Printf("Refreshing templates from %s", loader.paths)

	loader.compileError = nil
	loader.templatePaths = map[string]string{}
	loader.templateNames = map[string]string{}

	// Set the template delimiters for the project if present, then split into left
	// and right delimiters around a space character
	var splitDelims []string
	if TemplateDelims != "" {
		splitDelims = strings.Split(TemplateDelims, " ")
		if len(splitDelims) != 2 {
			log.Fatalln("app.conf: Incorrect format for template.delimiters")
		}
	}

	// Walk through the template loader's paths and build up a template set.
	var templateSet *template.Template = nil
	for _, basePath := range loader.paths {
		// Walk only returns an error if the template loader is completely unusable
		// (namely, if one of the TemplateFuncs does not have an acceptable signature).

		// Handling symlinked directories
		var fullSrcDir string
		f, err := os.Lstat(basePath)
		if err == nil && f.Mode()&os.ModeSymlink == os.ModeSymlink {
			fullSrcDir, err = filepath.EvalSymlinks(basePath)
			if err != nil {
				panic(err)
			}
		} else {
			fullSrcDir = basePath
		}

		var templateWalker func(path string, info os.FileInfo, err error) error
		templateWalker = func(path string, info os.FileInfo, err error) error {
			if err != nil {
				ERROR.Println("error walking templates:", err)
				return nil
			}

			// is it a symlinked template?
			link, err := os.Lstat(path)
			if err == nil && link.Mode()&os.ModeSymlink == os.ModeSymlink {
				TRACE.Println("symlink template:", path)
				// lookup the actual target & check for goodness
				targetPath, err := filepath.EvalSymlinks(path)
				if err != nil {
					ERROR.Println("Failed to read symlink", err)
					return err
				}
				targetInfo, err := os.Stat(targetPath)
				if err != nil {
					ERROR.Println("Failed to stat symlink target", err)
					return err
				}

				// set the template path to the target of the symlink
				path = targetPath
				info = targetInfo

				// need to save state and restore for recursive call to Walk on symlink
				tmp := fullSrcDir
				fullSrcDir = filepath.Dir(targetPath)
				filepath.Walk(targetPath, templateWalker)
				fullSrcDir = tmp
			}

			// Walk into watchable directories
			if info.IsDir() {
				if !loader.WatchDir(info) {
					return filepath.SkipDir
				}
				return nil
			}

			// Only add watchable
			if !loader.WatchFile(info.Name()) {
				return nil
			}

			var fileStr string

			// addTemplate loads a template file into the Go template loader so it can be rendered later
			addTemplate := func(templateName string) (err error) {
				// Convert template names to use forward slashes, even on Windows.
				if os.PathSeparator == '\\' {
					templateName = strings.Replace(templateName, `\`, `/`, -1) // `
				}

				// If we already loaded a template of this name, skip it.
				lowerTemplateName := strings.ToLower(templateName)
				if _, ok := loader.templateNames[lowerTemplateName]; ok {
					return nil
				}

				loader.templatePaths[templateName] = path
				loader.templateNames[lowerTemplateName] = templateName

				// Load the file if we haven't already
				if fileStr == "" {
					fileBytes, err := ioutil.ReadFile(path)
					if err != nil {
						ERROR.Println("Failed reading file:", path)
						return nil
					}
					fileStr = string(fileBytes)
				}

				nameSplit := strings.Split(templateName, ".")
				if len(nameSplit) >= 2 {
					extension := nameSplit[len(nameSplit)-1]
					if(extension == "haml"){
						var scope = make(map[string]interface{})
			      scope["lang"] = "HAML"

			      re := regexp.MustCompile("\"(.*?)\"")
			      fileStr = re.ReplaceAllStringFunc(fileStr, func(str string) (string){
				      re := regexp.MustCompile("#{(.*?)}")
				      str = re.ReplaceAllString(str, "__7777__${1}__9999__")
			      	return str
			      })

			      re = regexp.MustCompile("\"(.*?__7777__.*?),([^\"]*?__9999__.*?)\"")
			      fileStr = re.ReplaceAllString(fileStr, "\"${1}__888__${2}\"")
				    // fmt.Println(fileStr)
			      engine, _ := gohaml.NewEngine(fileStr)
			      fileStr = engine.Render(scope)
			      fileStr = strings.Replace(fileStr, "__7777__", "{{", -1)
			      fileStr = strings.Replace(fileStr, "__9999__", "}}", -1)
			      fileStr = strings.Replace(fileStr, "__888__", ",", -1)
			      // fmt.Println(fileStr)
					}
				}

				if templateSet == nil {
					// Create the template set.  This panics if any of the funcs do not
					// conform to expectations, so we wrap it in a func and handle those
					// panics by serving an error page.
					var funcError *Error
					func() {
						defer func() {
							if err := recover(); err != nil {
								funcError = &Error{
									Title:       "Panic (Template Loader)",
									Description: fmt.Sprintln(err),
								}
							}
						}()
						templateSet = template.New(templateName).Funcs(TemplateFuncs)
						// If alternate delimiters set for the project, change them for this set
						if splitDelims != nil && basePath == ViewsPath {
							templateSet.Delims(splitDelims[0], splitDelims[1])
						} else {
							// Reset to default otherwise
							templateSet.Delims("", "")
						}

						_, err = templateSet.Parse(fileStr)
					}()

					if funcError != nil {
						return funcError
					}

				} else {
					if splitDelims != nil && basePath == ViewsPath {
						templateSet.Delims(splitDelims[0], splitDelims[1])
					} else {
						templateSet.Delims("", "")
					}
					_, err = templateSet.New(templateName).Parse(fileStr)
				}
				return err
			}

			templateName := path[len(fullSrcDir)+1:]

			err = addTemplate(templateName)

			// Store / report the first error encountered.
			if err != nil && loader.compileError == nil {
				_, line, description := parseTemplateError(err)
				loader.compileError = &Error{
					Title:       "Template Compilation Error",
					Path:        templateName,
					Description: description,
					Line:        line,
					SourceLines: strings.Split(fileStr, "\n"),
				}
				ERROR.Printf("Template compilation error (In %s around line %d):\n%s",
					templateName, line, description)
			}
			return nil
		}

		funcErr := filepath.Walk(fullSrcDir, templateWalker)

		// If there was an error with the Funcs, set it and return immediately.
		if funcErr != nil {
			loader.compileError = funcErr.(*Error)
			return loader.compileError
		}
	}

	// Note: compileError may or may not be set.
	loader.templateSet = templateSet
	return loader.compileError
}

func (loader *TemplateLoader) WatchDir(info os.FileInfo) bool {
	// Watch all directories, except the ones starting with a dot.
	return !strings.HasPrefix(info.Name(), ".")
}

func (loader *TemplateLoader) WatchFile(basename string) bool {
	// Watch all files, except the ones starting with a dot.
	return !strings.HasPrefix(basename, ".")
}

// Parse the line, and description from an error message like:
// html/template:Application/Register.html:36: no such template "footer.html"
func parseTemplateError(err error) (templateName string, line int, description string) {
	description = err.Error()
	i := regexp.MustCompile(`:\d+:`).FindStringIndex(description)
	if i != nil {
		line, err = strconv.Atoi(description[i[0]+1 : i[1]-1])
		if err != nil {
			ERROR.Println("Failed to parse line number from error message:", err)
		}
		templateName = description[:i[0]]
		if colon := strings.Index(templateName, ":"); colon != -1 {
			templateName = templateName[colon+1:]
		}
		templateName = strings.TrimSpace(templateName)
		description = description[i[1]+1:]
	}
	return templateName, line, description
}

// Return the Template with the given name.  The name is the template's path
// relative to a template loader root.
//
// An Error is returned if there was any problem with any of the templates.  (In
// this case, if a template is returned, it may still be usable.)
func (loader *TemplateLoader) Template(name string) (Template, error) {
	// Case-insensitive matching of template file name
	templateName := loader.templateNames[strings.ToLower(name)]

	// Look up and return the template.
	tmpl := loader.templateSet.Lookup(templateName)

	// This is necessary.
	// If a nil loader.compileError is returned directly, a caller testing against
	// nil will get the wrong result.  Something to do with casting *Error to error.
	var err error
	if loader.compileError != nil {
		err = loader.compileError
	}

	if tmpl == nil && err == nil {
		return nil, fmt.Errorf("Template %s not found.", name)
	}

	return GoTemplate{tmpl, loader}, err
}

// Adapter for Go Templates.
type GoTemplate struct {
	*template.Template
	loader *TemplateLoader
}

// return a 'revel.Template' from Go's template.
func (gotmpl GoTemplate) Render(wr io.Writer, arg interface{}) error {
	return gotmpl.Execute(wr, arg)
}

func (gotmpl GoTemplate) Content() []string {
	content, _ := ReadLines(gotmpl.loader.templatePaths[gotmpl.Name()])
	return content
}

/////////////////////
// Template functions
/////////////////////

// Return a url capable of invoking a given controller method:
// "Application.ShowApp 123" => "/app/123"
func ReverseUrl(args ...interface{}) (template.URL, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("no arguments provided to reverse route")
	}

	action := args[0].(string)
	if action == "Root" {
		return template.URL(AppRoot), nil
	}
	actionSplit := strings.Split(action, ".")
	if len(actionSplit) != 2 {
		return "", fmt.Errorf("reversing '%s', expected 'Controller.Action'", action)
	}

	// Look up the types.
	var c Controller
	if err := c.SetAction(actionSplit[0], actionSplit[1]); err != nil {
		return "", fmt.Errorf("reversing %s: %s", action, err)
	}

	if len(c.MethodType.Args) < len(args)-1 {
		return "", fmt.Errorf("reversing %s: route defines %d args, but received %d",
			action, len(c.MethodType.Args), len(args)-1)
	}

	// Unbind the arguments.
	argsByName := make(map[string]string)
	for i, argValue := range args[1:] {
		Unbind(argsByName, c.MethodType.Args[i].Name, argValue)
	}

	return template.URL(MainRouter.Reverse(args[0].(string), argsByName).Url), nil
}

func Slug(text string) string {
	separator := "-"
	text = strings.ToLower(text)
	text = invalidSlugPattern.ReplaceAllString(text, "")
	text = whiteSpacePattern.ReplaceAllString(text, separator)
	text = strings.Trim(text, separator)
	return text
}

