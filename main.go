package main

import (
	"flag"
	"fmt"

	"github.com/lonegunmanb/yorbox/pkg"
)

func main() {
	// Define command line flags
	var dirPath string
	flag.StringVar(&dirPath, "dir", "", "Path to the directory containing .tf files")

	var toggleName string
	flag.StringVar(&toggleName, "toggleName", "yor_toggle", "Name of the toggle to add")

	var help bool
	flag.BoolVar(&help, "help", false, "Print help information")

	flag.Parse()

	if help {
		// Print help information
		fmt.Println("Usage: myprogram -dir <directory path> [-toggleName <toggle name>]")
		flag.PrintDefaults()
		return
	}

	if dirPath == "" {
		fmt.Println("Directory path is required. Use -help for more information.")
		return
	}

	err := pkg.ProcessDirectory(pkg.Options{
		Path:       dirPath,
		ToggleName: toggleName,
	})
	if err != nil {
		fmt.Println("Error processing directory:", err)
		return
	}

	fmt.Println("Directory processed successfully.")
}
