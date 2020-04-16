package converter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// Schema : Basic interface to schema
type Schema struct {
	// the whole schema direct from the provider
	schema map[string]interface{}

	// references within the schema for faster processing
	resourceSchemas   map[string]interface{}
	dataSourceSchemas map[string]interface{}
}

// LoadSchemaFromFile : load schema from git repository
func (s *Schema) LoadSchemaFromFile(filename string) {
	// Open our jsonFile
	jsonFile, err := os.Open(filename)
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	filebytes, _ := ioutil.ReadAll(jsonFile)

	s.FromBytes(filebytes)
}

// FromBytes : construct a schema from a byte array
func (s *Schema) FromBytes(bytes []byte) {
	err := json.Unmarshal(bytes, &s.schema)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}

	// find the resource schema starting point
	ps := s.schema["provider_schemas"].(map[string]interface{})
	az := ps["azurerm"].(map[string]interface{})
	s.resourceSchemas = az["resource_schemas"].(map[string]interface{})
	s.dataSourceSchemas = az["data_source_schemas"].(map[string]interface{})
}

func (s *Schema) findBlockByName(start map[string]interface{}, name string) map[string]interface{} {
	var found map[string]interface{}

	for k, v := range start {
		switch v.(type) {
		case map[string]interface{}:
			if strings.Compare(k, name) == 0 {
				return v.(map[string]interface{})["block"].(map[string]interface{})
			}
			found = s.findBlockByName(v.(map[string]interface{}), name)
		}
	}

	return found
}

// GetAttribute : Find an attribute in the schema
func (s *Schema) GetAttribute(schema map[string]interface{}, name string) map[string]interface{} {
	attr := schema["attributes"].(map[string]interface{})
	if attrSchema, ok := attr[name]; ok {
		return attrSchema.(map[string]interface{})
	}

	return nil
}

// GetBlock : Find a block in the schema
func (s *Schema) GetBlock(schema map[string]interface{}, name string) map[string]interface{} {
	blockTypes := schema["block_types"]

	if blockTypes != nil {
		block := blockTypes.(map[string]interface{})
		if blockSchema, ok := block[name]; ok {
			return blockSchema.(map[string]interface{})["block"].(map[string]interface{})
		}
	}

	return nil
}

// GetResourceSchema : Returns
func (s *Schema) GetResourceSchema(name string) map[string]interface{} {
	return s.findBlockByName(s.resourceSchemas, name)
}
