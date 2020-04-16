package converter

import (
	"encoding/json"
	"fmt"
	"os"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// NewConverter : construct a new converter object
func NewConverter() Converter {
	return Converter{
		files:  make(map[string][]byte),
		Output: TerraformConfig{},
	}
}

// AttachSchema : attach a schema for the conversion of provider resources
func (c *Converter) AttachSchema(s Schema) {
	c.schema = &s
}

// AddFile : add a file to the conversion process
func (c *Converter) AddFile(name string, bytes []byte) {
	// do a simple HCL parse on the file starting from the beginning
	parsed, diags := hclsyntax.ParseConfig(bytes, name, hcl.Pos{Byte: 0, Line: 1, Column: 1})
	if diags.HasErrors() {
		var details []error
		for _, each := range diags.Errs() {
			details = append(details, each)
		}
	}

	c.files[name] = bytes
	body := parsed.Body.(*hclsyntax.Body)

	// rather than use the generic convertBody function, we know there are no attributes
	// and all the blocks are of a specific type, i want to bunch the similar blocks together
	// so will process the high level blocks here and group them according to type
	var err error
	var out JSONObj

	// for each block within the file we will convert the block and add it to the output based on the block type
	for _, block := range body.Blocks {
		var schemaBlock map[string]interface{}

		// if block type is "resource" I need to get the corresponding schema block from the schema
		if block.Type == "resource" {
			resourceType := block.Labels[0]

			if c.schema != nil {
				schemaBlock = c.schema.GetResourceSchema(resourceType)
			}
		}

		out = make(JSONObj)
		err = c.convertBlock(block, out, bytes, schemaBlock)
		if err != nil {
			fmt.Println(err)
		}

		c.Output.addBlock(block.Type, out)
	}
}

// ToJSON : return an indented JSON representation of the config
func (c *Converter) ToJSON() string {
	j, err := json.MarshalIndent(c.Output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal hcl2 to json: %w", err)
	}
	return string(j)
}
