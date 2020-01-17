// Copyright 2018 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genutil

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/operator-framework/operator-sdk/internal/scaffold"
	"github.com/operator-framework/operator-sdk/internal/util/k8sutil"
	"github.com/operator-framework/operator-sdk/internal/util/projutil"

	log "github.com/sirupsen/logrus"
	generatorargs "k8s.io/kube-openapi/cmd/openapi-gen/args"
	"k8s.io/kube-openapi/pkg/generators"
)

// OpenAPIGen generates OpenAPI validation specs for all CRD's in dirs.
func OpenAPIGen() error {
	projutil.MustInProjectRoot()

	repoPkg := projutil.GetGoPkg()

	gvMap, err := k8sutil.ParseGroupSubpackages(scaffold.ApisDir)
	if err != nil {
		return fmt.Errorf("failed to parse group versions: %v", err)
	}
	gvb := &strings.Builder{}
	for g, vs := range gvMap {
		gvb.WriteString(fmt.Sprintf("%s:%v, ", g, vs))
	}

	log.Infof("Running OpenAPI code-generation for Custom Resource group versions: [%v]\n", gvb.String())

	apisPkg := filepath.Join(repoPkg, scaffold.ApisDir)
	fqApis := k8sutil.CreateFQAPIs(apisPkg, gvMap)
	f := func(a string) error { return openAPIGen(a, fqApis) }
	if err = generateWithHeaderFile(f); err != nil {
		return err
	}

	log.Info("Code-generation complete.")
	return nil
}

func openAPIGen(hf string, fqApis []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := flag.Set("logtostderr", "true"); err != nil {
		return err
	}
	for _, api := range fqApis {
		api = filepath.FromSlash(api)
		// Use relative API path so the generator writes to the correct path.
		apiPath := "." + string(filepath.Separator) + api[strings.Index(api, scaffold.ApisDir):]
		args, cargs := generatorargs.NewDefaults()
		// Ignore default output base and set our own output path.
		args.OutputBase = ""
		// openapi-gen already generates a "do not edit" comment.
		args.GeneratedByCommentTemplate = ""
		args.InputDirs = []string{apiPath}
		args.OutputFileBaseName = "zz_generated.openapi"
		args.OutputPackagePath = filepath.Join(wd, apiPath)
		args.GoHeaderFilePath = hf
		// Print API rule violations to stdout
		cargs.ReportFilename = "-"
		if err := generatorargs.Validate(args); err != nil {
			return fmt.Errorf("openapi-gen argument validation error: %v", err)
		}

		err := args.Execute(
			generators.NameSystems(),
			generators.DefaultNameSystem(),
			generators.Packages,
		)
		if err != nil {
			return fmt.Errorf("openapi-gen generator error: %v", err)
		}
	}
	return nil
}
