package architecture

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// typeRecord pairs one reported type declaration with its checker object.
// The checker object stays available for the implementation pass.
type typeRecord struct {
	declaration TypeDeclaration
	object      *types.TypeName
}

// inspectTypes extracts every named type declared in the analyzed packages.
// The result is sorted by identifier, so the graph output stays deterministic.
func (analyzer *analyzer) inspectTypes(
	ctx context.Context,
	modulePath string,
	fileSet *token.FileSet,
	loadedPackages []*packages.Package,
	sourcePaths []string,
) ([]TypeDeclaration, error) {
	selectedPaths := absolutePathSet(sourcePaths)
	inspectedPaths := make(stringSet)
	records := make([]*typeRecord, 0)
	for _, loadedPackage := range loadedPackages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if loadedPackage.TypesInfo == nil || strings.HasSuffix(loadedPackage.ID, ".test") {
			continue
		}
		for _, file := range loadedPackage.Syntax {
			absolutePath := filepath.Clean(physicalSourcePosition(fileSet, file.Pos()).Filename)
			if !selectedPaths.contains(absolutePath) || inspectedPaths.contains(absolutePath) {
				continue
			}
			if !functionPackageOwnsFile(loadedPackage, absolutePath) {
				continue
			}
			inspectedPaths.add(absolutePath)
			fileRecords, err := analyzer.typeDeclarationsInFile(
				modulePath,
				loadedPackage,
				fileSet,
				file,
			)
			if err != nil {
				return nil, err
			}
			records = append(records, fileRecords...)
		}
	}
	annotateImplementations(records)
	declarations := make([]TypeDeclaration, 0, len(records))
	for _, record := range records {
		declarations = append(declarations, record.declaration)
	}
	slices.SortFunc(declarations, func(first, second TypeDeclaration) int {
		return strings.Compare(first.Identifier, second.Identifier)
	})
	return declarations, nil
}

func (analyzer *analyzer) typeDeclarationsInFile(
	modulePath string,
	loadedPackage *packages.Package,
	fileSet *token.FileSet,
	file *ast.File,
) ([]*typeRecord, error) {
	records := make([]*typeRecord, 0)
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.TYPE {
			continue
		}
		for _, specification := range group.Specs {
			specifiedType, ok := specification.(*ast.TypeSpec)
			if !ok {
				continue
			}
			object, ok := loadedPackage.TypesInfo.Defs[specifiedType.Name].(*types.TypeName)
			if !ok {
				continue
			}
			record, classified, err := analyzer.typeDeclarationFromObject(modulePath, object, fileSet)
			if err != nil {
				return nil, err
			}
			if classified {
				records = append(records, record)
			}
		}
	}
	return records, nil
}

func (analyzer *analyzer) typeDeclarationFromObject(
	modulePath string,
	object *types.TypeName,
	fileSet *token.FileSet,
) (*typeRecord, bool, error) {
	if object.Pkg() == nil {
		return nil, false, nil
	}
	packagePath, local := localPackagePath(modulePath, object.Pkg().Path())
	if !local {
		return nil, false, nil
	}
	position := physicalSourcePosition(fileSet, object.Pos())
	relativePath, insideRepository := analyzer.relativeSourcePath(position.Filename)
	classificationPath := packagePath
	if insideRepository {
		classificationPath = relativePath
	}
	ignored, err := analyzer.ignoredPaths.matches(classificationPath)
	if err != nil {
		return nil, false, err
	}
	if ignored {
		return nil, false, nil
	}
	component, classified := analyzer.classifier.classify(classificationPath)
	if !classified {
		return nil, false, nil
	}
	declaration := TypeDeclaration{
		Identifier: packagePath + "." + object.Name(),
		Name:       object.Name(),
		Package:    packagePath,
		Component:  component.identifier,
		Path:       relativePath,
		Line:       position.Line,
		Kind:       typeDeclarationKind(object),
		Exported:   object.Exported(),
		Test:       strings.HasSuffix(relativePath, "_test.go"),
	}
	if err := analyzer.describeTypeContent(modulePath, fileSet, &declaration, object); err != nil {
		return nil, false, err
	}
	return &typeRecord{declaration: declaration, object: object}, true, nil
}

func typeDeclarationKind(object *types.TypeName) TypeKind {
	if object.IsAlias() {
		return typeKindAlias
	}
	switch object.Type().Underlying().(type) {
	case *types.Struct:
		return typeKindStruct
	case *types.Interface:
		return typeKindInterface
	default:
		return typeKindBasic
	}
}

// describeTypeContent fills the fields, methods, embedded types, and
// referenced types of one declaration from its checker object.
func (analyzer *analyzer) describeTypeContent(
	modulePath string,
	fileSet *token.FileSet,
	declaration *TypeDeclaration,
	object *types.TypeName,
) error {
	qualifier := relativeQualifier(object.Pkg())
	switch declaration.Kind {
	case typeKindStruct:
		structType, ok := object.Type().Underlying().(*types.Struct)
		if !ok {
			return nil
		}
		if err := analyzer.describeStruct(
			modulePath,
			fileSet,
			declaration,
			structType,
			qualifier,
		); err != nil {
			return err
		}
		return analyzer.describeMethods(declaration, object, qualifier)
	case typeKindInterface:
		interfaceType, ok := object.Type().Underlying().(*types.Interface)
		if !ok {
			return nil
		}
		return analyzer.describeInterface(modulePath, fileSet, declaration, interfaceType, qualifier)
	case typeKindAlias:
		aliased := types.Unalias(object.Type())
		declaration.Underlying = types.TypeString(aliased, qualifier)
		references, err := analyzer.namedReferences(modulePath, fileSet, declaration.Identifier, aliased)
		if err != nil {
			return err
		}
		declaration.References = references
		return nil
	default:
		declaration.Underlying = types.TypeString(object.Type().Underlying(), qualifier)
		references, err := analyzer.namedReferences(
			modulePath,
			fileSet,
			declaration.Identifier,
			object.Type().Underlying(),
		)
		if err != nil {
			return err
		}
		declaration.References = references
		return analyzer.describeMethods(declaration, object, qualifier)
	}
}

func (analyzer *analyzer) describeStruct(
	modulePath string,
	fileSet *token.FileSet,
	declaration *TypeDeclaration,
	structType *types.Struct,
	qualifier types.Qualifier,
) error {
	var referenced []types.Type
	for field := range structType.Fields() {
		declaration.Fields = append(declaration.Fields, TypeField{
			Name:     field.Name(),
			Type:     types.TypeString(field.Type(), qualifier),
			Embedded: field.Embedded(),
			Exported: field.Exported(),
		})
		if !field.Embedded() {
			referenced = append(referenced, field.Type())
			continue
		}
		embedded, resolved, err := analyzer.typeReference(modulePath, fileSet, namedObject(field.Type()))
		if err != nil {
			return err
		}
		if resolved && embedded.Identifier != declaration.Identifier {
			declaration.Embeds = append(declaration.Embeds, embedded)
		}
	}
	declaration.Embeds = sortedTypeReferences(declaration.Embeds)
	references, err := analyzer.namedReferences(
		modulePath,
		fileSet,
		declaration.Identifier,
		referenced...,
	)
	if err != nil {
		return err
	}
	declaration.References = withoutReferences(references, declaration.Embeds)
	return nil
}

func (analyzer *analyzer) describeInterface(
	modulePath string,
	fileSet *token.FileSet,
	declaration *TypeDeclaration,
	interfaceType *types.Interface,
	qualifier types.Qualifier,
) error {
	for method := range interfaceType.ExplicitMethods() {
		signature, ok := method.Type().(*types.Signature)
		if !ok {
			continue
		}
		declaration.Methods = append(declaration.Methods, TypeMethod{
			Name:      method.Name(),
			Signature: methodSignature(signature, qualifier),
			Exported:  method.Exported(),
		})
	}
	sortTypeMethods(declaration.Methods)
	for embedded := range interfaceType.EmbeddedTypes() {
		reference, resolved, err := analyzer.typeReference(modulePath, fileSet, namedObject(embedded))
		if err != nil {
			return err
		}
		if resolved && reference.Identifier != declaration.Identifier {
			declaration.Embeds = append(declaration.Embeds, reference)
		}
	}
	declaration.Embeds = sortedTypeReferences(declaration.Embeds)
	return nil
}

func (analyzer *analyzer) describeMethods(
	declaration *TypeDeclaration,
	object *types.TypeName,
	qualifier types.Qualifier,
) error {
	named, ok := object.Type().(*types.Named)
	if !ok {
		return nil
	}
	for method := range named.Methods() {
		signature, ok := method.Type().(*types.Signature)
		if !ok {
			continue
		}
		_, pointerReceiver := types.Unalias(signature.Recv().Type()).(*types.Pointer)
		declaration.Methods = append(declaration.Methods, TypeMethod{
			Name:            method.Name(),
			Signature:       methodSignature(signature, qualifier),
			Exported:        method.Exported(),
			PointerReceiver: pointerReceiver,
		})
	}
	sortTypeMethods(declaration.Methods)
	return nil
}

// methodSignature renders one signature without the receiver and the
// `func` keyword, so a diagram row reads `(ctx context.Context) error`.
func methodSignature(signature *types.Signature, qualifier types.Qualifier) string {
	bare := types.NewSignatureType(
		nil,
		nil,
		nil,
		signature.Params(),
		signature.Results(),
		signature.Variadic(),
	)
	return strings.TrimPrefix(types.TypeString(bare, qualifier), "func")
}

func sortTypeMethods(methods []TypeMethod) {
	slices.SortFunc(methods, func(first, second TypeMethod) int {
		return strings.Compare(first.Name, second.Name)
	})
}

// namedReferences collects the module-local named types the candidates
// mention, excluding the declaring type itself.
func (analyzer *analyzer) namedReferences(
	modulePath string,
	fileSet *token.FileSet,
	selfIdentifier string,
	candidates ...types.Type,
) ([]TypeReference, error) {
	visited := make(map[types.Type]bool)
	var objects []*types.TypeName
	for _, candidate := range candidates {
		collectNamedObjects(candidate, visited, &objects)
	}
	var references []TypeReference
	for _, object := range objects {
		reference, resolved, err := analyzer.typeReference(modulePath, fileSet, object)
		if err != nil {
			return nil, err
		}
		if resolved && reference.Identifier != selfIdentifier {
			references = append(references, reference)
		}
	}
	return sortedTypeReferences(references), nil
}

// collectNamedObjects walks one checker type and gathers every named or
// aliased type it mentions, without following the content of those types.
func collectNamedObjects(candidate types.Type, visited map[types.Type]bool, objects *[]*types.TypeName) {
	if candidate == nil || visited[candidate] {
		return
	}
	visited[candidate] = true
	switch value := candidate.(type) {
	case *types.Alias:
		*objects = append(*objects, value.Obj())
		collectTypeArguments(value.TypeArgs(), visited, objects)
	case *types.Named:
		*objects = append(*objects, value.Obj())
		collectTypeArguments(value.TypeArgs(), visited, objects)
	case *types.Pointer:
		collectNamedObjects(value.Elem(), visited, objects)
	case *types.Slice:
		collectNamedObjects(value.Elem(), visited, objects)
	case *types.Array:
		collectNamedObjects(value.Elem(), visited, objects)
	case *types.Chan:
		collectNamedObjects(value.Elem(), visited, objects)
	case *types.Map:
		collectNamedObjects(value.Key(), visited, objects)
		collectNamedObjects(value.Elem(), visited, objects)
	case *types.Signature:
		collectTupleObjects(value.Params(), visited, objects)
		collectTupleObjects(value.Results(), visited, objects)
	case *types.Struct:
		for field := range value.Fields() {
			collectNamedObjects(field.Type(), visited, objects)
		}
	case *types.Interface:
		for embedded := range value.EmbeddedTypes() {
			collectNamedObjects(embedded, visited, objects)
		}
	}
}

func collectTypeArguments(
	arguments *types.TypeList,
	visited map[types.Type]bool,
	objects *[]*types.TypeName,
) {
	if arguments == nil {
		return
	}
	for index := range arguments.Len() {
		collectNamedObjects(arguments.At(index), visited, objects)
	}
}

func collectTupleObjects(tuple *types.Tuple, visited map[types.Type]bool, objects *[]*types.TypeName) {
	if tuple == nil {
		return
	}
	for variable := range tuple.Variables() {
		collectNamedObjects(variable.Type(), visited, objects)
	}
}

// typeReference classifies one named type object into a graph reference.
// It resolves only module-local objects inside the analysis classification.
func (analyzer *analyzer) typeReference(
	modulePath string,
	fileSet *token.FileSet,
	object *types.TypeName,
) (TypeReference, bool, error) {
	if object == nil || object.Pkg() == nil {
		return TypeReference{}, false, nil
	}
	packagePath, local := localPackagePath(modulePath, object.Pkg().Path())
	if !local {
		return TypeReference{}, false, nil
	}
	position := physicalSourcePosition(fileSet, object.Pos())
	relativePath, insideRepository := analyzer.relativeSourcePath(position.Filename)
	classificationPath := packagePath
	if insideRepository {
		classificationPath = relativePath
	}
	ignored, err := analyzer.ignoredPaths.matches(classificationPath)
	if err != nil {
		return TypeReference{}, false, err
	}
	if ignored {
		return TypeReference{}, false, nil
	}
	component, classified := analyzer.classifier.classify(classificationPath)
	if !classified {
		return TypeReference{}, false, nil
	}
	return TypeReference{
		Identifier: packagePath + "." + object.Name(),
		Component:  component.identifier,
	}, true, nil
}

// namedObject resolves the type name behind one embedded field or embedded
// interface entry, unwrapping aliases and one pointer level.
func namedObject(candidate types.Type) *types.TypeName {
	unaliased := candidate
	if alias, ok := candidate.(*types.Alias); ok {
		return alias.Obj()
	}
	if pointer, ok := types.Unalias(unaliased).(*types.Pointer); ok {
		unaliased = types.Unalias(pointer.Elem())
	}
	if alias, ok := unaliased.(*types.Alias); ok {
		return alias.Obj()
	}
	if named, ok := types.Unalias(unaliased).(*types.Named); ok {
		return named.Obj()
	}
	return nil
}

func sortedTypeReferences(references []TypeReference) []TypeReference {
	if len(references) == 0 {
		return nil
	}
	slices.SortFunc(references, func(first, second TypeReference) int {
		return strings.Compare(first.Identifier, second.Identifier)
	})
	return slices.CompactFunc(references, func(first, second TypeReference) bool {
		return first.Identifier == second.Identifier
	})
}

func withoutReferences(references, excluded []TypeReference) []TypeReference {
	if len(references) == 0 || len(excluded) == 0 {
		return references
	}
	result := slices.DeleteFunc(references, func(reference TypeReference) bool {
		return slices.ContainsFunc(excluded, func(candidate TypeReference) bool {
			return candidate.Identifier == reference.Identifier
		})
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

// annotateImplementations records which declared types satisfy which
// declared interfaces. Pointer receiver method sets are included, so a
// type whose pointer satisfies the interface is also reported.
func annotateImplementations(records []*typeRecord) {
	var interfaces []*typeRecord
	for _, record := range records {
		if record.declaration.Kind != typeKindInterface {
			continue
		}
		interfaceType, ok := record.object.Type().Underlying().(*types.Interface)
		if !ok || interfaceType.NumMethods() == 0 {
			continue
		}
		interfaces = append(interfaces, record)
	}
	for _, record := range records {
		if record.declaration.Kind == typeKindAlias {
			continue
		}
		candidate := record.object.Type()
		for _, abstraction := range interfaces {
			if abstraction.declaration.Identifier == record.declaration.Identifier {
				continue
			}
			interfaceType, ok := abstraction.object.Type().Underlying().(*types.Interface)
			if !ok {
				continue
			}
			if !implementsInterface(candidate, abstraction.object.Type(), interfaceType) {
				continue
			}
			record.declaration.Implements = append(record.declaration.Implements, TypeReference{
				Identifier: abstraction.declaration.Identifier,
				Component:  abstraction.declaration.Component,
			})
		}
		record.declaration.Implements = sortedTypeReferences(record.declaration.Implements)
	}
}

// implementsInterface reports whether the candidate satisfies the interface,
// through its value or pointer method set. A generic interface cannot feed
// `types.Implements` uninstantiated, so its type arguments are inferred from
// the candidate first.
func implementsInterface(
	candidate types.Type,
	abstraction types.Type,
	interfaceType *types.Interface,
) bool {
	if named, ok := abstraction.(*types.Named); ok && named.TypeParams().Len() > 0 {
		return implementsGenericInterface(candidate, named)
	}
	return types.Implements(candidate, interfaceType) ||
		types.Implements(types.NewPointer(candidate), interfaceType)
}

// implementsGenericInterface reports whether the candidate satisfies the
// generic interface under some instantiation. Each interface method signature
// is unified against the candidate's method of the same name to infer the
// type arguments; `types.Implements` then verifies the instantiated
// interface, so the inference proposes but never over-reports.
func implementsGenericInterface(candidate types.Type, generic *types.Named) bool {
	interfaceType, ok := generic.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	bindings := make([]types.Type, generic.TypeParams().Len())
	for index := range interfaceType.NumMethods() {
		method := interfaceType.Method(index)
		found, _, _ := types.LookupFieldOrMethod(
			types.NewPointer(candidate),
			true,
			method.Pkg(),
			method.Name(),
		)
		implementation, ok := found.(*types.Func)
		if !ok {
			return false
		}
		pattern, patternOk := method.Type().(*types.Signature)
		actual, actualOk := implementation.Type().(*types.Signature)
		if !patternOk || !actualOk || !unifySignatures(pattern, actual, bindings) {
			return false
		}
	}
	if slices.Contains(bindings, nil) {
		return false
	}
	instance, err := types.Instantiate(nil, generic, bindings, true)
	if err != nil {
		return false
	}
	instantiated, ok := instance.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	return types.Implements(candidate, instantiated) ||
		types.Implements(types.NewPointer(candidate), instantiated)
}

func unifySignatures(pattern, actual *types.Signature, bindings []types.Type) bool {
	return pattern.Variadic() == actual.Variadic() &&
		unifyTuples(pattern.Params(), actual.Params(), bindings) &&
		unifyTuples(pattern.Results(), actual.Results(), bindings)
}

func unifyTuples(pattern, actual *types.Tuple, bindings []types.Type) bool {
	if pattern.Len() != actual.Len() {
		return false
	}
	for index := range pattern.Len() {
		if !unifyTypes(pattern.At(index).Type(), actual.At(index).Type(), bindings) {
			return false
		}
	}
	return true
}

// unifyTypes walks one interface type and one candidate type in parallel and
// records the candidate type that stands where the pattern names a type
// parameter. A parameter bound twice must bind to identical types.
func unifyTypes(pattern, actual types.Type, bindings []types.Type) bool {
	switch typed := pattern.(type) {
	case *types.TypeParam:
		index := typed.Index()
		if index < 0 || index >= len(bindings) {
			return false
		}
		if bindings[index] == nil {
			bindings[index] = actual
			return true
		}
		return types.Identical(bindings[index], actual)
	case *types.Pointer:
		other, ok := actual.(*types.Pointer)
		return ok && unifyTypes(typed.Elem(), other.Elem(), bindings)
	case *types.Slice:
		other, ok := actual.(*types.Slice)
		return ok && unifyTypes(typed.Elem(), other.Elem(), bindings)
	case *types.Array:
		other, ok := actual.(*types.Array)
		return ok && typed.Len() == other.Len() && unifyTypes(typed.Elem(), other.Elem(), bindings)
	case *types.Chan:
		other, ok := actual.(*types.Chan)
		return ok && typed.Dir() == other.Dir() && unifyTypes(typed.Elem(), other.Elem(), bindings)
	case *types.Map:
		other, ok := actual.(*types.Map)
		return ok &&
			unifyTypes(typed.Key(), other.Key(), bindings) &&
			unifyTypes(typed.Elem(), other.Elem(), bindings)
	case *types.Signature:
		other, ok := actual.(*types.Signature)
		return ok && unifySignatures(typed, other, bindings)
	case *types.Named:
		other, ok := actual.(*types.Named)
		if !ok || typed.Origin() != other.Origin() ||
			typed.TypeArgs().Len() != other.TypeArgs().Len() {
			return types.Identical(pattern, actual)
		}
		for index := range typed.TypeArgs().Len() {
			if !unifyTypes(typed.TypeArgs().At(index), other.TypeArgs().At(index), bindings) {
				return false
			}
		}
		return true
	default:
		return types.Identical(pattern, actual)
	}
}

// relativeQualifier renders type names without a package prefix for the
// declaring package, and with the short package name for every other one.
func relativeQualifier(declaringPackage *types.Package) types.Qualifier {
	return func(other *types.Package) string {
		if other == declaringPackage {
			return ""
		}
		return other.Name()
	}
}
