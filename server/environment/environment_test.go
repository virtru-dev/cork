package environment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/virtru/cork/server/environment"
	"github.com/virtru/cork/testutils"
)

var fakeEnv = map[string]string{
	"CORK_VARS": "VAR1,VAR2",
	"VAR1":      "one",
	"VAR2":      "two",
}

func TestSaveAndLoadEnvironment(t *testing.T) {
	tempDir, err := testutils.NewTempDir()
	assert.NoError(t, err)
	defer tempDir.Remove()
	fakeRetriever := func(key string) string {
		return fakeEnv[key]
	}

	envFilePath := tempDir.InPath("cork.env.json")
	// Save the env
	err = environment.SaveEnvFileWithRetriever(fakeRetriever, envFilePath)
	assert.NoError(t, err)

	// Load the env
	env, err := environment.LoadEnvFile(envFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "one", env["VAR1"])
	assert.Equal(t, "two", env["VAR2"])
}
