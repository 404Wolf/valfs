package main

import (
	"fmt"
	"github.com/404wolf/valfs/sdk"
)

func main() {
	client := sdk.NewClient()
	res, _ := client.Vals.Search("wolf/XKCDComicOfTheDay")
	fmt.Println(res)
}
