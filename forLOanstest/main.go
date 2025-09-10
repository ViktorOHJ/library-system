package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ViktorOHJ/library-system/loans/clients"
)

func main() {

	client, err := clients.NewLoansClient("localhost:50053", 5*time.Second)

	if err != nil {
		fmt.Println("Error creating client:", err)
		return
	}
	res, err := client.Return(context.Background(), "28")
	if err != nil {
		fmt.Println("Error borrowing book:", err)
		return
	}
	fmt.Println(res)
}
