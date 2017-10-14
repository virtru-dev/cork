package params

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/fatih/color"
	"github.com/segmentio/go-prompt"
	pb "github.com/virtru/cork/protocol"
)

type InteractiveParamProvider struct {
	ProvidedParams map[string]string
}

func NewInteractiveProvider(providedParams map[string]string) *InteractiveParamProvider {
	if providedParams == nil {
		providedParams = make(map[string]string)
	}
	return &InteractiveParamProvider{
		ProvidedParams: providedParams,
	}
}

func (ipp *InteractiveParamProvider) LoadParams(paramDefinitions map[string]*pb.ParamDefinition) (map[string]string, error) {
	resolvedParams := make(map[string]string)
	if len(paramDefinitions) == 0 {
		return resolvedParams, nil
	}
	for paramName, paramDefinition := range paramDefinitions {
		providedParamValue, ok := ipp.ProvidedParams[paramName]
		if ok {
			resolvedParams[paramName] = providedParamValue
			continue
		}

		var paramValue string
		fmt.Println("")
		if paramDefinition.GetHasDefault() {
			resolvedParams[paramName] = paramDefinition.GetDefault()
			continue
		}

		if paramDefinition.GetDescription() != "" {
			color.Blue(`Param "%s": %s`, paramName, paramDefinition.GetDescription())
		}

		if paramDefinition.GetIsSensitive() {
			paramValue = prompt.PasswordMasked("Input")
		} else {
			paramValue = prompt.String("Input")
		}
		resolvedParams[paramName] = paramValue
		log.Debugf(`Got input %s="%s"`, paramName, paramValue)
	}
	return resolvedParams, nil
}
