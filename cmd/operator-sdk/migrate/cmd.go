// Copyright 2019 The Operator-SDK Authors
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

package migrate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/operator-framework/operator-sdk/internal/scaffold"
	"github.com/operator-framework/operator-sdk/internal/scaffold/ansible"
	"github.com/operator-framework/operator-sdk/internal/scaffold/helm"
	"github.com/operator-framework/operator-sdk/internal/scaffold/input"
	"github.com/operator-framework/operator-sdk/internal/util/projutil"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	headerFile string
	repo       string
)

// NewCmd returns a command that will add source code to an existing non-go operator
func NewCmd() *cobra.Command {
	newCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Adds source code to an operator",
		Long: `operator-sdk migrate adds a main.go source file and any associated source files` +
			`for an operator that is not of the "go" type.`,
		RunE: migrateRun,
	}

	newCmd.Flags().StringVar(&headerFile, "header-file", "",
		"Path to file containing headers for generated Go files. Copied to hack/boilerplate.go.txt")
	newCmd.Flags().StringVar(&repo, "repo", "",
		"Project repository path. Used as the project's Go import path. This must be set if outside of "+
			"$GOPATH/src (e.g. github.com/example-inc/my-operator)")

	return newCmd
}

// migrateRun determines the current operator type and runs the corresponding
// migrate function.
func migrateRun(cmd *cobra.Command, args []string) error {
	projutil.MustInProjectRoot()

	if err := verifyFlags(); err != nil {
		return err
	}

	if repo == "" {
		repo = projutil.GetGoPkg()
	}

	opType := projutil.GetOperatorType()
	switch opType {
	case projutil.OperatorTypeAnsible:
		return migrateAnsible()
	case projutil.OperatorTypeHelm:
		return migrateHelm()
	}
	return fmt.Errorf("operator of type %s cannot be migrated", opType)
}

func verifyFlags() error {
	err := projutil.CheckRepo(repo)
	if err != nil {
		return err
	}
	return nil
}

// migrateAnsible runs the migration process for an ansible-based operator
func migrateAnsible() error {
	wd := projutil.MustGetwd()

	cfg := &input.Config{
		Repo:           repo,
		AbsProjectPath: wd,
		ProjectName:    filepath.Base(wd),
	}

	dockerfile := ansible.DockerfileHybrid{
		Watches: true,
		Roles:   true,
	}
	_, err := os.Stat(ansible.PlaybookYamlFile)
	switch {
	case err == nil:
		dockerfile.Playbook = true
	case os.IsNotExist(err):
		log.Info("No playbook was found, so not including it in the new Dockerfile")
	default:
		return fmt.Errorf("error trying to stat %s: (%v)", ansible.PlaybookYamlFile, err)
	}
	if err := renameDockerfile(); err != nil {
		return err
	}

	s := &scaffold.Scaffold{}
	if headerFile != "" {
		err = s.Execute(cfg, &scaffold.Boilerplate{BoilerplateSrcPath: headerFile})
		if err != nil {
			return fmt.Errorf("boilerplate scaffold failed: (%v)", err)
		}
		s.BoilerplatePath = headerFile
	}

	err = s.Execute(cfg,
		&ansible.GoMod{},
		&scaffold.Tools{},
		&ansible.Main{},
		&dockerfile,
		&ansible.Entrypoint{},
		&ansible.UserSetup{},
		&ansible.AoLogs{},
	)
	if err != nil {
		return fmt.Errorf("migrate ansible scaffold failed: (%v)", err)
	}
	return nil
}

// migrateHelm runs the migration process for a helm-based operator
func migrateHelm() error {
	wd := projutil.MustGetwd()

	cfg := &input.Config{
		Repo:           repo,
		AbsProjectPath: wd,
		ProjectName:    filepath.Base(wd),
	}

	if err := renameDockerfile(); err != nil {
		return err
	}

	s := &scaffold.Scaffold{}
	if headerFile != "" {
		err := s.Execute(cfg, &scaffold.Boilerplate{BoilerplateSrcPath: headerFile})
		if err != nil {
			return fmt.Errorf("boilerplate scaffold failed: (%v)", err)
		}
		s.BoilerplatePath = headerFile
	}

	err := s.Execute(cfg,
		&helm.GoMod{},
		&scaffold.Tools{},
		&helm.Main{},
		&helm.DockerfileHybrid{
			Watches:    true,
			HelmCharts: true,
		},
		&helm.Entrypoint{},
		&helm.UserSetup{},
	)
	if err != nil {
		return fmt.Errorf("migrate helm scaffold failed: (%v)", err)
	}
	return nil
}

func renameDockerfile() error {
	dockerfilePath := filepath.Join(scaffold.BuildDir, scaffold.DockerfileFile)
	newDockerfilePath := dockerfilePath + ".sdkold"
	err := os.Rename(dockerfilePath, newDockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to rename Dockerfile: (%v)", err)
	}
	log.Infof("Renamed Dockerfile to %s and replaced with newer version. Compare the new Dockerfile to your"+
		" old one and manually migrate any customizations", newDockerfilePath)
	return nil
}
