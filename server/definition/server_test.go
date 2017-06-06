// Test the server definition
package definition_test

import (
	"fmt"
	"testing"

	"github.com/virtru/cork/server/definition"

	"github.com/stretchr/testify/assert"
)

var good_definition_yml = `
params:
  build_param:
    type: string
    description: "Some build param"

stages:
  validate:
    - name: lint
      type: container
      args:
        image: virtrudocker.io/node-lint:v1
        command: "/usr/sbin/some-command-on-that-server"
      match_tags:
        - ci
    
    - name: security 
      type: container
      args:
        image: virtrudocker.io/node-security:v1
      match_tags:
        - ci

  build:
    - type: stage
      args:
        stage: validate

    - name: build_container
      type: command
      args:
        command: build
        params:
          build_param: '{{ param "build_param" }}'
      outputs:
        - app_image

    # Export a key/value from this cork stage
    - type: export
      args:
        export:
          name: app_image
          value: '{{ output "build_container.app_image" }}'

  test:
    - type: stage
      args:
        stage: build

    - name: test
      type: command
      args:
        command: test
        params:
          app_image: '{{ output "build_container.app_image" }}'

  default:
    - type: stage
      args:
        stage: test
`

var circular_definition_yml = `
stages:
  foo:
    - type: stage
      args:
        stage: bar

  bar:
    - type: stage
      args:
        stage: foo

  default:
    - type: stage
      args:
        stage: foo
`

var invalid_step_type = `
stages:
  foo:
    - type: blah
`

var unavailable_output_definition_yml = `
version: 1

params:
  foo:
    type: string
    description: This is foo
  bar:
    type: string
    description: this is bar

stages:
  build:
    - name: build_container
      type: command
      args:
        command: build_container
        params:
          foo: '{{ param "foo" }}'
          not_available: '{{ output "unknown_step.not_available" }}'
      outputs: 
        - app_image
    
    - type: stage
      args:
        stage: input

  test:
    - name: test
      type: command
      args:
        command: test
        params:
          bar: '{{ param "bar" }}'
          app_image: '{{ output "build_container.app_image" }}'

  default:
    - type: stage
      args:
        stage: build
`

var undefined_vars_definition_yml = `
version: 1

stages:
  foo:
    - name: foo
      type: command
      args:
        command: foo
        params:
          foo: '{{ param "foo" }}'
          bar: '{{ param "bar" }}'
`

var bad_definitions_table = map[string]string{
	"has circular dependencies": circular_definition_yml,
	"has an invalid Step type":  invalid_step_type,
	"has an unavailable Step":   unavailable_output_definition_yml,
	"has an undefined param":    undefined_vars_definition_yml,
}

func TestGoodDefLoadFromString(t *testing.T) {
	def, err := definition.LoadFromString(good_definition_yml)
	if !assert.NoError(t, err) {
		return
	}
	steps, err := def.ListSteps("default")
	if assert.NoError(t, err) {
		assert.Equal(t, 5, len(steps))
		var stepNames []string
		for _, step := range steps {
			stepNames = append(stepNames, fmt.Sprintf("%s:%s", step.Type, step.Name))
		}
		assert.EqualValues(
			t,
			[]string{"container:lint", "container:security", "command:build_container", "export:", "command:test"},
			stepNames,
		)
	}
}

func TestBadDefLoadFromString(t *testing.T) {
	for failReason, badDefStr := range bad_definitions_table {
		_, err := definition.LoadFromString(badDefStr)
		if err == nil {
			t.Errorf("Definition should not successfully load because it %s", failReason)
		}
	}
}
