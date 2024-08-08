package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/ansel1/merry"
	"storj.io/common/storj"
)

var convMap = map[string]struct {
	fromString FromStringFunc
	toString   ToStringFunc
}{
	"hex": {
		fromString: func(str string) (storj.NodeID, error) {
			buf, err := hex.DecodeString(str)
			if err != nil {
				return storj.NodeID{}, merry.Wrap(err)
			}
			id, err := storj.NodeIDFromBytes(buf)
			return id, merry.Wrap(err)
		},
		toString: func(id storj.NodeID) (string, error) {
			return hex.EncodeToString(id[:]), nil
		},
	},
	"base58": {
		fromString: func(str string) (storj.NodeID, error) {
			id, err := storj.NodeIDFromString(str)
			return id, merry.Wrap(err)
		},
		toString: func(id storj.NodeID) (string, error) {
			return id.String(), nil
		},
	},
	"difficulty": {
		fromString: func(str string) (storj.NodeID, error) {
			return storj.NodeID{}, merry.New("can not make node from its difficulty value")
		},
		toString: func(id storj.NodeID) (string, error) {
			dif, err := id.Difficulty()
			if err != nil {
				return "", merry.Wrap(err)
			}
			return strconv.FormatUint(uint64(dif), 10), nil
		},
	},
}

type Format struct {
	Val string
}

func (e *Format) Set(name string) error {
	if _, ok := convMap[name]; !ok {
		return merry.New("wrong format: " + name)
	}
	e.Val = name
	return nil
}

func (e Format) String() string {
	return e.Val
}

func (e Format) Type() string {
	return "string"
}

type FromStringFunc func(string) (storj.NodeID, error)
type ToStringFunc func(storj.NodeID) (string, error)

func forEachLine(fromString FromStringFunc, toString ToStringFunc) error {
	// file, err := os.Open()
	// if err != nil {
	// 	return merry.Wrap(err)
	// }
	// defer file.Close()
	file := os.Stdin

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}
		line = strings.TrimPrefix(line, "\\x") //cut off Postgresql HEX prefix (if any)
		id, err := fromString(line)
		if err != nil {
			return merry.Wrap(err)
		}
		str, err := toString(id)
		if err != nil {
			return merry.Wrap(err)
		}
		os.Stdout.WriteString(str + "\n")
	}

	if err := scanner.Err(); err != nil {
		return merry.Wrap(err)
	}
	return nil
}

func process(modeFrom, modeTo Format) error {
	return merry.Wrap(forEachLine(
		convMap[modeFrom.Val].fromString,
		convMap[modeTo.Val].toString,
	))
}

func main() {
	var modesArr []string
	for name := range convMap {
		modesArr = append(modesArr, name)
	}
	modes := strings.Join(modesArr, ", ")

	modeFrom := Format{Val: "hex"}
	modeTo := Format{Val: "base58"}
	flag.Var(&modeFrom, "from", "source format: "+modes)
	flag.Var(&modeTo, "to", "destination format")
	flag.Parse()

	if err := process(modeFrom, modeTo); err != nil {
		panic(merry.Details(err))
	}
}
