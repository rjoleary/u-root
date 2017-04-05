package bbshare

import (
	"errors"
	"fmt"
)

type initList map[string][]func()

var (
	varInits initList = initList{}
	inits    initList = initList{}
	mains    initList = initList{}
)

func (l *initList) add(pkgName string, f func()) {
	(*l)[pkgName] = append((*l)[pkgName], f)
}

func (l *initList) run(pkgName string) {
	if v, ok := (*l)[pkgName]; ok {
		for _, f := range v {
			f()
		}
	}
}

// Run executes the init and main for pkgName.
func Run(pkgName string) error {
	_, ok := mains[pkgName]
	if !ok {
		return errors.New("package not found")
	}
	varInits.run(pkgName)
	inits.run(pkgName)
	mains.run(pkgName)
	return nil
}

// PkgNames returns a list of packges with registered main functions.
func PkgNames() []string {
	names := []string{}
	for k, _ := range mains {
		names = append(names, k)
	}
	return names
}

// AddVarInit adds a variable initializer for pkgName.
func AddVarInit(pkgName string, f func()) {
	varInits.add(pkgName, f)
}

// AddInit adds an init function for pkgName.
func AddInit(pkgName string, f func()) {
	inits.add(pkgName, f)
}

// AddMain adds a main function for pkgName.
// Note that a package may only have one main function.
func AddMain(pkgName string, f func()) {
	if _, ok := mains[pkgName]; ok {
		panic(fmt.Sprintf("package %q has multiple main functions", pkgName))
	}
	mains.add(pkgName, f)
}
