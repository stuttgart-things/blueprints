package main

import "fmt"

var (
	Version   string
	Commit    string
	BuildTime string
)

func main() {
	fmt.Println("Version:", Version)
	fmt.Println("Commit:", Commit)
	fmt.Println("BuildTime:", BuildTime)
}
