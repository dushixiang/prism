package main

import (
	"fmt"
	"strings"
	"time"
)

func main() {

	d := 5*time.Minute + 40*time.Second
	rounded := d.Round(time.Minute)
	hold, _ := strings.CutSuffix(rounded.String(), "0s")
	fmt.Println(hold) // 输出：6m

}
