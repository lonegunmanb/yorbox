package main

import (
	"flag"
	"fmt"
	"github.com/lonegunmanb/yorbox/pkg"
	"os"
)

func main() {
	// Define command line flags
	var dirPath string
	flag.StringVar(&dirPath, "dir", "", "path to the directory containing .tf files")

	var toggleName string
	flag.StringVar(&toggleName, "toggleName", "tracing_tags_enabled", "Name of the toggle to add")

	var boxTemplate string
	flag.StringVar(&boxTemplate, "boxTemplate", "/*<box>*/ (var.{{ .toggleName }} ? /*</box>*/ { yor_trace = 123 } /*<box>*/ : {} ) /*</box>*/",
		"Box template to use when adding boxes")

	var tagsPrefix string
	flag.StringVar(&tagsPrefix, "tagsPrefix", "", "Prefix for tags applied to resources")

	var help bool
	flag.BoolVar(&help, "help", false, "Print help information")

	flag.Parse()

	if help {
		// Print help information
		fmt.Println("Usage: myprogram -dir <directory path> [-toggleName <toggle name>] [-boxTemplate <box template>] [-tagsPrefix <tags prefix>]")
		flag.PrintDefaults()
		return
	}

	if dirPath == "" {
		fmt.Println("Directory path is required. Use -help for more information.")
		return
	}

	options := pkg.Options{
		Path:        dirPath,
		ToggleName:  toggleName,
		BoxTemplate: boxTemplate,
		TagsPrefix:  tagsPrefix,
	}
	valid := optionValid(options)
	if !valid {
		os.Exit(1)
	}

	err := pkg.ProcessDirectory(options)

	if err != nil {
		fmt.Println("Error processing directory:", err)
		return
	}

	fmt.Println("Directory processed successfully.")
}

func optionValid(options pkg.Options) bool {
	tplt, err := options.RenderBoxTemplate()
	if err != nil {
		fmt.Println("Error rendering box template:", err)
		return false
	}
	_, diag := pkg.BuildBoxFromTemplate(tplt)
	if diag.HasErrors() {
		fmt.Println("Error building box from template:", diag.Error())
		return false
	}
	return true
}
