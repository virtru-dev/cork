package main

import "github.com/virtru/cork/client"
import "github.com/virtru/cork/utils/params"
import "fmt"

func main() {
	corkClient, err := client.New(":11900")
	if err != nil {
		panic(err)
	}

	providedParams := make(map[string]string)
	paramsProvider := params.NewInteractiveProvider(providedParams)

	_, err = corkClient.StageExecute("build", paramsProvider)
	if err != nil {
		panic(err)
	}
	fmt.Println("hrm")
}
