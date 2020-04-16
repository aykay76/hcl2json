// Inspired by https://github.com/instrumenta/conftest/blob/master/parser/hcl2/convert.go

package converter

import (
	"fmt"
	"github.com/zclconf/go-cty/cty"
	ctyconvert "github.com/zclconf/go-cty/cty/convert"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	"strings"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func (tc *TerraformConfig) addBlock(blockType string, block map[string]interface{}) {
	switch blockType {
	case "resource":
		tc.Resources = append(tc.Resources, block)
	case "data":
		tc.DataSources = append(tc.DataSources, block)
	case "variable":
		tc.Variables = append(tc.Variables, block)
	case "module":
		tc.Modules = append(tc.Modules, block)
	case "provider":
		tc.Providers = append(tc.Providers, block)
	case "locals":
		tc.Locals = append(tc.Locals, block)
	case "terraform":
		tc.Terraform = append(tc.Terraform, block)
	case "output":
		tc.Outputs = append(tc.Outputs, block)
	}
}

func (c *Converter) rangeSource(bytes []byte, r hcl.Range) string {
	return string(bytes[r.Start.Byte:r.End.Byte])
}

func (c *Converter) convertBlock(block *hclsyntax.Block, out JSONObj, bytes []byte, schemaBlock map[string]interface{}) error {
	var key string = block.Type

	out["labels"] = block.Labels
	out["type"] = block.Type
	out["range"] = c.convertRangeToJSON(block.TypeRange)

	for _, label := range block.Labels {
		key = fmt.Sprintf("%s.%s", key, label)
	}

	c.convertBody(key, out, block.Body, bytes, schemaBlock)

	return nil
}

func (c *Converter) convertRangeToJSON(r hcl.Range) map[string]interface{} {
	jo := make(map[string]interface{})

	jo["filename"] = r.Filename

	s := map[string]interface{}{
		"byte":   r.Start.Byte,
		"line":   r.Start.Line,
		"column": r.Start.Column,
	}
	jo["start"] = s

	e := map[string]interface{}{
		"byte":   r.End.Byte,
		"line":   r.End.Line,
		"column": r.End.Column,
	}
	jo["end"] = e

	return jo
}

func (c *Converter) attributeExists(key string, arr []interface{}) bool {
	for _, v := range arr {
		if v.(map[string]interface{})["key"] == key {
			return true
		}
	}
	return false
}

func (c *Converter) convertBody(key string, out JSONObj, body *hclsyntax.Body, bytes []byte, schemaBlock map[string]interface{}) {
	var err error
	var attrSchema map[string]interface{}
	var blockSchema map[string]interface{}

	nestedBlocks := make([]interface{}, 0)

	// each attribute will be added to the output block in an "attributes" array
	for key, value := range body.Attributes {
		if schemaBlock != nil {
			attrSchema = c.schema.GetAttribute(schemaBlock, key)
		}

		// if we have a schema for this attribute, inject it into the attribute
		var attrValue interface{}
		attrValue, err = c.convertExpression(value.Expr, bytes)

		combined := make(map[string]interface{})
		combined["key"] = key
		combined["value"] = attrValue
		if attrSchema != nil {
			combined["schema"] = attrSchema
		}
		combined["range"] = c.convertRangeToJSON(value.SrcRange)
		out[key] = combined
	}

	// Add schema from provider so that we can identify required attributes, sensitive outputs etc.
	if schemaBlock != nil {
		schemaAttributes := schemaBlock["attributes"].(map[string]interface{})
		for k, v := range schemaAttributes {
			if _, ok := out[key]; !ok {
				combined := make(map[string]interface{})
				combined["schema"] = v
				combined["key"] = k
				out[k] = combined
			}
		}
	}

	for _, block := range body.Blocks {
		// try to get an appropriate schema
		if schemaBlock != nil {
			blockSchema = c.schema.GetBlock(schemaBlock, block.Type)
		}

		nestedBlock := make(JSONObj)
		err = c.convertBlock(block, nestedBlock, bytes, blockSchema)
		if err != nil {
			return
		}

		nestedBlocks = append(nestedBlocks, nestedBlock)
	}

	if len(nestedBlocks) > 0 {
		out["blocks"] = nestedBlocks
	}

	return
}

func (c *Converter) convertExpression(expr hclsyntax.Expression, bytes []byte) (interface{}, error) {
	switch value := expr.(type) {
	case *hclsyntax.AnonSymbolExpr:
		return nil, nil
	case *hclsyntax.BinaryOpExpr:
		return c.convertBinaryOp(value, bytes)
	case *hclsyntax.ConditionalExpr:
		return c.convertTemplateConditional(value, bytes)
	case *hclsyntax.ForExpr:
		return c.convertFor(value, bytes)
	case *hclsyntax.FunctionCallExpr:
		return c.convertFunctionCall(value, bytes)
	case *hclsyntax.IndexExpr:
		return c.convertIndex(value, bytes)
	case *hclsyntax.LiteralValueExpr:
		return ctyjson.SimpleJSONValue{Value: value.Val}, nil
	case *hclsyntax.ObjectConsExpr:
		return c.convertObjectConstructor(value, bytes)
	case *hclsyntax.ObjectConsKeyExpr:
		return nil, nil
	case *hclsyntax.RelativeTraversalExpr:
		return c.convertRelativeTraversal(value, bytes)
	case *hclsyntax.ScopeTraversalExpr:
		return c.convertScopeTraversal(value, bytes)
	case *hclsyntax.SplatExpr:
		return c.convertSplat(value, bytes)
	case *hclsyntax.TemplateExpr:
		return c.convertTemplate(value, bytes)
	case *hclsyntax.TemplateJoinExpr:
		return nil, nil
	case *hclsyntax.TemplateWrapExpr:
		return c.convertTemplateWrap(value, bytes)
	case *hclsyntax.TupleConsExpr:
		return c.convertTupleCons(value, bytes)
	case *hclsyntax.UnaryOpExpr:
		return nil, nil
	default:
		return c.wrapExpr(expr, bytes), nil
	}
}

func (c *Converter) convertIndex(e *hclsyntax.IndexExpr, bytes []byte) (string, error) {
	return string(bytes[e.SrcRange.Start.Byte:e.SrcRange.End.Byte]), nil
}

func (c *Converter) convertObjectConstructorAsString(e *hclsyntax.ObjectConsExpr, bytes []byte) (string, error) {
	return string(bytes[e.SrcRange.Start.Byte:e.SrcRange.End.Byte]), nil
}

func (c *Converter) convertTupleConstructorAsString(e *hclsyntax.TupleConsExpr, bytes []byte) (string, error) {
	return string(bytes[e.SrcRange.Start.Byte:e.SrcRange.End.Byte]), nil
}

func (c *Converter) convertObjectConstructor(e *hclsyntax.ObjectConsExpr, bytes []byte) (interface{}, error) {
	var object map[string]interface{}

	object = make(map[string]interface{})

	for _, item := range e.Items {
		key, _ := c.convertStringPart(item.KeyExpr, bytes)
		value, _ := c.convertExpression(item.ValueExpr, bytes)

		object[key] = value
	}

	return object, nil
}

func (c *Converter) convertTemplateWrap(e *hclsyntax.TemplateWrapExpr, bytes []byte) (string, error) {
	original := string(bytes[e.SrcRange.Start.Byte:e.SrcRange.End.Byte])
	return strings.ReplaceAll(original, "\"", ""), nil
}

// TODO: needs some work
func (c *Converter) convertScopeTraversal(st *hclsyntax.ScopeTraversalExpr, bytes []byte) (string, error) {
	return strings.ReplaceAll(string(bytes[st.Range().Start.Byte:st.Range().End.Byte]), "\"", "'"), nil
}

func (c *Converter) convertRelativeTraversal(st *hclsyntax.RelativeTraversalExpr, bytes []byte) (string, error) {
	return strings.ReplaceAll(string(bytes[st.Range().Start.Byte:st.Range().End.Byte]), "\"", "'"), nil
}

// TODO: needs work to process objects inside the tuple
func (c *Converter) convertTupleCons(e *hclsyntax.TupleConsExpr, bytes []byte) ([]interface{}, error) {
	var array []interface{}

	array = make([]interface{}, 0)

	for _, item := range e.Exprs {
		item, _ := c.convertExpression(item, bytes)
		if array == nil {
			array = []interface{}{item}
		} else {
			array = append(array, item)
		}
	}

	return array, nil
}

func (c *Converter) convertTemplate(t *hclsyntax.TemplateExpr, bytes []byte) (string, error) {
	if t.IsStringLiteral() {
		// safe because the value is just the string
		v, err := t.Value(nil)
		if err != nil {
			return "", err
		}

		return v.AsString(), nil
	}
	var builder strings.Builder
	for _, part := range t.Parts {
		s, err := c.convertStringPart(part, bytes)
		if err != nil {
			return "", err
		}
		builder.WriteString(s)
	}
	return builder.String(), nil
}

func (c *Converter) convertStringPart(expr hclsyntax.Expression, bytes []byte) (string, error) {
	switch v := expr.(type) {
	case *hclsyntax.AnonSymbolExpr:
		return "[as]", nil
	case *hclsyntax.BinaryOpExpr:
		return c.convertBinaryOp(v, bytes)
	case *hclsyntax.ConditionalExpr:
		return c.convertTemplateConditional(v, bytes)
	case *hclsyntax.ForExpr:
		return c.convertFor(v, bytes)
	case *hclsyntax.FunctionCallExpr:
		return c.convertFunctionCall(v, bytes)
	case *hclsyntax.IndexExpr:
		return c.convertIndex(v, bytes)
	case *hclsyntax.LiteralValueExpr:
		if v.Val.IsNull() {
			return "null", nil
		}

		s, err := ctyconvert.Convert(v.Val, cty.String)
		if err != nil {
			fmt.Println(err)
			return "£", err
		}
		return s.AsString(), nil
	case *hclsyntax.ObjectConsExpr:
		o, e := c.convertObjectConstructorAsString(v, bytes)
		return o, e
	case *hclsyntax.ObjectConsKeyExpr:
		return c.convertStringPart(v.Wrapped, bytes)
	case *hclsyntax.RelativeTraversalExpr:
		return c.convertRelativeTraversal(v, bytes)
	case *hclsyntax.ScopeTraversalExpr:
		return c.convertScopeTraversal(v, bytes)
	case *hclsyntax.SplatExpr:
		return c.convertSplat(v, bytes)
	case *hclsyntax.TemplateExpr:
		return c.convertTemplate(v, bytes)
	case *hclsyntax.TemplateJoinExpr:
		return "[tj]", nil
	case *hclsyntax.TemplateWrapExpr:
		return c.convertTemplateWrap(v, bytes)
	case *hclsyntax.TupleConsExpr:
		return c.convertTupleConstructorAsString(v, bytes)
	case *hclsyntax.UnaryOpExpr:
		return "[uo]", nil
	default:
		return c.wrapExpr(expr, bytes), nil
	}
}

func (c *Converter) convertBinaryOp(e *hclsyntax.BinaryOpExpr, bytes []byte) (string, error) {
	return string(bytes[e.SrcRange.Start.Byte:e.SrcRange.End.Byte]), nil
}

func (c *Converter) convertSplat(e *hclsyntax.SplatExpr, bytes []byte) (string, error) {
	return string(bytes[e.SrcRange.Start.Byte:e.SrcRange.End.Byte]), nil
}

func (c *Converter) convertFunctionCall(e *hclsyntax.FunctionCallExpr, bytes []byte) (string, error) {
	var builder strings.Builder

	name := string(bytes[e.NameRange.Start.Byte:e.NameRange.End.Byte])
	builder.WriteString(name)
	builder.WriteString("(")

	for i, arg := range e.Args {
		val, _ := c.convertStringPart(arg, bytes)
		builder.WriteString(val)
		if i < len(e.Args)-1 {
			builder.WriteString(", ")
		}
	}

	builder.WriteString(")")

	return builder.String(), nil
}

func (c *Converter) convertTemplateConditional(expr *hclsyntax.ConditionalExpr, bytes []byte) (string, error) {
	var builder strings.Builder
	builder.WriteString("( ")
	builder.WriteString(c.rangeSource(bytes, expr.Condition.Range()))
	builder.WriteString(" ? ")
	trueResult, err := c.convertStringPart(expr.TrueResult, bytes)
	if err != nil {
		return "", nil
	}
	builder.WriteString(trueResult)
	falseResult, err := c.convertStringPart(expr.FalseResult, bytes)
	if err != nil {
		return "", nil
	}
	if len(falseResult) > 0 {
		builder.WriteString(" : ")
		builder.WriteString(falseResult)
	}
	builder.WriteString(" )")

	return builder.String(), nil
}

func (c *Converter) convertFor(expr *hclsyntax.ForExpr, bytes []byte) (string, error) {
	var builder strings.Builder
	builder.WriteString("%{for ")
	if len(expr.KeyVar) > 0 {
		builder.WriteString(expr.KeyVar)
		builder.WriteString(", ")
	}
	builder.WriteString(expr.ValVar)
	builder.WriteString(" in ")
	builder.WriteString(c.rangeSource(bytes, expr.CollExpr.Range()))
	builder.WriteString("}")
	templ, err := c.convertStringPart(expr.ValExpr, bytes)
	if err != nil {
		return "", err
	}
	builder.WriteString(templ)
	builder.WriteString("%{endfor}")

	return builder.String(), nil
}

func (c *Converter) wrapExpr(expr hclsyntax.Expression, bytes []byte) string {
	return "${" + c.rangeSource(bytes, expr.Range()) + "}"
}
