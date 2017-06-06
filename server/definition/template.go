package definition

import (
	"bytes"
	"fmt"
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
	UserParams  map[string]string
}

type TemplateVar struct {
	Type   string
	Lookup string
}

type CorkTemplateRenderer struct {
	Outputs      map[string]map[string]string
	RequiredVars map[string]TemplateVar
	UserParams   map[string]string
	FuncMap      template.FuncMap
	WorkDir      string
	HostWorkDir  string
	CacheDir     string
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
	if options.UserParams == nil {
		options.UserParams = map[string]string{}
	}
	renderer := &CorkTemplateRenderer{
		WorkDir:      options.WorkDir,
		HostWorkDir:  options.HostWorkDir,
		CacheDir:     options.CacheDir,
		Outputs:      map[string]map[string]string{},
		UserParams:   options.UserParams,
		RequiredVars: map[string]TemplateVar{},
	}
	funcMap := template.FuncMap{
		"output":        renderer.outputsResolve,
		"WORK_DIR":      renderer.workDir,
		"HOST_WORK_DIR": renderer.hostWorkDir,
		"CACHE_DIR":     renderer.cacheDir,
		"param":         renderer.userResolve,
	}
	renderer.FuncMap = funcMap
	return renderer
}

func (c *CorkTemplateRenderer) trackRequiredVar(requiredVar TemplateVar) {
	lookup := fmt.Sprintf("%s:%s", requiredVar.Type, requiredVar.Lookup)
	c.RequiredVars[lookup] = requiredVar
}

func (c *CorkTemplateRenderer) outputsResolve(lookup string) string {
	c.trackRequiredVar(TemplateVar{
		Type:   "output",
		Lookup: lookup,
	})
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

func (c *CorkTemplateRenderer) userResolve(lookup string) string {
	c.trackRequiredVar(TemplateVar{
		Type:   "user",
		Lookup: lookup,
	})
	userParamValue, ok := c.UserParams[lookup]
	if !ok {
		return ""
	}
	return userParamValue
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

func (c *CorkTemplateRenderer) ResetRequiredVarTracker() {
	c.RequiredVars = map[string]TemplateVar{}
}

func (c *CorkTemplateRenderer) ListRequiredVars() map[string]TemplateVar {
	return c.RequiredVars
}
