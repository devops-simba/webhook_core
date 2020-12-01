//+build: ignore

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const outputHeader = `// AUTO GENERATED TEMPLATE FILE
`

type importInfo struct {
	Alias string
	Path  string
}

func check(err error) {
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		panic(err)
	}
}
func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
func writeOutput(f io.Writer, value string) {
	_, err := f.Write([]byte(value))
	check(err)
}
func writeOutputf(f io.Writer, format string, args ...interface{}) {
	writeOutput(f, fmt.Sprintf(format, args...))
}
func parseOption(s string) (optionName string, optionValue interface{}, err error) {
	i := strings.IndexByte(s, ' ')
	if i == -1 {
		optionName = s
		return
	}
	if i == 0 {
		err = errors.New("Missing option name")
		return
	}

	optionName = s[:i]
	for i < len(s) && s[i] == ' ' {
		i++
	}

	err = json.Unmarshal([]byte(s[i:]), &optionValue)
	return
}
func main() {
	var packageName, outputFile, parserNamespace, parserFunction string
	flag.StringVar(&packageName, "package", "", "package of the generated file")
	flag.StringVar(&outputFile, "output", "templates.autogen.go", "file that should receive the output")
	flag.StringVar(&parserNamespace, "parser-ns", "", "Namespace of the function that will be used to parse the templates")
	flag.StringVar(&parserFunction, "parser-fn", "", "Function that will be used to parse templates")
	flag.Parse()

	outputFile, err := filepath.Abs(outputFile)
	check(err)

	output, err := os.Create(outputFile)
	check(err)
	defer output.Close()

	writeOutput(output, outputHeader)
	writeOutputf(output, "package %s\n\n", packageName)
	writeOutput(output, "import (\n")
	writeOutput(output, "    \"io\"\n")
	writeOutput(output, "    \"os\"\n")
	writeOutput(output, "    \"strings\"\n")
	writeOutput(output, "    \"text/template\"\n")

	if parserNamespace != "" {
		writeOutputf(output, "    pns \"%s\"\n", parserNamespace)
		parserFunction = "pns." + parserFunction
		writeOutput(output, ")\n\n")
	} else if parserFunction != "" {
		writeOutput(output, ")\n\n")
	} else {
		writeOutput(output, ")\n\n")
		writeOutput(output, "func parseTemplate(name, body string) (*template.Template, error) { return template.New(name).Parse(body) }\n")
		writeOutput(output, "\n")
		parserFunction = "parseTemplate"
	}

	writeOutput(output, "\n")
	check(filepath.Walk("templates", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".gotmpl" {
			log.Printf("Ignoring non-template file: %s", path)
			return nil
		}

		log.Printf("Trying to parse template %s", path)
		in, err := os.Open(path)
		if err != nil {
			return err
		}

		var ok bool
		var opvalue interface{}
		var name, dataType, opname string
		content := make([]string, 0)
		scanner := bufio.NewScanner(in)
		for scanner.Scan() {
			line := scanner.Text()
			err = errors.New("TEMP")
			if strings.HasPrefix(line, "#+gotmpl:") {
				opname, opvalue, err = parseOption(line[9:])
			} else if strings.HasPrefix(line, "//+gotmpl:") {
				opname, opvalue, err = parseOption(line[10:])
			} else {
				content = append(content, line)
				continue
			}

			if err != nil {
				return err
			}

			switch opname {
			case "Name":
				if name, ok = opvalue.(string); !ok {
					return fmt.Errorf("Name option must be string, but received: %v", opvalue)
				}
			case "DataType":
				if dataType, ok = opvalue.(string); !ok {
					return fmt.Errorf("DataType option must be string, but received: %v", opvalue)
				}
			default:
				return fmt.Errorf("Unknown option: %s", opname)
			}
		}

		if name == "" {
			return errors.New("Missing template name")
		}
		if dataType == "" {
			dataType = "interface{}"
		}

		writeOutputf(output, "//region %s template\n", name)
		writeOutputf(output, "var %sTemplate = template.Must(%s(%s, strings.Join([]string{\n", name, parserFunction, quote(name))
		for i := 0; i < len(content); i++ {
			writeOutputf(output, "    %s,\n", quote(content[i]))
		}
		writeOutput(output, "}, \"\\n\")))\n")
		writeOutput(output, "\n")
		writeOutputf(output, "func Write%s(w io.Writer, data %s) error {\n", name, dataType)
		writeOutputf(output, "	return %sTemplate.Execute(w, data)\n", name)
		writeOutput(output, "}\n")
		writeOutputf(output, "func Write%sToFile(path string, data %s) error {\n", name, dataType)
		writeOutput(output, "	f, err := os.Create(path)\n")
		writeOutput(output, "	if err != nil {\n")
		writeOutput(output, "		return err\n")
		writeOutput(output, "	}\n")
		writeOutput(output, "	defer f.Close()\n")
		writeOutput(output, "\n")
		writeOutputf(output, "	return Write%s(f, data)\n", name)
		writeOutput(output, "}\n")
		writeOutputf(output, "func Render%s(data %s) (string, error) {\n", name, dataType)
		writeOutput(output, "	builder := &strings.Builder{}\n")
		writeOutputf(output, "	err := Write%s(builder, data)\n", name)
		writeOutput(output, "	if err != nil {\n")
		writeOutput(output, "		return \"\", err\n")
		writeOutput(output, "	}\n")
		writeOutput(output, "	return builder.String(), nil\n")
		writeOutput(output, "}\n")
		writeOutput(output, "\n")
		writeOutput(output, "//endregion\n\n")

		return nil
	}))
}
