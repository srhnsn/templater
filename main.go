package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flosch/pongo2"
)

var (
	envPrefix string
)

func main() {
	pongo2.SetAutoescape(false)
	flag.Usage = printUsage

	flag.StringVar(&envPrefix, "envprefix", "deployvar_", "prefix of environment variable names to parse")
	flag.Parse()

	args := len(os.Args) - 1

	if args == 0 {
		runFromStdIn()
	} else if args == 2 {
		runFromDirectory(os.Args[1], os.Args[2])
	} else {
		fmt.Fprintf(os.Stderr, "error: 0 or 2 arguments required, got %d\n", args)
		printUsage()
		os.Exit(1)
	}
}

func getContext() pongo2.Context {
	context := pongo2.Context{}

	for _, line := range os.Environ() {
		key, value := parseEnvironLine(line)
		key = strings.ToLower(key)

		if !strings.HasPrefix(key, envPrefix) {
			continue
		}

		key = key[len(envPrefix):]

		var printValue string

		if strings.Contains(key, "pass") || strings.Contains(key, "pwd") {
			printValue = "***"
		} else {
			printValue = value
		}
		fmt.Fprintf(os.Stderr, "var: %s=%v\n", key, printValue)

		context[key] = value
	}

	return context
}

func getRenderedBytes(context pongo2.Context, inputData []byte) []byte {
	tpl, err := pongo2.FromBytes(inputData)
	panicIfErr(err)

	outputData, err := tpl.ExecuteBytes(context)
	panicIfErr(err)

	return outputData
}

func parseEnvironLine(line string) (string, string) {
	splits := strings.SplitN(line, "=", 2)

	if len(splits) != 2 {
		panic("invalid environment variable line: " + line)
	}

	return splits[0], splits[1]
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage of %s: [OPTION]... INPUT_DIR OUTPUT_DIR\n", os.Args[0])
	flag.PrintDefaults()
}

func runFromDirectory(inputDir, outputDir string) {
	context := getContext()

	type dirTime struct {
		targetPath string
		mTime      time.Time
	}

	var dirTimes []dirTime

	err := filepath.Walk(inputDir, func(inputPath string, inputFileInfo os.FileInfo, err error) error {
		panicIfErr(err)

		relPath, err := filepath.Rel(inputDir, inputPath)
		panicIfErr(err)

		targetPath := filepath.Join(outputDir, relPath)

		if inputFileInfo.IsDir() {
			err = os.MkdirAll(targetPath, inputFileInfo.Mode())
			panicIfErr(err)

			dirTimes = append(dirTimes, dirTime{targetPath, inputFileInfo.ModTime()})
			return nil
		}

		inputData, err := ioutil.ReadFile(inputPath)
		panicIfErr(err)

		var outputData []byte

		if strings.HasSuffix(inputFileInfo.Name(), ".raw") {
			outputData = inputData
			targetPath = targetPath[:len(targetPath)-4]
		} else {
			outputData = getRenderedBytes(context, inputData)
		}

		err = ioutil.WriteFile(targetPath, outputData, inputFileInfo.Mode())
		panicIfErr(err)

		err = os.Chtimes(targetPath, inputFileInfo.ModTime(), inputFileInfo.ModTime())
		panicIfErr(err)

		return nil
	})

	panicIfErr(err)

	for _, dirTime := range dirTimes {
		err = os.Chtimes(dirTime.targetPath, dirTime.mTime, dirTime.mTime)
		panicIfErr(err)
	}
}

func runFromStdIn() {
	input, err := ioutil.ReadAll(os.Stdin)
	panicIfErr(err)

	tpl, err := pongo2.FromBytes(input)
	panicIfErr(err)

	context := getContext()

	err = tpl.ExecuteWriter(context, os.Stdout)
	panicIfErr(err)
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}
