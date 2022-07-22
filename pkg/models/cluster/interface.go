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

//go:generate mockgen -destination mock/mock_cluster.go -source interface.go Operator

package cluster

import (
	"context"

	"k8s.io/apimachinery/pkg/watch"

	"github.com/kubeclipper/kubeclipper/pkg/models"
	"github.com/kubeclipper/kubeclipper/pkg/query"
	v1 "github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1"
)

type OperatorReader interface {
	ClusterReader
	RegionReader
	NodeReader
	BackupReader
	BackupPointReader
	CronBackupReader
	DNSReader
}

type OperatorWriter interface {
	ClusterWriter
	RegionWriter
	NodeWriter
	BackupWriter
	BackupPointWriter
	CronBackupWriter
	DNSWriter
}

type Operator interface {
	ClusterReader
	ClusterWriter

	RegionReader
	RegionWriter

	NodeReader
	NodeWriter

	BackupReader
	BackupWriter

	BackupPointReader
	BackupPointWriter

	CronBackupReader
	CronBackupWriter

	DNSReader
	DNSWriter

	TemplateReader
	TemplateWriter
}

type ClusterReader interface {
	ListClusters(ctx context.Context, query *query.Query) (*v1.ClusterList, error)
	WatchClusters(ctx context.Context, query *query.Query) (watch.Interface, error)
	GetCluster(ctx context.Context, name string) (*v1.Cluster, error)
	ClusterReaderEx
}

type ClusterReaderEx interface {
	GetClusterEx(ctx context.Context, name string, resourceVersion string) (*v1.Cluster, error)
	ListClusterEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error)
}

type ClusterWriter interface {
	CreateCluster(ctx context.Context, cluster *v1.Cluster) (*v1.Cluster, error)
	UpdateCluster(ctx context.Context, cluster *v1.Cluster) (*v1.Cluster, error)
	DeleteCluster(ctx context.Context, name string) error
}

type RegionReader interface {
	ListRegions(ctx context.Context, query *query.Query) (*v1.RegionList, error)
	GetRegion(ctx context.Context, name string) (*v1.Region, error)
	WatchRegions(ctx context.Context, query *query.Query) (watch.Interface, error)
	RegionReaderEx
}

type RegionReaderEx interface {
	ListRegionEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error)
	GetRegionEx(ctx context.Context, name string, resourceVersion string) (*v1.Region, error)
}

type RegionWriter interface {
	CreateRegion(ctx context.Context, region *v1.Region) (*v1.Region, error)
	DeleteRegion(ctx context.Context, name string) error
}

type NodeReader interface {
	ListNodes(ctx context.Context, query *query.Query) (*v1.NodeList, error)
	WatchNodes(ctx context.Context, query *query.Query) (watch.Interface, error)
	GetNode(ctx context.Context, name string) (*v1.Node, error)
	NodeReaderEx
}

type NodeReaderEx interface {
	GetNodeEx(ctx context.Context, name string, resourceVersion string) (*v1.Node, error)
	ListNodesEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error)
}

type NodeWriter interface {
	UpdateNode(ctx context.Context, node *v1.Node) (*v1.Node, error)
	CreateNode(ctx context.Context, node *v1.Node) (*v1.Node, error)
	DeleteNode(ctx context.Context, name string) error
}

type BackupReader interface {
	ListBackups(ctx context.Context, query *query.Query) (*v1.BackupList, error)
	WatchBackups(ctx context.Context, query *query.Query) (watch.Interface, error)
	GetBackup(ctx context.Context, cluster string, name string) (*v1.Backup, error)
	BackupReaderEx
}

type BackupReaderEx interface {
	ListBackupEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error)
	GetBackupEx(ctx context.Context, cluster string, name string) (*v1.Backup, error)
}

type BackupWriter interface {
	CreateBackup(ctc context.Context, backup *v1.Backup) (*v1.Backup, error)
	UpdateBackup(ctx context.Context, backup *v1.Backup) (*v1.Backup, error)
	DeleteBackup(ctx context.Context, name string) error
}

type RecoveryReader interface {
	ListRecoveries(ctx context.Context, query *query.Query) (*v1.RecoveryList, error)
	WatchRecoveries(ctx context.Context, query *query.Query) (watch.Interface, error)
	GetRecovery(ctx context.Context, cluster string, name string) (*v1.Recovery, error)
	RecoveryReaderEx
}

type RecoveryReaderEx interface {
	ListRecoveryEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error)
	GetRecoveryEx(ctx context.Context, cluster string, name string) (*v1.Recovery, error)
}

type RecoveryWriter interface {
	CreateRecovery(ctc context.Context, recovery *v1.Recovery) (*v1.Recovery, error)
	UpdateRecovery(ctx context.Context, recovery *v1.Recovery) (*v1.Recovery, error)
	DeleteRecovery(ctx context.Context, name string) error
}

type BackupPointReaderEx interface {
	ListBackupPointEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error)
	GetBackupPointEx(ctx context.Context, name string, resourceVersion string) (*v1.BackupPoint, error)
}

type BackupPointReader interface {
	ListBackupPoints(ctx context.Context, query *query.Query) (*v1.BackupPointList, error)
	GetBackupPoint(ctx context.Context, name string, resourceVersion string) (*v1.BackupPoint, error)
	BackupPointReaderEx
}

type BackupPointWriter interface {
	CreateBackupPoint(ctx context.Context, backupPoint *v1.BackupPoint) (*v1.BackupPoint, error)
	UpdateBackupPoint(ctx context.Context, backupPoint *v1.BackupPoint) (*v1.BackupPoint, error)
	DeleteBackupPoint(ctx context.Context, name string) error
}

type CronBackupReaderEx interface {
	ListCronBackupEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error)
	GetCronBackupEx(ctx context.Context, name string, resourceVersion string) (*v1.CronBackup, error)
}

type CronBackupReader interface {
	ListCronBackups(ctx context.Context, query *query.Query) (*v1.CronBackupList, error)
	GetCronBackup(ctx context.Context, name string, resourceVersion string) (*v1.CronBackup, error)
	CronBackupReaderEx
}

type CronBackupWriter interface {
	CreateCronBackup(ctx context.Context, cronBackup *v1.CronBackup) (*v1.CronBackup, error)
	UpdateCronBackup(ctx context.Context, cronBackup *v1.CronBackup) (*v1.CronBackup, error)
	DeleteCronBackup(ctx context.Context, name string) error
}

type DNSReader interface {
	ListDomains(ctx context.Context, query *query.Query) (*v1.DomainList, error)
	GetDomain(ctx context.Context, name string) (*v1.Domain, error)
	WatchDomain(ctx context.Context, query *query.Query) (watch.Interface, error)
	DNSReaderEx
}
type DNSReaderEx interface {
	ListRecordsEx(ctx context.Context, name, subdomain string, query *query.Query) (*models.PageableResponse, error)
	ListDomainsEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error)
}

type DNSWriter interface {
	CreateDomain(ctc context.Context, domain *v1.Domain) (*v1.Domain, error)
	UpdateDomain(ctx context.Context, domain *v1.Domain) (*v1.Domain, error)
	DeleteDomain(ctx context.Context, name string) error
}

type TemplateReader interface {
	ListTemplates(ctx context.Context, query *query.Query) (*v1.TemplateList, error)
	WatchTemplates(ctx context.Context, query *query.Query) (watch.Interface, error)
	GetTemplate(ctx context.Context, name string) (*v1.Template, error)
	TemplateReaderEx
}

type TemplateReaderEx interface {
	ListTemplatesEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error)
	GetTemplateEx(ctx context.Context, name string, resourceVersion string) (*v1.Template, error)
}

type TemplateWriter interface {
	CreateTemplate(ctc context.Context, template *v1.Template) (*v1.Template, error)
	UpdateTemplate(ctx context.Context, template *v1.Template) (*v1.Template, error)
	DeleteTemplate(ctx context.Context, name string) error
	DeleteTemplateCollection(ctx context.Context, query *query.Query) error
}
