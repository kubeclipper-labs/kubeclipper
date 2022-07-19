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

package registry

import (
	"reflect"
	"sync"

	"github.com/kubeclipper/kubeclipper/pkg/server/registry/cronbackup"

	"github.com/kubeclipper/kubeclipper/pkg/server/registry/template"

	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"

	iamv1 "github.com/kubeclipper/kubeclipper/pkg/scheme/iam/v1"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/domain"

	"github.com/kubeclipper/kubeclipper/pkg/scheme"
	corev1 "github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/backup"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/backuppoint"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/cluster"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/event"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/globalrole"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/globalrolebinding"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/lease"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/loginrecord"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/node"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/operation"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/platformsetting"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/recovery"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/region"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/token"
	"github.com/kubeclipper/kubeclipper/pkg/server/registry/user"
)

type NewStorageFunc func(scheme *runtime.Scheme, restOptionsGetter generic.RESTOptionsGetter) (rest.StandardStorage, error)

type SharedStorageFactory interface {
	Clusters() rest.StandardStorage
	GlobalRoles() rest.StandardStorage
	GlobalRoleBindings() rest.StandardStorage
	Leases() rest.StandardStorage
	Nodes() rest.StandardStorage
	Operations() rest.StandardStorage
	Regions() rest.StandardStorage
	Tokens() rest.StandardStorage
	Users() rest.StandardStorage
	LoginRecords() rest.StandardStorage
	PlatformSettings() rest.StandardStorage
	Events() rest.StandardStorage
	Backups() rest.StandardStorage
	Recoveries() rest.StandardStorage
	BackupPoints() rest.StandardStorage
	CronBackups() rest.StandardStorage
	DNSDomains() rest.StandardStorage
	Template() rest.StandardStorage
}

var _ SharedStorageFactory = (*sharedStorageFactory)(nil)

type sharedStorageFactory struct {
	lock              sync.Mutex
	restOptionsGetter generic.RESTOptionsGetter
	storages          map[reflect.Type]rest.StandardStorage
}

func NewSharedStorageFactory(optsGetter generic.RESTOptionsGetter) SharedStorageFactory {
	factory := &sharedStorageFactory{
		restOptionsGetter: optsGetter,
		storages:          make(map[reflect.Type]rest.StandardStorage),
	}
	return factory
}

func (s *sharedStorageFactory) StorageFor(obj interface{}, storageFunc NewStorageFunc) rest.StandardStorage {
	s.lock.Lock()
	defer s.lock.Unlock()

	storageType := reflect.TypeOf(obj)
	storage, exist := s.storages[storageType]
	if exist {
		return storage
	}
	// panic when Storage create error occurred
	storage, err := storageFunc(scheme.Scheme, s.restOptionsGetter)
	if err != nil {
		panic(err)
	}
	s.storages[storageType] = storage
	return storage
}

func (s *sharedStorageFactory) Clusters() rest.StandardStorage {
	return s.StorageFor(&corev1.Cluster{}, cluster.NewStorage)
}

func (s *sharedStorageFactory) GlobalRoles() rest.StandardStorage {
	return s.StorageFor(&iamv1.GlobalRole{}, globalrole.NewStorage)
}

func (s *sharedStorageFactory) GlobalRoleBindings() rest.StandardStorage {
	return s.StorageFor(&iamv1.GlobalRoleBinding{}, globalrolebinding.NewStorage)
}

func (s *sharedStorageFactory) Leases() rest.StandardStorage {
	return s.StorageFor(&coordinationv1.Lease{}, lease.NewStorage)
}

func (s *sharedStorageFactory) Nodes() rest.StandardStorage {
	return s.StorageFor(&corev1.Node{}, node.NewStorage)
}

func (s *sharedStorageFactory) Operations() rest.StandardStorage {
	return s.StorageFor(&corev1.Operation{}, operation.NewStorage)
}

func (s *sharedStorageFactory) Regions() rest.StandardStorage {
	return s.StorageFor(&corev1.Region{}, region.NewStorage)
}

func (s *sharedStorageFactory) Tokens() rest.StandardStorage {
	return s.StorageFor(&iamv1.Token{}, token.NewStorage)
}

func (s *sharedStorageFactory) Users() rest.StandardStorage {
	return s.StorageFor(&iamv1.User{}, user.NewStorage)
}

func (s *sharedStorageFactory) LoginRecords() rest.StandardStorage {
	return s.StorageFor(&iamv1.LoginRecord{}, loginrecord.NewStorage)
}

func (s *sharedStorageFactory) PlatformSettings() rest.StandardStorage {
	return s.StorageFor(&corev1.PlatformSetting{}, platformsetting.NewStorage)
}

func (s *sharedStorageFactory) Events() rest.StandardStorage {
	return s.StorageFor(&corev1.Event{}, event.NewStorage)
}

func (s *sharedStorageFactory) Backups() rest.StandardStorage {
	return s.StorageFor(&corev1.Backup{}, backup.NewStorage)
}

func (s *sharedStorageFactory) Recoveries() rest.StandardStorage {
	return s.StorageFor(&corev1.Recovery{}, recovery.NewStorage)
}

func (s *sharedStorageFactory) BackupPoints() rest.StandardStorage {
	return s.StorageFor(&corev1.BackupPoint{}, backuppoint.NewStorage)
}

func (s *sharedStorageFactory) CronBackups() rest.StandardStorage {
	return s.StorageFor(&corev1.CronBackup{}, cronbackup.NewStorage)
}

func (s *sharedStorageFactory) DNSDomains() rest.StandardStorage {
	return s.StorageFor(&corev1.Domain{}, domain.NewStorage)
}

func (s *sharedStorageFactory) Template() rest.StandardStorage {
	return s.StorageFor(&corev1.Template{}, template.NewStorage)
}
