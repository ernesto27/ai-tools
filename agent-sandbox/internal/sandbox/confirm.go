package sandbox

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// confirm asks a yes/no question. Anything but an explicit yes is a no, and so
// is the end of the input: with no terminal attached there is nobody to ask.
func confirm(in io.Reader, out io.Writer, question string) (bool, error) {
	fmt.Fprint(out, question)

	// ReadString returns io.EOF along with an answer that has no newline, so
	// only some other error is fatal.
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	if errors.Is(err, io.EOF) {
		fmt.Fprintln(out)
	}

	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}
