/*
Copyright 2021 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cli

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/kubevela/apis/core.oam.dev/v1beta1"
	"github.com/oam-dev/kubevela/apis/types"
	"github.com/oam-dev/kubevela/pkg/utils/common"
	"github.com/oam-dev/kubevela/pkg/utils/helm"
	"github.com/oam-dev/kubevela/pkg/utils/util"
)

// UnInstallArgs the args for uninstall command
type UnInstallArgs struct {
	userInput  *UserInput
	helmHelper *helm.Helper
	Args       common.Args
	Namespace  string
	Detail     bool
}

// NewUnInstallCommand creates `uninstall` command to uninstall vela core
func NewUnInstallCommand(c common.Args, order string, ioStreams util.IOStreams) *cobra.Command {
	unInstallArgs := &UnInstallArgs{Args: c, userInput: NewUserInput(), helmHelper: helm.NewHelper()}
	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Uninstalls KubeVela from a Kubernetes cluster",
		Example: `vela uninstall`,
		Long:    "Uninstalls KubeVela from a Kubernetes cluster.",
		Args:    cobra.ExactArgs(0),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			userConfirmation := unInstallArgs.userInput.AskBool("Would you like to uninstall KubeVela from this cluster?", &UserInputOptions{AssumeYes: assumeYes})
			if !userConfirmation {
				return nil
			}
			kubeClient, err := c.GetClient()
			if err != nil {
				return errors.Wrapf(err, "failed to get kube client")
			}
			var apps v1beta1.ApplicationList
			err = kubeClient.List(context.Background(), &apps, &client.ListOptions{
				Namespace: "",
			})
			if err != nil {
				return errors.Wrapf(err, "failed to check app in cluster")
			}
			if len(apps.Items) > 0 {
				return fmt.Errorf("please delete all applications before uninstall. using \"vela ls -A\" view all applications")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ioStreams.Info("Starting to uninstall KubeVela")
			restConfig, err := c.GetConfig()
			if err != nil {
				return errors.Wrapf(err, "failed to get kube config, You can set KUBECONFIG env or make file ~/.kube/config")
			}
			if err := unInstallArgs.helmHelper.UninstallRelease(kubeVelaReleaseName, unInstallArgs.Namespace, restConfig, unInstallArgs.Detail, ioStreams); err != nil {
				return err
			}
			// Clean up vela-system namespace
			kubeClient, err := c.GetClient()
			if err != nil {
				return errors.Wrapf(err, "failed to get kube client")
			}
			if err := deleteNamespace(kubeClient, unInstallArgs.Namespace); err != nil {
				return err
			}
			var namespace corev1.Namespace
			var namespaceExists = true
			if err := kubeClient.Get(cmd.Context(), apitypes.NamespacedName{Name: "kubevela"}, &namespace); err != nil {
				if !apierror.IsNotFound(err) {
					return fmt.Errorf("failed to check if namespace kubevela already exists: %w", err)
				}
				namespaceExists = false
			}
			if namespaceExists {
				fmt.Printf("The namespace kubevela is exist, it is the default database of the velaux\n\n")
				userConfirmation := unInstallArgs.userInput.AskBool("Do you want to delete it?", &UserInputOptions{assumeYes})
				if userConfirmation {
					if err := deleteNamespace(kubeClient, "kubevela"); err != nil {
						return err
					}
				}
			}
			ioStreams.Info("Successfully uninstalled KubeVela")
			ioStreams.Info("Please delete all CRD from cluster using \"kubectl get crd |grep oam | awk '{print $1}' | xargs kubectl delete crd\"")
			return nil
		},
		Annotations: map[string]string{
			types.TagCommandOrder: order,
			types.TagCommandType:  types.TypeSystem,
		},
	}

	cmd.Flags().StringVarP(&unInstallArgs.Namespace, "namespace", "n", "vela-system", "namespace scope for installing KubeVela Core")
	cmd.Flags().BoolVarP(&unInstallArgs.Detail, "detail", "d", true, "show detail log of installation")
	return cmd
}

func deleteNamespace(kubeClient client.Client, namespace string) error {
	var ns corev1.Namespace
	ns.Name = namespace
	return kubeClient.Delete(context.Background(), &ns)
}
