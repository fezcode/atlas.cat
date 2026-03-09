package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Item struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Value   string   `json:"value"`
	Tags    []string `json:"tags"`
	Active  bool     `json:"active"`
	Comment string   `json:"comment"`
}

func main() {
	file, err := os.Create("large_test.json")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer file.Close()

	items := make([]Item, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = Item{
			ID:      i + 1,
			Name:    fmt.Sprintf("Item %d", i+1),
			Value:   fmt.Sprintf("Value for item %d", i+1),
			Tags:    []string{"testing", "atlas", "cat"},
			Active:  i%2 == 0,
			Comment: "This is a large JSON file for testing atlas.cat performance and scrolling.",
		}
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(items); err != nil {
		fmt.Printf("Error encoding: %v\n", err)
	}

	fmt.Println("Successfully generated large_test.json (1000 items)")
}
