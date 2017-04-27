package definition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/virtru/cork/server/definition"
)

func TestTemplateRenderOutputs(t *testing.T) {
	renderer := definition.NewTemplateRenderer()
	renderer.AddOutput("foo", "bar", "baz")
	renderer.AddOutput("foo", "fie", "foe")
	renderer.AddOutput("blah", "blah", "blah")
	rendered1, err := renderer.Render(`{{ outputs "foo.bar"}}`)
	rendered2, err := renderer.Render(`{{ outputs "foo.fie"}}`)
	rendered3, err := renderer.Render(`{{ outputs "blah.blah"}}`)
	if assert.NoError(t, err) {
		assert.Equal(t, "baz", rendered1)
		assert.Equal(t, "foe", rendered2)
		assert.Equal(t, "blah", rendered3)
	}
}

func TestTemplateRenderConstants(t *testing.T) {
	renderer := definition.NewTemplateRendererWithOptions(definition.CorkTemplateRendererOptions{
		WorkDir:     "workDir",
		HostWorkDir: "hostWorkDir",
		CacheDir:    "cacheDir",
	})
	rendered1, err := renderer.Render(`{{ WORK_DIR }}`)
	rendered2, err := renderer.Render(`{{ HOST_WORK_DIR }}`)
	rendered3, err := renderer.Render(`{{ CACHE_DIR }}`)
	if assert.NoError(t, err) {
		assert.Equal(t, "workDir", rendered1)
		assert.Equal(t, "hostWorkDir", rendered2)
		assert.Equal(t, "cacheDir", rendered3)
	}
}
