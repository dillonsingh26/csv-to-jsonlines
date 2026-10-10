package jsonl_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/dillonsingh26/csv-to-jsonlines/jsonl"
)

// Example shows how another Go program can reuse the package.
func Example() {
	input := strings.NewReader("value,income,age\n452600,8.3252,41\n")
	stats, err := jsonl.Convert(input, os.Stdout)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("rows:", stats.Rows)
	// Output:
	// {"value":452600,"income":8.3252,"age":41}
	// rows: 1
}
