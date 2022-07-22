/*
 *
 *  * Copyright 2021 KubeClipper Authors.
 *  *
 *  * Licensed under the Apache License, Version 2.0 (the "License");
 *  * you may not use this file except in compliance with the License.
 *  * You may obtain a copy of the License at
 *  *
 *  *     http://www.apache.org/licenses/LICENSE-2.0
 *  *
 *  * Unless required by applicable law or agreed to in writing, software
 *  * distributed under the License is distributed on an "AS IS" BASIS,
 *  * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  * See the License for the specific language governing permissions and
 *  * limitations under the License.
 *
 */

package v1

import (
	"k8s.io/client-go/rest"

	"github.com/kubeclipper/kubeclipper/pkg/client/clientrest"

	"github.com/kubeclipper/kubeclipper/pkg/client/clientset/versioned/scheme"

	corev1 "github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1"
)

var _ CoreV1Interface = (*CoreV1Client)(nil)

type CoreV1Interface interface {
	RESTClient() rest.Interface
	NodesGetter
	ClustersGetter
	RegionsGetter
	LeasesGetter
	OperationsGetter
	BackupsGetter
	BackupPointsGetter
	CronBackupsGetter
	DomainsGetter
}

type CoreV1Client struct {
	restClient rest.Interface
}

func New(c rest.Interface) CoreV1Interface {
	return &CoreV1Client{restClient: c}
}

func (c *CoreV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}

func (c *CoreV1Client) Nodes() NodesInterface {
	return newNodes(c)
}

func (c *CoreV1Client) Clusters() ClustersInterface {
	return newClusters(c)
}

func (c *CoreV1Client) Regions() RegionsInterface {
	return newRegions(c)
}

func (c *CoreV1Client) Leases() LeasesInterface {
	return newLeases(c)
}

func (c *CoreV1Client) Operations() OperationsInterface {
	return newOperations(c)
}

func (c *CoreV1Client) Backups() BackupsInterface {
	return newBackups(c)
}

func (c *CoreV1Client) BackupPoints() BackupPointsInterface {
	return newBackupPoints(c)
}

func (c *CoreV1Client) CronBackups() CronBackupsInterface {
	return newCronBackups(c)
}

func (c *CoreV1Client) Domains() DomainsInterface {
	return newDomains(c)
}

func NewForConfig(c *rest.Config) (*CoreV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := clientrest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &CoreV1Client{client}, nil
}

func setConfigDefaults(config *rest.Config) error {
	gv := corev1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/api"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	//if config.UserAgent == "" {
	//	config.UserAgent = rest.DefaultKubernetesUserAgent()
	//}

	return nil
}
