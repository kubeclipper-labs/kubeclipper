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
	"k8s.io/client-go/informers/internalinterfaces"

	"github.com/kubeclipper/kubeclipper/pkg/client/internal"
)

type Interface interface {
	Clusters() ClusterInformer
	Nodes() NodeInformer
	Operations() OperationInformer
	Regions() RegionInformer
	Backups() BackupInformer
	BackupPoints() BackupPointInformer
	CronBackups() CronBackupInformer
	Leases() LeaseInformer
	Domains() DomainInformer
}

type version struct {
	factory          internal.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internal.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

func (v *version) Clusters() ClusterInformer {
	return &clusterInformer{
		factory:          v.factory,
		tweakListOptions: v.tweakListOptions,
	}
}

func (v *version) Nodes() NodeInformer {
	return &nodeInformer{
		factory:          v.factory,
		tweakListOptions: v.tweakListOptions,
	}
}

func (v *version) Operations() OperationInformer {
	return &operationInformer{
		factory:          v.factory,
		tweakListOptions: v.tweakListOptions,
	}
}

func (v *version) Regions() RegionInformer {
	return &regionInformer{
		factory:          v.factory,
		tweakListOptions: v.tweakListOptions,
	}
}

func (v *version) Backups() BackupInformer {
	return &backupInformer{
		factory:          v.factory,
		tweakListOptions: v.tweakListOptions,
	}
}

func (v *version) BackupPoints() BackupPointInformer {
	return &backupPointInformer{
		factory:          v.factory,
		tweakListOptions: v.tweakListOptions,
	}
}

func (v *version) CronBackups() CronBackupInformer {
	return &cronBackupInformer{
		factory:          v.factory,
		tweakListOptions: v.tweakListOptions,
	}
}

func (v *version) Leases() LeaseInformer {
	return &leaseInformer{
		factory:          v.factory,
		tweakListOptions: v.tweakListOptions,
	}
}

func (v *version) Domains() DomainInformer {
	return &domainInformer{
		factory:          v.factory,
		tweakListOptions: v.tweakListOptions,
	}
}
