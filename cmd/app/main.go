package main

import (
	"flag"
	"fmt"
	"log"

	"cyberwriter/ui"
)

const version = "v0.4.0"

func main() {
	vault := flag.String("vault", "", "open a Vault directory on startup")
	showVersion := flag.Bool("version", false, "print the application version")
	flag.Parse()
	if *showVersion {
		fmt.Println("object-N/A", version)
		return
	}
	if err := ui.Run(*vault); err != nil {
		log.Fatal(err)
	}
}
