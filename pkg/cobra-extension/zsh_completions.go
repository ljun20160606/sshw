package cobra_extension

import (
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"io"
)

// GenZshCompletion generates zsh completion and writes to the passed writer.
func GenZshCompletion(c *cobra.Command, w io.Writer) error {
	buffer := bytes.NewBuffer(nil)
	if err := c.GenZshCompletion(buffer); err != nil {
		return err
	}
	name := c.Name()
	// original zsh completion does not work
	// so manually add below content
	compdef := fmt.Sprintf("compdef _%v %v", name, name)
	buffer.WriteString(compdef)
	if _, err := buffer.WriteTo(w); err != nil {
		return err
	}
	return nil
}
