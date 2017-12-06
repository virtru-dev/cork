// Test the server definition
package definition_test

import (
	"fmt"
	"testing"

	"github.com/virtru/cork/server/definition"

	log "github.com/sirupsen/logrus"

	"strings"

	"sort"

	"github.com/stretchr/testify/assert"
)

var good_definition_yml = `
version: 1

params:
  build_param:
    type: string
    description: "Some build param"
  other_param:
    type: string
    description: "some other param"
  default_param:
    type: string
    description: "some param with a default"
    default: hello

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
          other_param: '{{ param "other_param" }}'
          default_param: '{{ param "default_param" }}'
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
version: 1

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
version: 1

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

var undefined_template_function_definition_yml = `
version: 1

stages:
  build:
    - name: build_container
      type: command
      args:
        command: build_container
      outputs: 
        - app_image
    
  test:
    - name: test
      type: command
      args:
        command: test
        params:
          app_image: '{{ does_not_exist "build_container.app_image" }}'

  default:
    - type: stage
      args:
        stage: build
`

var lacks_unique_step_names_definition_yml = `
version: 1

stages:
  stage1:
    - name: foo
      type: command
      args:
        command: foo
    - name: bar
      type: command
      args:
        command: bar
    - name: foo
      type: command
      args:
        command: some_other

  default:
    - type: stage
      args:
        stage: stage1
`

var lacks_unique_step_names_across_stages_definition_yml = `
version: 1

stages:
  stage1:
    - name: foo
      type: command
      args:
        command: foo
    - name: bar
      type: command
      args:
        command: bar
  stage2:
    - type: stage
      args:
        stage: stage1

    - name: foo
      type: command
      args:
        command: some_other

  default:
    - type: stage
      args:
        stage: stage2
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

var no_version_definition_yml = `
stages:
  foo:
    - name: foo
      type: command
`

var wrong_version_definition_yml = `
version: 2

stages:
  foo:
    - name: foo
      type: command
`

var bad_definitions_table = map[string]string{
	"has circular dependencies":             circular_definition_yml,
	"has an invalid Step type":              invalid_step_type,
	"has an unavailable Step":               unavailable_output_definition_yml,
	"has an undefined param":                undefined_vars_definition_yml,
	"has an undefined template function":    undefined_template_function_definition_yml,
	"lacks unique step names":               lacks_unique_step_names_definition_yml,
	"lacks unique step names across stages": lacks_unique_step_names_across_stages_definition_yml,
}

var bad_version_definitions_table = map[string]string{
	"does not have a version": no_version_definition_yml,
	"does not have version 1": wrong_version_definition_yml,
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

		// Check the list of the stages
		stages := def.ListStages()
		sort.Strings(stages)
		assert.EqualValues(t, []string{"build", "default", "test", "validate"}, stages)

		// Check that the definition loads the proper required user params
		requiredBuildParams, err := def.RequiredUserParamsForStage("build")
		if err != nil {
			t.Errorf("Error occured gathering required user params for build: %+v", err)
			return
		}
		sort.Strings(requiredBuildParams)
		assert.EqualValues(t, []string{"build_param", "default_param", "other_param"}, requiredBuildParams)

		requiredValidateParams, err := def.RequiredUserParamsForStage("validate")
		if err != nil {
			t.Errorf("Error occured gathering required user params for validate: %+v", err)
			return
		}
		assert.EqualValues(t, 0, len(requiredValidateParams))

		requiredDefaultParams, err := def.RequiredUserParamsForStage("default")
		if err != nil {
			t.Errorf("Error occured gathering required user params for default: %+v", err)
			return
		}
		sort.Strings(requiredDefaultParams)
		assert.EqualValues(t, []string{"build_param", "default_param", "other_param"}, requiredDefaultParams)

		_, err = def.RequiredUserParamsForStage("does_not_exist")
		assert.Error(t, err, "does_not_exist should cause an error")
	}
}

func TestBadDefLoadFromString(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	for failReason, badDefStr := range bad_definitions_table {
		_, err := definition.LoadFromString(badDefStr)
		if err == nil {
			t.Errorf("Definition should not successfully load because it %s", failReason)
			continue
		}

		if strings.Contains(err.Error(), "version") {
			t.Errorf("Bad definition should not fail because of a bad version")
		}
	}
}
func TestBadDefLoadFromStringForVersionCheck(t *testing.T) {
	for failReason, badDefStr := range bad_version_definitions_table {
		_, err := definition.LoadFromString(badDefStr)
		if !strings.Contains(err.Error(), "version") {
			t.Errorf("Bad definition should fail because of a bad version")
		}
		if err == nil {
			t.Errorf("Definition should not successfully load because it %s", failReason)
		}
	}
}
