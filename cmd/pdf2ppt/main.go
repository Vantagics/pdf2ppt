// Command pdf2ppt converts a PDF into an editable PPTX.
//
//	pdf2ppt input.pdf output.pptx
package main

import (
	"fmt"
	"os"

	"github.com/Vantagics/pdf2ppt/converter"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println(converter.Version)
		return
	}
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "pdf2ppt %s\nusage: pdf2ppt input.pdf output.pptx\n", converter.Version)
		os.Exit(2)
	}
	c := converter.NewConverter(func(cur, total int) {
		fmt.Fprintf(os.Stderr, "page %d/%d\n", cur, total)
	})
	res, err := c.Convert(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d slides)\n", res.OutputPath, res.PageCount)
}
