// Copyright 2020 Red Hat, Inc. and/or its affiliates
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

package service

import (
	"github.com/kiegroup/kogito-operator/apis"
	"github.com/kiegroup/kogito-operator/apis/app/v1beta1"
	"github.com/kiegroup/kogito-operator/cmd/kogito/command/context"
	"github.com/kiegroup/kogito-operator/cmd/kogito/command/converter"
	"github.com/kiegroup/kogito-operator/cmd/kogito/command/flag"
	"github.com/kiegroup/kogito-operator/cmd/kogito/command/message"
	"github.com/kiegroup/kogito-operator/cmd/kogito/command/shared"
	"github.com/kiegroup/kogito-operator/cmd/kogito/command/util"
	"github.com/kiegroup/kogito-operator/core/client"
	"github.com/kiegroup/kogito-operator/core/client/kubernetes"
	"github.com/kiegroup/kogito-operator/core/logger"
	"github.com/kiegroup/kogito-operator/core/manager"
	"github.com/kiegroup/kogito-operator/core/operator"
	"github.com/kiegroup/kogito-operator/internal/app"
	"github.com/kiegroup/kogito-operator/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RuntimeService is interface to perform Kogito Runtime
type RuntimeService interface {
	InstallRuntimeService(cli *client.Client, flags *flag.RuntimeFlags) (err error)
	DeleteRuntimeService(cli *client.Client, name, project string) (err error)
}

type runtimeService struct {
	resourceCheckService shared.ResourceCheckService
}

// NewRuntimeService create and return runtimeService value
func NewRuntimeService() RuntimeService {
	return runtimeService{
		resourceCheckService: shared.NewResourceCheckService(),
	}
}

// InstallRuntimeService install Kogito runtime service
func (i runtimeService) InstallRuntimeService(cli *client.Client, flags *flag.RuntimeFlags) (err error) {
	log := context.GetDefaultLogger()
	log.Debugf("Installing Kogito Runtime : %s", flags.Name)
	configMap, err := converter.CreateConfigMapFromFile(cli, flags.Name, flags.Project, &flags.ConfigFlags)
	if err != nil {
		return err
	}
	kogitoRuntime := v1beta1.KogitoRuntime{
		ObjectMeta: v1.ObjectMeta{
			Name:      flags.Name,
			Namespace: flags.Project,
		},
		Spec: v1beta1.KogitoRuntimeSpec{
			EnableIstio: flags.EnableIstio,
			Runtime:     converter.FromRuntimeFlagsToRuntimeType(&flags.RuntimeTypeFlags),
			KogitoServiceSpec: v1beta1.KogitoServiceSpec{
				Replicas:              &flags.Replicas,
				Env:                   converter.FromStringArrayToEnvs(flags.Env, flags.SecretEnv),
				Image:                 flags.ImageFlags.Image,
				Resources:             converter.FromPodResourceFlagsToResourceRequirement(&flags.PodResourceFlags),
				ServiceLabels:         util.FromStringsKeyPairToMap(flags.ServiceLabels),
				InsecureImageRegistry: flags.ImageFlags.InsecureImageRegistry,
				PropertiesConfigMap:   configMap,
				Infra:                 flags.Infra,
				Monitoring:            converter.FromMonitoringFlagToMonitoring(&flags.MonitoringFlags),
				Config:                converter.FromConfigFlagsToMap(&flags.ConfigFlags),
				Probes:                converter.FromProbeFlagToKogitoProbe(&flags.ProbeFlags),
				TrustStoreSecret:      flags.TrustStoreSecret,
			},
		},
	}

	log.Debugf("Trying to deploy Kogito Service '%s'", kogitoRuntime.Name)
	// Create the Kogito application
	err = shared.
		ServicesInstallationBuilder(cli, flags.Project).
		CheckOperatorCRDs().
		InstallRuntimeService(&kogitoRuntime).
		GetError()
	if err != nil {
		return err
	}
	if err = printMgmtConsoleInfo(cli, flags.Project); err != nil {
		return err
	}
	return nil
}

func printMgmtConsoleInfo(client *client.Client, project string) error {
	log := context.GetDefaultLogger()
	context := operator.Context{
		Client: client,
		Log:    logger.GetLogger("deploy_runtime"),
		Scheme: meta.GetRegisteredSchema(),
	}
	supportingServiceHandler := app.NewKogitoSupportingServiceHandler(context)
	supportingServiceManager := manager.NewKogitoSupportingServiceManager(context, supportingServiceHandler)
	route, err := supportingServiceManager.FetchKogitoSupportingServiceRoute(project, api.MgmtConsole)
	if err != nil {
		return err
	}
	if len(route) == 0 {
		log.Info(message.RuntimeServiceMgmtConsole)
	} else {
		log.Infof(message.RuntimeServiceMgmtConsoleEndpoint, route)
	}
	return nil
}

// DeleteRuntimeService delete Kogito runtime service
func (i runtimeService) DeleteRuntimeService(cli *client.Client, name, project string) (err error) {
	log := context.GetDefaultLogger()
	if err := i.resourceCheckService.CheckKogitoRuntimeExists(cli, name, project); err != nil {
		return err
	}
	log.Debugf("About to delete service %s in namespace %s", name, project)
	if err := kubernetes.ResourceC(cli).Delete(&v1beta1.KogitoRuntime{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
	}); err != nil {
		return err
	}
	log.Infof("Successfully deleted Kogito Service %s in the Project %s", name, project)
	return nil
}
