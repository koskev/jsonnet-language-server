build:
	go build

schema: build
	./jsonnet-language-server --generate-config-schema

schema-doc: schema
	generate-schema-doc --config-file ./schema-config.yaml schema.json
