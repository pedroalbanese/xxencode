package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/pedroalbanese/xxencode"
)

var (
	dec   = flag.Bool("d", false, "Decode instead of Encode")
	ifile = flag.String("f", "", "Target file")
)

func main() {
	flag.Parse()

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage of", os.Args[0]+":")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if !*dec {
		// Codificação
		infile, err := os.Open(*ifile)
		if err != nil {
			log.Fatal(err)
		}
		defer infile.Close()

		info, err := infile.Stat()
		if err != nil {
			log.Fatal(err)
		}

		xxw := xxencode.NewWriter(os.Stdout, *ifile, info.Mode())
		if _, err := io.Copy(xxw, infile); err != nil {
			log.Fatal(err)
		}
		if err := xxw.Flush(); err != nil {
			log.Fatal(err)
		}
	} else {
		// Decodificação
		var infile *os.File
		var err error
		
		if *ifile == "-" || *ifile == "" {
			infile = os.Stdin
		} else {
			infile, err = os.Open(*ifile)
			if err != nil {
				log.Fatal(err)
			}
			defer infile.Close()
		}

		xxr := xxencode.NewReader(infile, nil)

		filename, err := xxr.File()
		if err != nil {
			log.Fatal(err)
		}

		outfile, err := os.Create(filename)
		if err != nil {
			log.Fatal(err)
		}
		defer outfile.Close()

		if _, err := io.Copy(outfile, xxr); err != nil {
			log.Fatal(err)
		}

		mode, err := xxr.Mode()
		if err != nil {
			log.Fatal(err)
		}

		if err := outfile.Chmod(mode); err != nil {
			log.Println("Warning: could not set file mode:", err)
		}

		fmt.Fprintf(os.Stderr, "file: %s, mode: %03o\n", filename, mode)
	}
}
