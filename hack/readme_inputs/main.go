package main

import (
	"bufio"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"log"
	"os"
)

const (
	startLine = "The configurable portions of the Provisioning CRD are:"
	endLine   = "## What are its outputs?"
)

func generatedDocs(apiPath string) []string {
	docs := []string{""}
	d, err := parser.ParseDir(token.NewFileSet(), apiPath, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
		return docs
	}

	for _, f := range d {
		p := doc.New(f, "./", 0)
		for _, t := range p.Types {
			if t.Name != "ProvisioningSpec" {
				continue
			}
			structType := t.Decl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
			for _, field := range structType.Fields.List {
				docs = append(docs, "- "+field.Doc.Text())
			}
		}
	}
	return append(docs, "")
}

func readmeContent(readmePath, apiPath string) ([]string, error) {
	file, err := os.Open(readmePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inBlock := false
	output := []string{}
	for scanner.Scan() {
		if scanner.Text() == endLine {
			inBlock = false
		}
		if !inBlock {
			output = append(output, scanner.Text())
		}
		if scanner.Text() == startLine {
			inBlock = true
			output = append(output, generatedDocs(apiPath)...)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return output, nil
}

func main() {
	readmePath := os.Args[1]
	apiPath := os.Args[2]
	output, err := readmeContent(readmePath, apiPath)
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.OpenFile(readmePath, os.O_RDWR, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	for _, line := range output {
		_, err = file.WriteString(line + "\n")
		if err != nil {
			log.Print(err)
		}
	}
}
