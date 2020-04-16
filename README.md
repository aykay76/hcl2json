# HCL2 to JSON Converter

This has been created to parse Terraform configurations so that they can be passed through the Open Policy Agent which requires input in JSON format. However, with a few small tweaks it could be used for general HCL format files.

## Usage

Import this module:

```
imports (
    ...
    "github.com/aykay76/hcl2json"
    ...
)
```

Create a new converter:

```
converter := hcl2json.NewConverter()
```

Add a schema if you want to convert additional information, for Terraform provider schema:

```
var s schema.Schema
s.FromBytes(schemaBytes)
converter.AttachSchema(s)
```

Add the files that need to be converted:

```
for filename, filebytes := range filedata {
    fmt.Printf("Adding file %s", filename)
    converter.AddFile(filename, filebytes)
}
```

Get the JSON representation of the converted data:

```
json := converter.ToJSON()
```

Hope this helps.