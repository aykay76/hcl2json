package converter

import ()

// Converter : converts from HCL to JSON
type Converter struct {
	files  map[string][]byte
	Output TerraformConfig
	schema *Schema
}

// JSONObj : easier to type version of what it is
type JSONObj map[string]interface{}

// TerraformConfig : represents the config of a TF run
type TerraformConfig struct {
	Providers   []map[string]interface{} `json:"providers"`
	Resources   []map[string]interface{} `json:"resources"`
	Variables   []map[string]interface{} `json:"variables"`
	Outputs     []map[string]interface{} `json:"outputs"`
	Modules     []map[string]interface{} `json:"modules"`
	DataSources []map[string]interface{} `json:"dataSources"`
	Locals      []map[string]interface{} `json:"locals"`
	Terraform   []map[string]interface{} `json:"terraform"`
}
