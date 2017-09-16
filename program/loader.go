package program

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/monopole/mdrip/lexer"
	"github.com/monopole/mdrip/model"
	"github.com/monopole/mdrip/util"
	"io"
)

// Tutorial UX Overview.
//
// Suppose it's a tutorial on Benelux.
//
// The first lesson is an overview of Benelux, with sibling (not child) lessons
// covering Belgium, Netherlands, and Luxembourg.  These in turn could contain
// lessons on provinces, which could contain lessons on cities, etc.
//
// Associated content REST addresses look like
//
//     benelux.com/overview                  // Describes Benelux in general.
//     benelux.com/history                   // Benelux history, economy, etc.
//     benelux.com/economy
//     benelux.com/belgium/overview          // Describes Belgium in general.
//     benelux.com/belgium/tintin            // Dive into important details.
//     benelux.com/belgium/beer
//     benelux.com/belgium/antwerp/overview  // Dive into Antwerp, etc.
//     benelux.com/belgium/antwerp/diamonds
//     benelux.com/belgium/antwerp/rubens
//     benelux.com/belgium/east-flanders
//     benelux.com/belgium/brabant
//     ...
//     benelux.com/netherlands/overview
//     benelux.com/netherlands/drenthe
//     benelux.com/netherlands/flevoland
//     ...
//
// Crucially, all content is accessible from a left nav in a page like this:
//
//      overview     |                             {content outline
//      belgium      |                              here - title, h1,
//     [netherlands] |       {main page             h2, h3 etc.}
//      luxembourg   |      content here}
//
// The core interaction here is that
//   * At all times exactly one of the left nav choices is selected.
//   * The main page shows content associated with that selection.
// It's always obvious where you are, where you can go, and how to get back.
//
// The first item, in this case "overview" is the initial highlight.
// If one hits the domain without a REST path, one is redirected to
// /overview and that item is highlighted in the menu, and its
// content is shown.
//
// Items in the left nav either name content and show it when clicked, or
// they name sub-tutorials and expand sub-tutorial choices when clicked.
// In the latter case, the main content and the left nav highlighting
// _do not change_.  A second click hides the exposed sub-tutorial names.
//
// Only the name of a Lesson (a leaf) with content can 1) be highlighted,
// 2) change the main page content when clicked, and 3) serve at a meaningful
// REST address.  Everything else is a sub-tutorial, and only expands or hides
// its own appearance.
//
// By design, this scheme maps to this filesystem layout:
//
//     benelux/
//       01_history.md
//       02_economy.md
//       README.md
//       03_belgium/
//         01_tintin.md
//         02_beer.md
//         03_antwerp/
//           README.md
//           01_diamonds.md
//           ...
//         04_east-flanders.md
//         05_brabant.md
//         ...
//       04_netherlands/
//         README.md
//         01_drenthe.md
//         02_flevoland.md
//       ...
//
// Where, say README (a github name convention) is converted to "overview"
// by a file loader.
//
// The proposed command line to read and serve content is
//
//      mdrip --mode web /foo/benelux
// or
//      mdrip --mode web /foo/benelux/README.md
//
// i.e. the argument names either a directory or a file.
//
// If the arg is a directory name, the tree below it is read in an attempt
// to build RESTfully addressable content and UX.  The names shown in the UX
// could be raw file names or could be processed a bit, e.g. underscores or
// hyphens become spaces, the ordering of the content in the UX could be
// controlled by omittable numerical prefixes on file names, etc.
// Errors in tree structure dealt with reasonably or cause immediate server
// failure.
//
// If only one file is read, then only that content is shown -
// no left nav needed.

type TutVisitor interface {
	VisitLesson(l *Lesson)
	VisitCourse(c *Course)
	VisitTopCourse(t *TopCourse)
}

type TutorialPrinter struct {
	indent int
	w io.Writer
}

func NewTutorialPrinter (w io.Writer) *TutorialPrinter  {
	return &TutorialPrinter{0, w}
}

func (v *TutorialPrinter) spaces(indent int) string {
	if indent < 1 {
		return ""
	}
	return fmt.Sprintf("%"+strconv.Itoa(indent)+"s", " ")
}

func (v *TutorialPrinter) VisitLesson(l *Lesson) {
	fmt.Fprintf(v.w,
		v.spaces(v.indent)+"%s --- %s...\n",
		l.Name(), util.SampleString(l.Content(), 60))
}

func (v *TutorialPrinter) VisitCourse(c *Course) {
	fmt.Fprintf(v.w, v.spaces(v.indent)+"%s\n", c.Name())
	v.indent += 3
	for _, x := range c.children {
		x.Accept(v)
	}
	v.indent -= 3
}

func (v *TutorialPrinter) VisitTopCourse(t *TopCourse) {
	for _, x := range t.children {
		x.Accept(v)
	}
}

type TutorialParser struct {
	label       model.Label
	parsedFiles []*model.ParsedFile
}

func NewTutorialParser(l model.Label) *TutorialParser {
	return &TutorialParser{l, []*model.ParsedFile{}}
}

func (v *TutorialParser) Files() []*model.ParsedFile {
	return v.parsedFiles
}

func (v *TutorialParser) VisitLesson(l *Lesson) {
	m := lexer.Parse(l.Content())
	// Parse returns a map of label to array of block for the given content.
	// The next line discards ALL block arrays save the one associated
	// with desired label, and accumulates that array.
	if blocks, ok := m[v.label]; ok {
		v.parsedFiles = append(v.parsedFiles, model.NewParsedFile(l.Path(), blocks))
	}
}

func (v *TutorialParser) VisitCourse(c *Course) {
	for _, x := range c.children {
		x.Accept(v)
	}
}

func (v *TutorialParser) VisitTopCourse(t *TopCourse) {
	for _, x := range t.children {
		x.Accept(v)
	}
}

type Tutorial interface {
	Name() string
	Path() model.FilePath
	Content() string
	// The order matters.
	Children() []Tutorial
	Accept(v TutVisitor)
}

// A Lesson, or file, must have a name, must have content and zero children.
type Lesson struct {
	filepath model.FilePath
	content  string
}

func (l *Lesson) Name() string         { return l.filepath.Base() }
func (l *Lesson) Path() model.FilePath { return l.filepath }
func (l *Lesson) Content() string      { return l.content }
func (l *Lesson) Children() []Tutorial { return []Tutorial{} }
func (l *Lesson) Accept(v TutVisitor) {
	v.VisitLesson(l)
}

// A Course, or directory, has a name, no content, and an ordered list of
// Lessons and Courses. If the list is empty, the Course is dropped.
type Course struct {
	filepath model.FilePath
	children []Tutorial
}

func (c *Course) Name() string         { return c.filepath.Base() }
func (c *Course) Path() model.FilePath { return c.filepath }
func (c *Course) Content() string      { return "" }
func (c *Course) Children() []Tutorial { return c.children }
func (c *Course) Accept(v TutVisitor) {
	v.VisitCourse(c)
}

// A TopCourse is a Course with no name - it's the root of the tree (benelux).
type TopCourse struct {
	filepath model.FilePath
	children []Tutorial
}

func (t *TopCourse) Name() string         { return "" }
func (t *TopCourse) Path() model.FilePath { return t.filepath }
func (t *TopCourse) Content() string      { return "" }
func (t *TopCourse) Children() []Tutorial { return t.children }
func (t *TopCourse) Accept(v TutVisitor) {
	v.VisitTopCourse(t)
}

const badLeadingChar = "~.#"

func isDesirableFile(n model.FilePath) bool {
	s, err := os.Stat(string(n))
	if err != nil {
		glog.Info("Stat error on "+s.Name(), err)
		return false
	}
	if s.IsDir() {
		glog.Info("Ignoring NON-file " + s.Name())
		return false
	}
	if !s.Mode().IsRegular() {
		glog.Info("Ignoring irregular file " + s.Name())
		return false
	}
	if filepath.Ext(s.Name()) != ".md" {
		glog.Info("Ignoring non markdown file " + s.Name())
		return false
	}
	base := filepath.Base(s.Name())
	if strings.Index(badLeadingChar, string(base[0])) > -1 {
		glog.Info("Ignoring because bad leading char: " + s.Name())
		return false
	}
	return true
}

func isDesirableDir(n model.FilePath) bool {
	s, err := os.Stat(string(n))
	if err != nil {
		glog.Info("Stat error on "+s.Name(), err)
		return false
	}
	if !s.IsDir() {
		glog.Info("Ignoring NON-dir " + s.Name())
		return false
	}
	if s.Name() == "." || s.Name() == "./" || s.Name() == ".." {
		// Allow special dir names.
		return true
	}
	if strings.HasPrefix(filepath.Base(s.Name()), ".") {
		glog.Info("Ignoring dot dir " + s.Name())
		// Ignore .git, etc.
		return false
	}
	return true
}

func scanDir(d model.FilePath) (*Course, error) {
	files, err := d.ReadDir()
	if err != nil {
		return nil, err
	}
	var items = []Tutorial{}
	for _, f := range files {
		p := d.Join(f)
		if isDesirableFile(p) {
			l, err := scanFile(p)
			if err != nil {
				return nil, err
			}
			items = append(items, l)
		} else if isDesirableDir(p) {
			c, err := scanDir(p)
			if err != nil {
				return nil, err
			}
			if c != nil {
				items = append(items, c)
			}
		}
	}
	if len(items) > 0 {
		return &Course{d, items}, nil
	}
	return nil, nil
}

func scanFile(n model.FilePath) (*Lesson, error) {
	contents, err := n.Read()
	if err != nil {
		return nil, err
	}
	return &Lesson{n, contents}, nil
}

func LoadOne(root model.FilePath) (Tutorial, error) {
	if isDesirableFile(root) {
		return scanFile(root)
	}
	if isDesirableDir(root) {
		c, err := scanDir(root)
		if err != nil {
			return nil, err
		}
		if c != nil {
			return &TopCourse{root, c.children}, nil
		}
	}
	return nil, errors.New("Cannot process " + string(root))
}

func LoadMany(fileNames []model.FilePath) (Tutorial, error) {
	if len(fileNames) == 0 {
		return nil, errors.New("no files?")
	}
	if len(fileNames) == 1 {
		return LoadOne(fileNames[0])
	}
	var items = []Tutorial{}
	for _, f := range fileNames {
		if isDesirableFile(f) {
			l, err := scanFile(f)
			if err != nil {
				return nil, err
			}
			items = append(items, l)
		} else if isDesirableDir(f) {
			c, err := scanDir(f)
			if err != nil {
				return nil, err
			}
			if c != nil {
				items = append(items, c)
			}
		}
	}
	if len(items) > 0 {
		return &TopCourse{model.FilePath(""), items}, nil
	}
	return nil, errors.New("Nothing useful found")
}
