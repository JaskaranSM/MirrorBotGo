package engine

import (
	"errors"
	"fmt"
	"github.com/dop251/goja"
	"regexp"
	"strings"
)

func escapeError(err error) error {
	if err == nil {
		return err
	}
	return errors.New(strings.ReplaceAll(strings.ReplaceAll(err.Error(), "<", ""), ">", ""))
}

func extractDDLJS(link string, script string, secrets map[string]string) (string, error) {
	r, err := CreateJSRuntime(secrets)
	if err != nil {
		return "", escapeError(err)
	}
	_, err = r.RunString(script)
	if err != nil {
		return "", escapeError(err)
	}
	extract, ok := goja.AssertFunction(r.Get("extract"))
	if !ok {
		return "", fmt.Errorf("extract function not found in the specified script, please recheck")
	}
	value, err := extract(goja.Undefined(), r.ToValue(link))
	if err != nil {
		return "", escapeError(err)
	}
	if goja.IsNull(value) {
		return "", fmt.Errorf("javascript returned null")
	}
	if goja.IsUndefined(value) {
		return "", fmt.Errorf("javascript returned undefined")
	}
	if goja.IsInfinity(value) {
		return "", fmt.Errorf("javascript returned infinity")
	}
	if value == nil {
		return "", fmt.Errorf("internal error occured, please recheck the script")
	}
	return value.String(), nil
}

func ExtractDDL(link string, extractors map[string]string, secrets map[string]string) (string, error) {
	for regex, script := range extractors {
		matched, err := regexp.MatchString(regex, link)
		if err != nil {
			return "", err
		}
		if !matched {
			continue
		}
		return extractDDLJS(link, script, secrets)
	}
	return "", fmt.Errorf("no extractor found for this url, do download normally")
}
