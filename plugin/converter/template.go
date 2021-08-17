// Copyright 2019 Drone IO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !oss

package converter

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/drone/drone/core"
	"github.com/drone/drone/plugin/converter/jsonnet"
	"github.com/drone/drone/plugin/converter/starlark"

	"gopkg.in/yaml.v2"
)

var (
	// TemplateFileRE regex to verifying kind is template.
	TemplateFileRE          = regexp.MustCompile("^kind:\\s+template+\\n")
	ErrTemplateNotFound     = errors.New("template converter: template name given not found")
	ErrTemplateSyntaxErrors = errors.New("template converter: there is a problem with the yaml file provided")
)

func Template(templateStore core.TemplateStore) core.ConvertService {
	return &templatePlugin{
		templateStore: templateStore,
	}
}

type templatePlugin struct {
	templateStore core.TemplateStore
}

func (p *templatePlugin) Convert(ctx context.Context, req *core.ConvertArgs) (*core.Config, error) {
	// check type is yaml
	if strings.HasSuffix(req.Repo.Config, ".yml") == false {
		return nil, nil
	}

	// check kind is template
	if TemplateFileRE.MatchString(req.Config.Data) == false {
		return nil, nil
	}
	// map to templateArgs
	var templateArgs core.TemplateArgs
	err := yaml.Unmarshal([]byte(req.Config.Data), &templateArgs)
	if err != nil {
		return nil, ErrTemplateSyntaxErrors
	}
	// get template from db
	template, err := p.templateStore.FindName(ctx, templateArgs.Load, req.Repo.Namespace)
	if err == sql.ErrNoRows {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}

	switch filepath.Ext(templateArgs.Load) {
	case ".yml", ".yaml":
		return parseYaml(template, templateArgs)
	case ".star", ".starlark", ".script":
		return parseStarlark(req, template, templateArgs)
	case ".jsonnet":
		return parseJsonnet(req, template, templateArgs)
	default:
	}

	return nil, nil
}

func parseYaml(t *core.Template, templateArgs core.TemplateArgs) (*core.Config, error) {
	tmpl, err := template.New(t.Name).Parse(t.Data)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = tmpl.Execute(&out, templateArgs.Data)
	if err != nil {
		return nil, err
	}
	return &core.Config{
		Data: fmt.Sprint(out),
	}, nil
}

func parseJsonnet(req *core.ConvertArgs, template *core.Template, templateArgs core.TemplateArgs) (*core.Config, error) {
	file, err := jsonnet.Parse(req, template, templateArgs.Data)
	if err != nil {
		return nil, err
	}
	return &core.Config{
		Data: file,
	}, nil
}

func parseStarlark(req *core.ConvertArgs, template *core.Template, templateArgs core.TemplateArgs) (*core.Config, error) {
	file, err := starlark.Parse(req, template, templateArgs.Data)
	if err != nil {
		return nil, err
	}
	return &core.Config{
		Data: file,
	}, nil
}
