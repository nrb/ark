/*
Copyright 2019 the Heptio Ark contributors.

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

package install

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"k8s.io/client-go/dynamic"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	"github.com/heptio/velero/pkg/cmd/util/flag"
	"github.com/heptio/velero/pkg/cmd/util/output"
	"github.com/heptio/velero/pkg/install"
)

// InstallOptions collects all the options for installing Velero into a Kubernetes cluster.
type InstallOptions struct {
	Namespace            string
	Image                string
	BucketName           string
	Prefix               string
	ProviderName         string
	RestoreOnly          bool
	Secret               string
	DryRun               bool
	BackupStorageConfig  flag.Map
	VolumeSnapshotConfig flag.Map
}

// BindFlags adds command line values to the options struct.
func (o *InstallOptions) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.ProviderName, "provider", o.ProviderName, "provider name for backup and volume storage")
	flags.StringVar(&o.BucketName, "bucket", o.BucketName, "name of the object storage bucket where backups should be stored")
	flags.StringVar(&o.Secret, "secret", o.Secret, "file containing credentials for backup and volume provider")
	flags.StringVar(&o.Image, "image", o.Image, "image to use for the Velero and restic server pods. Optional.")
	flags.StringVar(&o.Prefix, "prefix", o.Prefix, "prefix under which all Velero data should be stored within the bucket. Optional.")
	flags.Var(&o.BackupStorageConfig, "backup-location-config", "configuration to use for the backup storage location. Format is key1=value1,key2=value2")
	flags.Var(&o.VolumeSnapshotConfig, "snapshot-location-config", "configuration to use for the volume snapshot location. Format is key1=value1,key2=value2")
	flags.BoolVar(&o.RestoreOnly, "restore-only", o.RestoreOnly, "run the server in restore-only mode")
	flags.BoolVar(&o.DryRun, "dry-run", o.DryRun, "generate resources, but don't send them to the cluster. Use with -o")
}

// NewInstallOptions instantiates a new, default InstallOptions stuct.
func NewInstallOptions() *InstallOptions {
	return &InstallOptions{
		Namespace:            api.DefaultNamespace,
		Image:                install.DefaultImage,
		BackupStorageConfig:  flag.NewMap(),
		VolumeSnapshotConfig: flag.NewMap(),
	}
}

// AsVeleroOptions translates the values provided at the command line into values used to instantiate Kubernetes resources
func (o *InstallOptions) AsVeleroOptions() (*install.VeleroOptions, error) {
	realPath, err := filepath.Abs(o.Secret)
	if err != nil {
		return nil, err
	}
	secretData, err := ioutil.ReadFile(realPath)
	if err != nil {
		return nil, err
	}
	return &install.VeleroOptions{
		Namespace:    o.Namespace,
		Image:        o.Image,
		ProviderName: o.ProviderName,
		Bucket:       o.BucketName,
		Prefix:       o.Prefix,
		SecretData:   secretData,
		RestoreOnly:  o.RestoreOnly,
		BSLConfig:    o.BackupStorageConfig.Data(),
		VSLConfig:    o.VolumeSnapshotConfig.Data(),
	}, nil
}

// NewCommand creates a cobra command.
func NewCommand(f client.Factory) *cobra.Command {
	o := NewInstallOptions()
	c := &cobra.Command{
		Use:   "install",
		Short: "Install Velero",
		Long: `
Install Velero onto a Kubernetes cluster using the supplied provider information, such as
the provider's name, a bucket name, and a file containing the credentials to access that bucket.
A prefix within the bucket and configuration for the backup store location may also be supplied.
Additionally, volume snapshot information for the same provider may be supplied.

All required CustomResourceDefinitions will be installed to the server, as well as the
Velero Deployment and associated Restic DaemonSet.

The provided secret data will be created in a Secret named 'cloud-credentials'.

All namespaced resources will be placed in the 'velero' namespace.

Use '-o yaml' or '-o json'  with '--dry-run' to output all generated resources as text instead of sending the resources to the server.
This is useful as a starting point for more customized installations.
		`,
		Run: func(c *cobra.Command, args []string) {
			cmd.CheckError(o.Validate(c, args, f))
			cmd.CheckError(o.Complete(args, f))
			cmd.CheckError(o.Run(c))
		},
	}

	o.BindFlags(c.Flags())
	output.BindFlags(c.Flags())
	output.ClearOutputFlagDefault(c)

	return c
}

// Run executes a command in the context of the provided arguments.
func (o *InstallOptions) Run(c *cobra.Command) error {
	vo, err := o.AsVeleroOptions()
	if err != nil {
		return err
	}

	resources, err := install.AllResources(vo)
	if err != nil {
		return err
	}

	if _, err := output.PrintWithFormat(c, resources); err != nil {
		return err
	}

	if o.DryRun {
		return nil
	}

	clientConfig, err := client.Config("", "", fmt.Sprintf("%s-%s", c.Parent().Name(), c.Name()))
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	err = install.Install(client.NewDynamicFactory(dynamicClient), resources, os.Stdout)
	if err != nil {
		return err
	}
	return nil
}

//Complete completes options for a command.
func (o *InstallOptions) Complete(args []string, f client.Factory) error {
	return nil
}

// Validate validates options provided to a command.
func (o *InstallOptions) Validate(c *cobra.Command, args []string, f client.Factory) error {
	if err := output.ValidateFlags(c); err != nil {
		return err
	}

	if o.BucketName == "" {
		return errors.New("--bucket is required")
	}

	if o.ProviderName == "" {
		return errors.New("--provider is required")
	}

	if o.Secret == "" {
		return errors.New("--secret is required")
	}

	return nil
}
