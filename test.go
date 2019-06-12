package main

import (
	"fmt"
)

func main() {

	a := []string{"Foo", "Bar", "jj"}
	for i, s := range a {
		fmt.Println(i, s)
	}
}
