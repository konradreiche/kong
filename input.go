package kong

import (
	"fmt"
)

// ReadString reads the user input from stdin and returns the input as a
// string.
func ReadString(prompt string) (string, error) {
	var input string
	fmt.Print(prompt)
	if _, err := fmt.Scanln(&input); err != nil {
		return "", err
	}
	return input, nil
}
