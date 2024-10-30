package traefik_inline_response_test

import (
	"github.com/mitchellh/mapstructure"
	"github.com/tuxgal/traefik_inline_response"
	"gopkg.in/yaml.v3"
)

func buildConfig(input string) *traefik_inline_response.Config {
	return mapToConfig(yamlToMap(input))
}

func mapToConfig(input map[string]interface{}) *traefik_inline_response.Config {
	result := traefik_inline_response.CreateConfig()

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToSliceHookFunc(","),
		WeaklyTypedInput: true,
		Result:           result,
	})
	if err != nil {
		panic(err)
	}

	err = dec.Decode(input)
	if err != nil {
		panic(err)
	}

	return result
}

func yamlToMap(input string) map[string]interface{} {
	var result map[string]interface{}
	err := yaml.Unmarshal([]byte(input), &result)
	if err != nil {
		panic(err)
	}
	return result
}
