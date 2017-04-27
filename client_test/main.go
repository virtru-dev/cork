package main

import "github.com/virtru/cork/client"
import "fmt"

func main() {
	corkClient, err := client.New(":11900")
	if err != nil {
		panic(err)
	}

	err = corkClient.StageExecute("build")
	if err != nil {
		panic(err)
	}
	fmt.Println("hrm")
}
