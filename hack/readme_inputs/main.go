package main

import (
	"bufio"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	startLine = "The configurable portions of the Provisioning CRD are:"
	endLine   = "## What are its outputs?"
)

// generatedDocs returns the README lines generated from the ProvisioningSpec fields.
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
				comment := strings.TrimSpace(field.Doc.Text())
				commentLines := strings.Split(comment, "\n")
				filteredLines := commentLines[:0]
				for _, line := range commentLines {
					if strings.TrimSpace(line) != "+optional" {
						filteredLines = append(filteredLines, line)
					}
				}
				docs = append(docs, "- "+strings.Join(filteredLines, "\n"), "")
			}
		}
	}
	return docs
}

// readmeContent reads a README and replaces its configurable portions with API field docs.
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

// writeReadme atomically replaces a README while preserving its permissions.
func writeReadme(readmePath string, output []string) (err error) {
	info, err := os.Stat(readmePath)
	if err != nil {
		return fmt.Errorf("stat README %q: %w", readmePath, err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(readmePath), "."+filepath.Base(readmePath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary README: %w", err)
	}
	tempPath := tempFile.Name()
	renamed := false
	defer func() {
		if !renamed {
			if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove temporary README %q: %w", tempPath, removeErr))
			}
		}
	}()

	var writeErr error
	for _, line := range output {
		data := []byte(line + "\n")
		n, err := tempFile.Write(data)
		if err != nil {
			writeErr = fmt.Errorf("write temporary README: %w", err)
			break
		}
		if n != len(data) {
			writeErr = fmt.Errorf("write temporary README: %w", io.ErrShortWrite)
			break
		}
	}
	if writeErr == nil {
		if chmodErr := tempFile.Chmod(info.Mode().Perm()); chmodErr != nil {
			writeErr = fmt.Errorf("set temporary README permissions: %w", chmodErr)
		}
	}
	closeErr := tempFile.Close()
	if writeErr != nil || closeErr != nil {
		if closeErr != nil {
			closeErr = fmt.Errorf("close temporary README: %w", closeErr)
		}
		return errors.Join(writeErr, closeErr)
	}

	if err := os.Rename(tempPath, readmePath); err != nil {
		return fmt.Errorf("replace README %q: %w", readmePath, err)
	}
	renamed = true
	return nil
}

// main regenerates the README's configurable Provisioning API section.
func main() {
	readmePath := os.Args[1]
	apiPath := os.Args[2]
	output, err := readmeContent(readmePath, apiPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := writeReadme(readmePath, output); err != nil {
		log.Fatal(err)
	}
}
