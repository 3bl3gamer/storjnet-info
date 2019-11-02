package utils

import "github.com/ansel1/merry"

type Env struct {
	Val string
}

func (e *Env) Set(name string) error {
	if name != "dev" && name != "prod" {
		return merry.New("wrong env: " + name)
	}
	e.Val = name
	return nil
}

func (e Env) String() string {
	return e.Val
}

func (e Env) IsDev() bool {
	return e.Val == "dev"
}

func (e Env) IsProd() bool {
	return e.Val == "prod"
}
