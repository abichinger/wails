package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/samber/lo"
	"github.com/wailsapp/wails/v3/internal/flags"
	"github.com/wailsapp/wails/v3/internal/hash"
	"golang.org/x/tools/go/packages"
)

type Parameter struct {
	*types.Var
	index int

	Parent *BoundMethod
}

func (p *Parameter) Name() (name string) {
	name = p.Var.Name()

	if name == "" || name == "_" {
		return "$" + strconv.Itoa(p.index)
	} else if slices.Contains(reservedWords, name) {
		return "$" + name
	}
	return name
}

func (p *Parameter) Variadic() bool {
	s := p.Parent.Signature()
	return s.Variadic() && p.index == s.Params().Len()-1
}

func (p *Package) namespaceOf(t *types.TypeName) string {
	if p.Types.String() == t.Pkg().String() {
		return ""
	}
	return t.Pkg().Name() + "."
}

// JSTypes returns the javascript type for the given types.Type
// The second return value indicates whether parentheses are needed
func JSType(t types.Type, pkg *Package) (string, bool) {

	switch x := t.(type) {
	case *types.Basic:
		switch x.Kind() {
		case types.String:
			return "string", false
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr, types.Float32, types.Float64:
			return "number", false
		case types.Bool:
			return "boolean", false
		default:
			return "any", false
		}
	case *types.Slice:
		jstype, needsParentheses := JSType(x.Elem(), pkg)
		if needsParentheses {
			return "(" + jstype + ")[]", false
		}
		return jstype + "[]", false
	case *types.Array:
		jstype, needsParentheses := JSType(x.Elem(), pkg)
		if needsParentheses {
			return "(" + jstype + ")[]", false
		}
		return jstype + "[]", false
	case *types.Named:
		return pkg.namespaceOf(x.Obj()) + x.Obj().Name(), false
	case *types.Map:
		jstype, _ := JSType(x.Elem(), pkg)
		return "{ [_: string]: " + jstype + " }", false
	case *types.Pointer:
		jstype, _ := JSType(x.Elem(), pkg)
		return jstype + " | null", true
	case *types.Struct:
		return pkg.anonymousStructID(x), false
	}
	return "any", false
}

func (p *Parameter) JSType(pkg *Package) string {
	jstype, _ := JSType(p.Type(), pkg)
	return jstype
}

type BoundMethod struct {
	*types.Func
	ID  uint32
	FQN string
}

func (m *BoundMethod) ParseTuple(tuple *types.Tuple) (result []*Parameter) {
	if tuple == nil {
		return
	}

	for i := 0; i < tuple.Len(); i++ {
		result = append(result, &Parameter{
			Var:    tuple.At(i),
			index:  i,
			Parent: m,
		})
	}
	return
}

func (m *BoundMethod) Signature() *types.Signature {
	// Type of *types.Func is always a *types.Signature
	return m.Type().(*types.Signature)
}

func (m *BoundMethod) Params() []*Parameter {
	tuple := m.Signature().Params()
	return m.ParseTuple(tuple)
}

func (m *BoundMethod) Results() []*Parameter {
	tuple := m.Signature().Results()
	return m.ParseTuple(tuple)
}

func (m *BoundMethod) JSInputs() []*Parameter {
	params := m.Params()

	if len(params) > 0 {
		if named, ok := params[0].Type().(*types.Named); ok && named.Obj() != nil {
			if named.Obj().Name() == "Context" && named.Obj().Pkg().Name() == "context" {
				return params[1:]
			}
		}
	}

	return params
}

func (m *BoundMethod) JSOutputs() (outputs []*Parameter) {
	for _, output := range m.Results() {
		if types.TypeString(output.Var.Type(), nil) == "error" {
			continue
		}
		outputs = append(outputs, output)
	}

	return outputs
}

type Service struct {
	*types.TypeName
	Methods []*BoundMethod
}

func ParseMethods(service *types.TypeName, main *packages.Package) (methods []*BoundMethod) {
	if named, ok := service.Type().(*types.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			fn := named.Method(i)
			if !fn.Exported() {
				continue
			}

			packagePath := service.Pkg().Path()
			// use "main" as package path if service is inside main package,
			// because reflect.Type.PkgPath() == "main"
			// https://github.com/golang/go/issues/8559
			if packagePath == main.Types.Path() {
				packagePath = "main"
			}

			fqn := fmt.Sprintf("%s.%s.%s", packagePath, service.Name(), fn.Name())

			id, err := hash.Fnv(fqn)
			if err != nil {
				panic("Failed to hash fqn")
			}

			method := &BoundMethod{
				Func: fn,
				FQN:  fqn,
				ID:   id,
			}

			interfaceFound := false
			for param := range method.Models(nil, false) {
				if types.IsInterface(param.Obj().Type()) {
					interfaceFound = true
					pterm.Warning.Printf("can't bind method %v with interface %v\n", fqn, param.Obj().Name())
				}
			}
			if interfaceFound {
				continue
			}

			methods = append(methods, method)
		}
	}
	return
}

type Package struct {
	*packages.Package
	project          *Project
	services         []*Service
	models           *Models
	anonymousStructs map[string]string
	doc              *Doc
}

func ParsePackages(pPkgs []*packages.Package, project *Project, buildFlags []string) ([]*Package, error) {
	result := make(map[string]*Package)

	// wrap types.Package
	for _, pPkg := range pPkgs {
		result[pPkg.Types.Path()] = &Package{
			Package:          pPkg,
			project:          project,
			services:         []*Service{},
			models:           NewModels(),
			anonymousStructs: make(map[string]string),
			doc:              NewDoc(pPkg),
		}
	}

	// helper function to load missing packages
	loadPackage := func(pkgPath string) (*Package, error) {
		pPkg, err := LoadPackage(buildFlags, true, pkgPath)
		if err != nil {
			return nil, err
		}

		return &Package{
			Package:          pPkg,
			project:          project,
			services:         []*Service{},
			models:           NewModels(),
			anonymousStructs: make(map[string]string),
			doc:              NewDoc(pPkg),
		}, nil
	}

	// add services to packages
	services, err := ParseServices(pPkgs, project.main)
	if err != nil {
		return nil, err
	}
	for _, service := range services {
		if pkg, ok := result[service.Pkg().Path()]; ok {
			pkg.addService(service)
		} else {
			// load missing packages of service
			pkg, err := loadPackage(service.Pkg().Path())
			if err != nil {
				return nil, err
			}
			pkg.addService(service)
			result[service.Pkg().Path()] = pkg
		}
	}

	// add models to packages
	allModels := []*types.Named{}
	for _, pkg := range result {
		allModels = append(allModels, lo.Keys(pkg.Models())...)
	}
	for _, model := range allModels {
		pkgPath := model.Obj().Pkg().Path()
		pkg, ok := result[pkgPath]
		if !ok {
			// load missing packages of models
			pkg, err = loadPackage(pkgPath)
			if err != nil {
				return nil, err
			}
			result[pkgPath] = pkg
		}
		pkg.addModel(model, project.marshaler, project.textMarshaler)
	}

	return lo.Values(result), nil
}

func (p *Package) addService(s *Service) {
	p.services = append(p.services, s)
}

func (p *Package) anonymousStructID(s *types.Struct) string {
	key := s.String()

	if _, ok := p.anonymousStructs[key]; !ok {
		p.anonymousStructs[key] = "$anon" + strconv.Itoa(len(p.anonymousStructs)+1)
	}
	return p.anonymousStructs[key]
}

// Credit: https://stackoverflow.com/a/70999797/3140799
func (p *Package) constantsOf(t *types.Named) []*ConstDef {
	values := []*ConstDef{}

	for _, file := range p.Syntax {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, specs := range genDecl.Specs {
				valueSpec, ok := specs.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range valueSpec.Names {
					c := p.TypesInfo.ObjectOf(name).(*types.Const)
					if strings.HasSuffix(c.Type().String(), t.Obj().Name()) {
						values = append(values, &ConstDef{Name: name.Name, Const: c})
					}
				}
			}
		}
	}
	return values
}

type Stats struct {
	NumPackages int
	NumServices int
	NumMethods  int
	NumModels   int
	NumEnums    int
	NumAliases  int
	StartTime   time.Time
	EndTime     time.Time
}

type Project struct {
	pkgs    []*Package
	main    *packages.Package
	options *flags.GenerateBindingsOptions
	Stats   Stats

	marshaler     *types.Interface
	textMarshaler *types.Interface
}

func loadMarshalerInterfaces() (*types.Interface, *types.Interface, error) {
	var marshaler, textMarshaler *types.Interface

	pkg, err := LoadPackage(nil, true, "encoding/json")
	if err != nil {
		return nil, nil, err
	}
	for _, t := range pkg.TypesInfo.Types {
		switch i := t.Type.Underlying().(type) {
		case *types.Interface:
			if named, ok := t.Type.(*types.Named); ok {
				if named.Obj().Name() == "Marshaler" {
					marshaler = i
				} else if named.Obj().Name() == "TextMarshaler" {
					textMarshaler = i
				}

			}
		}
	}
	if marshaler == nil {
		return nil, nil, errors.New("could not find interface json.Marshaler")
	}
	if textMarshaler == nil {
		return nil, nil, errors.New("could not find interface encoding.TextMarshaler")
	}
	return marshaler, textMarshaler, nil
}

func ParseProject(options *flags.GenerateBindingsOptions) (*Project, error) {
	startTime := time.Now()

	// load json interfaces
	marshaler, textMarshaler, err := loadMarshalerInterfaces()
	if err != nil {
		return nil, err
	}

	buildFlags, err := options.BuildFlags()
	if err != nil {
		return nil, err
	}

	pPkgs, err := LoadPackages(buildFlags, true,
		options.ProjectDirectory, WailsAppPkgPath,
	)
	if err != nil {
		return nil, err
	}
	if n := packages.PrintErrors(pPkgs); n > 0 {
		return nil, errors.New("error while loading packages")
	}

	mainIndex := slices.IndexFunc(pPkgs, func(pkg *packages.Package) bool { return pkg.Name == "main" })
	if mainIndex == -1 {
		return nil, errors.New("application.New() must be inside main package")
	}

	project := &Project{
		main:          pPkgs[mainIndex],
		options:       options,
		marshaler:     marshaler,
		textMarshaler: textMarshaler,
		Stats: Stats{
			StartTime: startTime,
		},
	}

	project.pkgs, err = ParsePackages(pPkgs, project, buildFlags)
	if err != nil {
		return project, err
	}

	project.Stats.NumPackages = len(project.pkgs)
	return project, nil
}

func GenerateBindingsAndModels(options *flags.GenerateBindingsOptions) (*Project, error) {
	p, err := ParseProject(options)
	if err != nil {
		return p, err
	}

	if NumMethods := len(p.BoundMethods()); NumMethods == 0 {
		return p, nil
	} else {
		p.Stats.NumMethods += NumMethods
	}

	err = os.MkdirAll(options.OutputDirectory, 0755)
	if err != nil {
		return p, err
	}

	// generate bindings
	generatedMethods, err := p.GenerateBindings()
	if err != nil {
		return p, err
	}
	for pkgDir, structs := range generatedMethods {
		// Write the directory
		err = os.MkdirAll(filepath.Join(options.OutputDirectory, pkgDir), 0755)
		if err != nil && !os.IsExist(err) {
			return p, err
		}
		// Write the files
		for structName, text := range structs {
			p.Stats.NumServices++
			var filename string
			if options.TS {
				filename = structName + ".ts"
			} else {
				filename = structName + ".js"
			}
			err = os.WriteFile(filepath.Join(options.OutputDirectory, pkgDir, filename), []byte(text), 0644)
			if err != nil {
				return p, err
			}
		}
	}

	// generate models
	generatedModels, err := p.GenerateModels()
	if err != nil {
		return p, err
	}
	for pkgDir, text := range generatedModels {
		// Write the directory
		err = os.MkdirAll(filepath.Join(options.OutputDirectory, pkgDir), 0755)
		if err != nil && !os.IsExist(err) {
			return p, err
		}
		// Write the file
		var filename string
		if options.TS {
			filename = options.ModelsFilename + ".ts"
		} else {
			filename = options.ModelsFilename + ".js"
		}
		err = os.WriteFile(filepath.Join(options.OutputDirectory, pkgDir, filename), []byte(text), 0644)
	}
	if err != nil {
		return p, err
	}

	p.Stats.EndTime = time.Now()

	return p, nil
}

func ParseServices(pkgs []*packages.Package, main *packages.Package) (services []*Service, err error) {
	var app *packages.Package
	otherPkgs := append(make([]*packages.Package, 0, len(pkgs)), pkgs...)
	if index := slices.IndexFunc(pkgs, func(pkg *packages.Package) bool { return pkg.PkgPath == WailsAppPkgPath }); index >= 0 {
		app = pkgs[index]
		otherPkgs = slices.Delete(otherPkgs, index, index+1)
	}

	if app == nil {
		err = errors.New("LoadPackages() did not load the application package")
		return
	}

	found, err := FindServices(app, otherPkgs)
	if err != nil {
		return
	}

	for _, service := range found {
		services = append(services, &Service{
			TypeName: service,
			Methods:  ParseMethods(service, main),
		})
	}
	return
}

func (p *Project) PackageDir(pkg *types.Package) string {
	root := p.main.Types.Path()
	if pkg.Path() == root {
		return "main"
	}

	if strings.HasPrefix(pkg.Path(), root) {
		path, err := filepath.Rel(root, pkg.Path())
		if err != nil {
			panic(err)
		}
		return filepath.ToSlash(path)
	}
	return strings.ReplaceAll(pkg.Path(), "/", "-")
}

func (p *Project) RelativePackageDir(base *types.Package, target *types.Package) string {
	if base == target {
		return "."
	}

	basePath := p.PackageDir(base)
	targetPath := p.PackageDir(target)

	relativePath, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		panic(err)
	}

	return filepath.ToSlash(relativePath)
}

func (p *Project) BoundMethods() []*BoundMethod {
	methods := []*BoundMethod{}

	for _, pkg := range p.pkgs {
		for _, service := range pkg.services {
			methods = append(methods, service.Methods...)
		}
	}
	return methods
}

var reservedWords = []string{
	"abstract",
	"arguments",
	"await",
	"boolean",
	"break",
	"byte",
	"case",
	"catch",
	"char",
	"class",
	"const",
	"continue",
	"debugger",
	"default",
	"delete",
	"do",
	"double",
	"else",
	"enum",
	"eval",
	"export",
	"extends",
	"false",
	"final",
	"finally",
	"float",
	"for",
	"function",
	"goto",
	"if",
	"implements",
	"import",
	"in",
	"instanceof",
	"int",
	"interface",
	"let",
	"long",
	"native",
	"new",
	"null",
	"package",
	"private",
	"protected",
	"public",
	"return",
	"short",
	"static",
	"super",
	"switch",
	"synchronized",
	"this",
	"throw",
	"throws",
	"transient",
	"true",
	"try",
	"typeof",
	"var",
	"void",
	"volatile",
	"while",
	"with",
	"yield",
	"object",
}
