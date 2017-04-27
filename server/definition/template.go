package definition

import (
	"bytes"
	"html/template"
	"os"
	"strings"
)

type Empty struct {
}

type CorkTemplateRendererOptions struct {
	WorkDir     string
	HostWorkDir string
	CacheDir    string
}

type CorkTemplateRenderer struct {
	Outputs     map[string]map[string]string
	FuncMap     template.FuncMap
	WorkDir     string
	HostWorkDir string
	CacheDir    string
}

func NewTemplateRenderer() *CorkTemplateRenderer {
	rendererOptions := CorkTemplateRendererOptions{
		WorkDir:     os.Getenv("CORK_WORK_DIR"),
		HostWorkDir: os.Getenv("CORK_HOST_WORK_DIR"),
		CacheDir:    os.Getenv("CORK_CACHE_DIR"),
	}
	return NewTemplateRendererWithOptions(rendererOptions)
}

func NewTemplateRendererWithOptions(options CorkTemplateRendererOptions) *CorkTemplateRenderer {
	renderer := &CorkTemplateRenderer{
		WorkDir:     options.WorkDir,
		HostWorkDir: options.HostWorkDir,
		CacheDir:    options.CacheDir,
		Outputs:     map[string]map[string]string{},
	}
	funcMap := template.FuncMap{
		"outputs":       renderer.outputsResolve,
		"WORK_DIR":      renderer.workDir,
		"HOST_WORK_DIR": renderer.hostWorkDir,
		"CACHE_DIR":     renderer.cacheDir,
	}
	renderer.FuncMap = funcMap
	return renderer
}

func (c *CorkTemplateRenderer) outputsResolve(lookup string) string {
	splitLookup := strings.Split(lookup, ".")
	stepName := splitLookup[0]
	varName := splitLookup[1]
	stepOutputs, ok := c.Outputs[stepName]
	if !ok {
		return ""
	}
	outputValue, ok := stepOutputs[varName]
	if !ok {
		return ""
	}
	return outputValue
}

func (c *CorkTemplateRenderer) workDir() string {
	return c.WorkDir
}

func (c *CorkTemplateRenderer) hostWorkDir() string {
	return c.HostWorkDir
}

func (c *CorkTemplateRenderer) cacheDir() string {
	return c.CacheDir
}

func (c *CorkTemplateRenderer) AddOutput(stepName string, varName string, value string) {
	stepOutputs, ok := c.Outputs[stepName]
	if !ok {
		stepOutputs = map[string]string{}
		c.Outputs[stepName] = stepOutputs
	}
	stepOutputs[varName] = value
}

func (c *CorkTemplateRenderer) Render(templateStr string) (string, error) {
	tmpl, err := template.New("line").Funcs(c.FuncMap).Parse(templateStr)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, Empty{})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
