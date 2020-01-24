/*
Copyright 2020 the Velero contributors.

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

package restore

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// The CRDV1PreserveUnknownFieldsAction will take a CRD and inspect it for the API version and the PreserveUnknownFields value.
// If the API Version is 1 and the PreserveUnknownFields value is True, then the x-preserve-unknown-fields value in the OpenAPIV3 schema will be set to True
// and PreserveUnknownFields set to False in order to allow Kubernetes 1.16+ servers to accept the object.
type CRDV1PreserveUnknownFieldsAction struct {
	logger logrus.FieldLogger
}

func NewCRDV1PreserveUnknownFieldsAction(logger logrus.FieldLogger) *CRDV1PreserveUnknownFieldsAction {
	return &CRDV1PreserveUnknownFieldsAction{logger: logger}
}

func (c *CRDV1PreserveUnknownFieldsAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"customresourcedefinition.apiextensions.k8s.io"},
	}, nil
}

func (c *CRDV1PreserveUnknownFieldsAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	c.logger.Info("Executing CRDV1PreserveUnknownFieldsAction")

	log := c.logger.WithField("plugin", "CRDV1PreserveUnknownFieldsAction")

	version, _, err := unstructured.NestedString(input.Item.UnstructuredContent(), "apiVersion")
	if err != nil {
		return nil, errors.Wrap(err, "could not get CRD version")
	}

	// We don't want to "fix" anything in beta CRDS at the moment, just v1 versions with preserveunknownfields = true
	if version == "apiextensions.k8s.io/v1beta1" {
		return &velero.RestoreItemActionExecuteOutput{
			UpdatedItem: input.Item,
		}, nil
	}

	var crd apiextv1.CustomResourceDefinition

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), &crd); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to custom resource definition")
	}

	// The v1 API doesn't allow the PreserveUnknownFields value to be true, so make sure the schema flag is set instead
	if crd.Spec.PreserveUnknownFields {
		// First, change the top-level value since the Kubernetes API server on 1.16+ will generate errors otherwise.
		log.Info("Set PreserveUnknownFields to False")
		crd.Spec.PreserveUnknownFields = false

		// Make sure all versions are set to preserve unknown fields
		for _, v := range crd.Spec.Versions {
			// Use the address, since the XPreserveUnknownFields value is undefined or true (false is not allowed)
			preserve := true
			v.Schema.OpenAPIV3Schema.XPreserveUnknownFields = &preserve
			log.Infof("Set x-preserve-unknown-fields in Open API for schema version %s", v.Name)
		}
	}

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&crd)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert crd to runtime.Unstructured")
	}

	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem: &unstructured.Unstructured{Object: res},
	}, nil
}
