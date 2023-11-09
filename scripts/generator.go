package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/spf13/pflag"
	"os"
	"strings"
	"text/template"
	"time"
)

/*

scripts/generator.go

This script generates basic CRUD functionality for a new Kind.

It's rude and crude, but it generates working code.

TODO: all of it can be better

*/

var (
	kind string = "Asteroid"
)

func init() {
	_ = flag.Set("logtostderr", "true")
	flags := pflag.CommandLine
	flags.AddGoFlagSet(flag.CommandLine)

	flags.StringVar(&kind, "kind", kind, "the name of the kind.  e.g Account or User")
}

func main() {
	// Parse flags
	pflag.Parse()

	templates := []string{
		"api",
		"dao",
		"services",
		"mock",
		"migration",
		"test",
	}

	for _, nm := range templates {
		path := fmt.Sprintf("templates/generate-%s.txt", nm)
		contents, err := os.ReadFile(path)
		if err != nil {
			panic(err)
			return
		}

		kindTmpl, err := template.New(nm).Parse(string(contents))
		if err != nil {
			panic(err)
		}

		k := myWriter{
			Kind:              kind,
			KindLowerPlural:   strings.ToLower(fmt.Sprintf("%ss", kind)),
			KindLowerSingular: strings.ToLower(kind),
		}

		now := time.Now()
		k.ID = fmt.Sprintf("%d%s%s%s%s", now.Year(), datePad(int(now.Month())), datePad(now.Day()), datePad(now.Hour()), datePad(now.Minute()))

		var outPath string

		if strings.Contains(nm, "mock") {
			outPath = fmt.Sprintf("pkg/dao/mocks/%s.go", k.KindLowerSingular)
		} else if strings.Contains(nm, "migration") {
			outPath = fmt.Sprintf("pkg/db/migrations/%s_add_%s.go", k.ID, k.KindLowerPlural)
		} else if strings.Contains(nm, "test") {
			outPath = fmt.Sprintf("test/integration/%s_test.go", k.KindLowerPlural)
		} else {
			outPath = fmt.Sprintf("pkg/%s/%s.go", nm, k.KindLowerSingular)
		}

		f, err := os.Create(outPath)
		defer f.Close()

		w := bufio.NewWriter(f)
		err = kindTmpl.Execute(w, k)
		if err != nil {
			panic(err)
		}
		w.Flush()
		f.Sync()
	}
}

func datePad(d int) string {
	if d < 10 {
		return fmt.Sprintf("0%d", d)
	}
	return fmt.Sprintf("%d", d)
}

type myWriter struct {
	Kind string
	//KindLower         string
	KindLowerPlural   string
	KindLowerSingular string
	ID                string
}
