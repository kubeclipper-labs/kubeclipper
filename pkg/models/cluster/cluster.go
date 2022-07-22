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

package cluster

import (
	"context"
	"fmt"
	"sort"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"

	"github.com/kubeclipper/kubeclipper/pkg/scheme/common"

	apimachineryErrors "k8s.io/apimachinery/pkg/api/errors"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/kubeclipper/kubeclipper/pkg/logger"
	"github.com/kubeclipper/kubeclipper/pkg/models"
	"github.com/kubeclipper/kubeclipper/pkg/query"
	v1 "github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1"
)

var _ Operator = (*clusterOperator)(nil)

type clusterOperator struct {
	clusterStorage     rest.StandardStorage
	nodeStorage        rest.StandardStorage
	regionStorage      rest.StandardStorage
	backupStorage      rest.StandardStorage
	recoveryStorage    rest.StandardStorage
	backupPointStorage rest.StandardStorage
	cronBackupStorage  rest.StandardStorage
	upgradeStorage     rest.StandardStorage
	dnsStorage         rest.StandardStorage
	templateStorage    rest.StandardStorage
}

func NewClusterOperator(clusterStorage rest.StandardStorage, nodeStorage rest.StandardStorage,
	regionStorage rest.StandardStorage, backupStorage rest.StandardStorage, recoveryStorage, backupPointStorage,
	cronBackupStorage rest.StandardStorage, dnsStorage rest.StandardStorage, templateStorage rest.StandardStorage) Operator {
	return &clusterOperator{
		clusterStorage:     clusterStorage,
		nodeStorage:        nodeStorage,
		regionStorage:      regionStorage,
		backupStorage:      backupStorage,
		recoveryStorage:    recoveryStorage,
		backupPointStorage: backupPointStorage,
		cronBackupStorage:  cronBackupStorage,
		dnsStorage:         dnsStorage,
		templateStorage:    templateStorage,
	}
}

func (c *clusterOperator) UpdateCluster(ctx context.Context, cluster *v1.Cluster) (*v1.Cluster, error) {
	obj, wasCreated, err := c.clusterStorage.Update(ctx, cluster.Name, rest.DefaultUpdatedObjectInfo(cluster),
		nil, nil, false, &metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	if wasCreated {
		logger.Debug("cluster not exist, use create instead of update", zap.String("cluster", cluster.Name))
	}
	return obj.(*v1.Cluster), nil
}

func (c *clusterOperator) UpdateNode(ctx context.Context, node *v1.Node) (*v1.Node, error) {
	obj, wasCreated, err := c.nodeStorage.Update(ctx, node.Name, rest.DefaultUpdatedObjectInfo(node),
		nil, nil, false, &metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	if wasCreated {
		logger.Debug("node not exist, use create instead of update", zap.String("node_id", node.Name))
	}
	return obj.(*v1.Node), nil
}

func (c *clusterOperator) ListClusters(ctx context.Context, query *query.Query) (*v1.ClusterList, error) {
	list, err := models.List(ctx, c.clusterStorage, query)
	if err != nil {
		return nil, err
	}
	list.GetObjectKind().SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("ClusterList"))
	return list.(*v1.ClusterList), nil
}

func (c *clusterOperator) ListClusterEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error) {
	return models.ListExV2(ctx, c.clusterStorage, query, c.clusterFuzzyFilter, nil, nil)
}

func (c *clusterOperator) CreateCluster(ctx context.Context, cluster *v1.Cluster) (*v1.Cluster, error) {
	obj, err := c.clusterStorage.Create(ctx, cluster, nil, &metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Cluster), nil
}

func (c *clusterOperator) GetCluster(ctx context.Context, name string) (*v1.Cluster, error) {
	return c.GetClusterEx(ctx, name, "")
}

func (c *clusterOperator) GetClusterEx(ctx context.Context, name string, resourceVersion string) (*v1.Cluster, error) {
	cluster, err := models.Get(ctx, c.clusterStorage, name, resourceVersion)
	if err != nil {
		return nil, err
	}
	return cluster.(*v1.Cluster), nil
}

func (c *clusterOperator) DeleteCluster(ctx context.Context, name string) error {
	var err error
	_, _, err = c.clusterStorage.Delete(ctx, name, func(ctx context.Context, obj runtime.Object) error {
		return nil
	}, &metav1.DeleteOptions{})
	return err
}

func (c *clusterOperator) WatchClusters(ctx context.Context, query *query.Query) (watch.Interface, error) {
	return models.Watch(ctx, c.clusterStorage, query)
}

func (c *clusterOperator) WatchNodes(ctx context.Context, query *query.Query) (watch.Interface, error) {
	return models.Watch(ctx, c.nodeStorage, query)
}

func (c *clusterOperator) ListNodes(ctx context.Context, query *query.Query) (*v1.NodeList, error) {
	list, err := models.List(ctx, c.nodeStorage, query)
	if err != nil {
		return nil, err
	}
	list.GetObjectKind().SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("NodeList"))
	return list.(*v1.NodeList), nil
}

func (c *clusterOperator) ListNodesEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error) {
	return models.ListExV2(ctx, c.nodeStorage, query, c.nodeFuzzyFilter, nil, nil)
}

func (c *clusterOperator) GetNode(ctx context.Context, name string) (*v1.Node, error) {
	return c.GetNodeEx(ctx, name, "")
}

func (c *clusterOperator) GetNodeEx(ctx context.Context, name string, resourceVersion string) (*v1.Node, error) {
	node, err := models.Get(ctx, c.nodeStorage, name, resourceVersion)
	if err != nil {
		return nil, err
	}
	return node.(*v1.Node), nil
}

func (c *clusterOperator) DeleteNode(ctx context.Context, name string) error {
	_, _, err := c.nodeStorage.Delete(ctx, name, func(ctx context.Context, obj runtime.Object) error {
		return nil
	}, &metav1.DeleteOptions{})
	return err
}

func (c *clusterOperator) CreateNode(ctx context.Context, node *v1.Node) (*v1.Node, error) {
	obj, err := c.nodeStorage.Create(ctx, node, nil, &metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Node), nil
}

func (c *clusterOperator) ListRegions(ctx context.Context, query *query.Query) (*v1.RegionList, error) {
	list, err := models.List(ctx, c.regionStorage, query)
	if err != nil {
		return nil, err
	}
	list.GetObjectKind().SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("RegionList"))
	return list.(*v1.RegionList), nil
}

func (c *clusterOperator) GetRegion(ctx context.Context, name string) (*v1.Region, error) {
	return c.GetRegionEx(ctx, name, "")
}

func (c *clusterOperator) ListRegionEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error) {
	return models.ListExV2(ctx, c.regionStorage, query, c.regionFuzzyFilter, nil, nil)
}

func (c *clusterOperator) GetRegionEx(ctx context.Context, name string, resourceVersion string) (*v1.Region, error) {
	region, err := models.Get(ctx, c.regionStorage, name, resourceVersion)
	if err != nil {
		return nil, err
	}
	return region.(*v1.Region), nil
}

func (c *clusterOperator) WatchRegions(ctx context.Context, query *query.Query) (watch.Interface, error) {
	return models.Watch(ctx, c.regionStorage, query)
}

func (c *clusterOperator) CreateRegion(ctx context.Context, region *v1.Region) (*v1.Region, error) {
	obj, err := c.regionStorage.Create(ctx, region, nil, &metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Region), nil
}

func (c *clusterOperator) DeleteRegion(ctx context.Context, name string) error {
	_, _, err := c.regionStorage.Delete(ctx, name, func(ctx context.Context, obj runtime.Object) error {
		return nil
	}, &metav1.DeleteOptions{})
	return err
}

func (c *clusterOperator) CreateBackup(ctx context.Context, backup *v1.Backup) (*v1.Backup, error) {
	obj, err := c.backupStorage.Create(ctx, backup, nil, &metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Backup), nil
}

// TODO: refactor needed
func (c *clusterOperator) GetBackup(ctx context.Context, cluster string, name string) (*v1.Backup, error) {
	q := query.New()
	q.LabelSelector = fmt.Sprintf("%s=%s", common.LabelClusterName, cluster)

	backups, err := c.ListBackups(ctx, q)
	if err != nil {
		return nil, err
	}
	for _, val := range backups.Items {
		if val.Name == name {
			return &val, nil
		}
	}

	return nil, apimachineryErrors.NewNotFound(v1.Resource("backup"), name)
}

func (c *clusterOperator) GetBackupEx(ctx context.Context, cluster string, name string) (*v1.Backup, error) {
	return c.GetBackup(ctx, cluster, name)
}

func (c *clusterOperator) ListBackups(ctx context.Context, query *query.Query) (*v1.BackupList, error) {
	list, err := models.List(ctx, c.backupStorage, query)
	if err != nil {
		return nil, err
	}
	list.GetObjectKind().SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("BackupList"))
	return list.(*v1.BackupList), nil
}

func (c *clusterOperator) WatchBackups(ctx context.Context, query *query.Query) (watch.Interface, error) {
	return models.Watch(ctx, c.backupStorage, query)
}

func (c *clusterOperator) ListBackupEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error) {
	return models.ListExV2(ctx, c.backupStorage, query, c.backupFuzzyFilter, nil, nil)
}

func (c *clusterOperator) UpdateBackup(ctx context.Context, backup *v1.Backup) (*v1.Backup, error) {
	obj, existed, err := c.backupStorage.Update(ctx, backup.Name, rest.DefaultUpdatedObjectInfo(backup),
		nil, nil, false, &metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	if existed {
		logger.Debug("backup not exist, use create instead of update", zap.String("backup", backup.Name))
	}
	return obj.(*v1.Backup), nil
}

func (c *clusterOperator) DeleteBackup(ctx context.Context, name string) (err error) {
	_, _, err = c.backupStorage.Delete(ctx, name, func(ctx context.Context, obj runtime.Object) error {
		return nil
	}, &metav1.DeleteOptions{})
	return
}

func (c *clusterOperator) GetBackupPoint(ctx context.Context, name string, resourceVersion string) (*v1.BackupPoint, error) {
	return c.GetBackupPointEx(ctx, name, resourceVersion)
}

func (c *clusterOperator) GetBackupPointEx(ctx context.Context, name string, resourceVersion string) (*v1.BackupPoint, error) {
	backupPoint, err := models.Get(ctx, c.backupPointStorage, name, resourceVersion)
	if err != nil {
		return nil, err
	}
	return backupPoint.(*v1.BackupPoint), nil
}

func (c *clusterOperator) ListBackupPoints(ctx context.Context, query *query.Query) (*v1.BackupPointList, error) {
	list, err := models.List(ctx, c.backupPointStorage, query)
	if err != nil {
		return nil, err
	}
	list.GetObjectKind().SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("BackupPointList"))
	return list.(*v1.BackupPointList), nil
}

func (c *clusterOperator) ListBackupPointEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error) {
	return models.ListExV2(ctx, c.backupPointStorage, query, c.backupPointFuzzyFilter, nil, nil)
}

func (c *clusterOperator) CreateBackupPoint(ctx context.Context, backupPoint *v1.BackupPoint) (*v1.BackupPoint, error) {
	obj, err := c.backupPointStorage.Create(ctx, backupPoint, nil, &metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*v1.BackupPoint), nil
}

func (c *clusterOperator) UpdateBackupPoint(ctx context.Context, backupPoint *v1.BackupPoint) (*v1.BackupPoint, error) {
	obj, existed, err := c.backupPointStorage.Update(ctx, backupPoint.Name, rest.DefaultUpdatedObjectInfo(backupPoint),
		nil, nil, false, &metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	if !existed {
		logger.Debug("backuppoint not exist, use create instead of update", zap.String("backupPoint", backupPoint.Name))
	}
	return obj.(*v1.BackupPoint).DeepCopy(), nil
}

func (c *clusterOperator) DeleteBackupPoint(ctx context.Context, name string) error {
	_, _, err := c.backupPointStorage.Delete(ctx, name, func(ctx context.Context, obj runtime.Object) error {
		return nil
	}, &metav1.DeleteOptions{})
	return err
}

func (c *clusterOperator) GetCronBackup(ctx context.Context, name string, resourceVersion string) (*v1.CronBackup, error) {
	return c.GetCronBackupEx(ctx, name, resourceVersion)
}

func (c *clusterOperator) GetCronBackupEx(ctx context.Context, name string, resourceVersion string) (*v1.CronBackup, error) {
	cronBackup, err := models.Get(ctx, c.cronBackupStorage, name, resourceVersion)
	if err != nil {
		return nil, err
	}
	return cronBackup.(*v1.CronBackup), nil
}

func (c *clusterOperator) ListCronBackups(ctx context.Context, query *query.Query) (*v1.CronBackupList, error) {
	list, err := models.List(ctx, c.cronBackupStorage, query)
	if err != nil {
		return nil, err
	}
	list.GetObjectKind().SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("CronBackupList"))
	return list.(*v1.CronBackupList), nil
}

func (c *clusterOperator) ListCronBackupEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error) {
	return models.ListExV2(ctx, c.cronBackupStorage, query, c.cronBackupFuzzyFilter, nil, nil)
}

func (c *clusterOperator) CreateCronBackup(ctx context.Context, cronBackup *v1.CronBackup) (*v1.CronBackup, error) {
	obj, err := c.cronBackupStorage.Create(ctx, cronBackup, nil, &metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*v1.CronBackup), nil
}

func (c *clusterOperator) UpdateCronBackup(ctx context.Context, cronBackup *v1.CronBackup) (*v1.CronBackup, error) {
	obj, existed, err := c.cronBackupStorage.Update(ctx, cronBackup.Name, rest.DefaultUpdatedObjectInfo(cronBackup),
		nil, nil, false, &metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	if !existed {
		logger.Debug("cronbackup not exist, use create instead of update", zap.String("cronBackup", cronBackup.Name))
	}
	return obj.(*v1.CronBackup).DeepCopy(), nil
}

func (c *clusterOperator) DeleteCronBackup(ctx context.Context, name string) error {
	_, _, err := c.cronBackupStorage.Delete(ctx, name, func(ctx context.Context, obj runtime.Object) error {
		return nil
	}, &metav1.DeleteOptions{})
	return err
}

// dns 相关

func (c *clusterOperator) ListDomains(ctx context.Context, query *query.Query) (*v1.DomainList, error) {
	list, err := models.List(ctx, c.dnsStorage, query)
	if err != nil {
		return nil, err
	}
	list.GetObjectKind().SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("DomainList"))
	return list.(*v1.DomainList), nil
}

func (c *clusterOperator) WatchDomain(ctx context.Context, query *query.Query) (watch.Interface, error) {
	return models.Watch(ctx, c.dnsStorage, query)
}

func (c *clusterOperator) ListDomainsEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error) {
	return models.ListExV2(ctx, c.dnsStorage, query, c.dnsFuzzyFilter, nil, nil)
}

func (c *clusterOperator) GetDomain(ctx context.Context, name string) (*v1.Domain, error) {
	return c.GetDomainEx(ctx, name, "")
}

func (c *clusterOperator) GetDomainEx(ctx context.Context, name string, resourceVersion string) (*v1.Domain, error) {
	domain, err := models.Get(ctx, c.dnsStorage, name, resourceVersion)
	if err != nil {
		return nil, err
	}
	return domain.(*v1.Domain), nil
}

func (c *clusterOperator) CreateDomain(ctx context.Context, domain *v1.Domain) (*v1.Domain, error) {
	obj, err := c.dnsStorage.Create(ctx, domain, nil, &metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Domain), nil
}

func (c *clusterOperator) UpdateDomain(ctx context.Context, domain *v1.Domain) (*v1.Domain, error) {
	obj, creating, err := c.dnsStorage.Update(ctx, domain.Name, rest.DefaultUpdatedObjectInfo(domain),
		nil, nil, false, &metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	if creating {
		logger.Debug("domain not exist, use create instead of update", zap.String("domain", domain.Name))
	}
	return obj.(*v1.Domain), nil
}

func (c *clusterOperator) DeleteDomain(ctx context.Context, name string) error {
	_, _, err := c.dnsStorage.Delete(ctx, name, func(ctx context.Context, obj runtime.Object) error {
		return nil
	}, &metav1.DeleteOptions{})
	return err
}

//  records

func (c *clusterOperator) ListRecordsEx(ctx context.Context, name, subdomain string, query *query.Query) (*models.PageableResponse, error) {
	getDomain, err := c.GetDomainEx(ctx, name, "0")
	if err != nil {
		return nil, err
	}

	var records []v1.Record
	if subdomain != "" { // filter by subdomain
		record, ok := getDomain.Spec.Records[subdomain]
		if ok {
			records = append(records, record)
		}
	} else {
		for _, record := range getDomain.Spec.Records {
			records = append(records, record)
		}
	}

	if query.Reverse {
		sort.Sort(sort.Reverse(recordList(records)))
	} else {
		sort.Sort(recordList(records))
	}

	total := len(records)
	start, end := query.Pagination.GetValidPagination(total)
	items := make([]interface{}, end-start)
	for i := start; i < end; i++ {
		items[i-start] = records[i]
	}
	return &models.PageableResponse{
		Items:      items,
		TotalCount: total,
	}, nil
}

func (c *clusterOperator) clusterFuzzyFilter(obj runtime.Object, q *query.Query) []runtime.Object {
	clus, ok := obj.(*v1.ClusterList)
	if !ok {
		return nil
	}
	objs := make([]runtime.Object, 0, len(clus.Items))
	for index, clu := range clus.Items {
		selected := true
		for k, v := range q.FuzzySearch {
			if !models.ObjectMetaFilter(clu.ObjectMeta, k, v) {
				selected = false
			}
		}
		if selected {
			objs = append(objs, &clus.Items[index])
		}
	}
	return objs
}

func (c *clusterOperator) nodeFuzzyFilter(obj runtime.Object, q *query.Query) []runtime.Object {
	nodes, ok := obj.(*v1.NodeList)
	if !ok {
		return nil
	}
	objs := make([]runtime.Object, 0, len(nodes.Items))
	for index, node := range nodes.Items {
		selected := true
		for k, v := range q.FuzzySearch {
			if !models.ObjectMetaFilter(node.ObjectMeta, k, v) {
				selected = false
			}
		}
		if selected {
			objs = append(objs, &nodes.Items[index])
		}
	}
	return objs
}

func (c *clusterOperator) regionFuzzyFilter(obj runtime.Object, q *query.Query) []runtime.Object {
	regions, ok := obj.(*v1.RegionList)
	if !ok {
		return nil
	}
	objs := make([]runtime.Object, 0, len(regions.Items))
	for index, region := range regions.Items {
		selected := true
		for k, v := range q.FuzzySearch {
			if !models.ObjectMetaFilter(region.ObjectMeta, k, v) {
				selected = false
			}
		}
		if selected {
			objs = append(objs, &regions.Items[index])
		}
	}
	return objs
}

func (c *clusterOperator) dnsFuzzyFilter(obj runtime.Object, q *query.Query) []runtime.Object {
	domains, ok := obj.(*v1.DomainList)
	if !ok {
		return nil
	}
	objs := make([]runtime.Object, 0, len(domains.Items))
	for index, domain := range domains.Items {
		selected := true
		for k, v := range q.FuzzySearch {
			if !models.ObjectMetaFilter(domain.ObjectMeta, k, v) {
				selected = false
			}
		}
		if selected {
			objs = append(objs, &domains.Items[index])
		}
	}
	return objs
}

func (c *clusterOperator) backupFuzzyFilter(obj runtime.Object, q *query.Query) []runtime.Object {
	backups, ok := obj.(*v1.BackupList)
	if !ok {
		return nil
	}
	objs := make([]runtime.Object, 0, len(backups.Items))
	for index, backup := range backups.Items {
		selected := true
		for k, v := range q.FuzzySearch {
			if !models.ObjectMetaFilter(backup.ObjectMeta, k, v) {
				selected = false
			}
		}
		if selected {
			objs = append(objs, &backups.Items[index])
		}
	}
	return objs
}

func (c *clusterOperator) backupPointFuzzyFilter(obj runtime.Object, q *query.Query) []runtime.Object {
	backupPoints, ok := obj.(*v1.BackupPointList)
	if !ok {
		return nil
	}
	objs := make([]runtime.Object, 0, len(backupPoints.Items))
	for index, backup := range backupPoints.Items {
		selected := true
		for k, v := range q.FuzzySearch {
			if !models.ObjectMetaFilter(backup.ObjectMeta, k, v) {
				selected = false
			}
		}
		if selected {
			objs = append(objs, &backupPoints.Items[index])
		}
	}
	return objs
}

func (c *clusterOperator) cronBackupFuzzyFilter(obj runtime.Object, q *query.Query) []runtime.Object {
	cronBackups, ok := obj.(*v1.CronBackupList)
	if !ok {
		return nil
	}
	objs := make([]runtime.Object, 0, len(cronBackups.Items))
	for index, backup := range cronBackups.Items {
		selected := true
		for k, v := range q.FuzzySearch {
			if !models.ObjectMetaFilter(backup.ObjectMeta, k, v) {
				selected = false
			}
		}
		if selected {
			objs = append(objs, &cronBackups.Items[index])
		}
	}
	return objs
}

func (c *clusterOperator) ListTemplates(ctx context.Context, query *query.Query) (*v1.TemplateList, error) {
	list, err := models.List(ctx, c.templateStorage, query)
	if err != nil {
		return nil, err
	}
	list.GetObjectKind().SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("TemplateList"))
	return list.(*v1.TemplateList), nil
}

func (c *clusterOperator) WatchTemplates(ctx context.Context, query *query.Query) (watch.Interface, error) {
	return models.Watch(ctx, c.templateStorage, query)
}

func (c *clusterOperator) GetTemplate(ctx context.Context, name string) (*v1.Template, error) {
	return c.GetTemplateEx(ctx, name, "")
}

func (c *clusterOperator) ListTemplatesEx(ctx context.Context, query *query.Query) (*models.PageableResponse, error) {
	return models.ListExV2(ctx, c.templateStorage, query, c.templateFuzzyFilter, nil, nil)
}

func (c *clusterOperator) GetTemplateEx(ctx context.Context, name string, resourceVersion string) (*v1.Template, error) {
	obj, err := models.Get(ctx, c.templateStorage, name, resourceVersion)
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Template), nil
}

func (c *clusterOperator) CreateTemplate(ctx context.Context, template *v1.Template) (*v1.Template, error) {
	obj, err := c.templateStorage.Create(ctx, template, nil, &metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Template), nil
}

func (c *clusterOperator) UpdateTemplate(ctx context.Context, template *v1.Template) (*v1.Template, error) {
	obj, creating, err := c.templateStorage.Update(ctx, template.Name, rest.DefaultUpdatedObjectInfo(template),
		nil, nil, false, &metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	if creating {
		logger.Debug("template not exist, use create instead of update", zap.String("domain", template.Name))
	}
	return obj.(*v1.Template), nil
}

func (c *clusterOperator) DeleteTemplate(ctx context.Context, name string) error {
	_, _, err := c.templateStorage.Delete(ctx, name, func(ctx context.Context, obj runtime.Object) error {
		return nil
	}, &metav1.DeleteOptions{})
	return err
}

func (c *clusterOperator) DeleteTemplateCollection(ctx context.Context, query *query.Query) error {
	if _, err := c.templateStorage.DeleteCollection(ctx, func(ctx context.Context, obj runtime.Object) error {
		return nil
	}, &metav1.DeleteOptions{}, &metainternalversion.ListOptions{
		LabelSelector: query.GetLabelSelector(),
		FieldSelector: query.GetFieldSelector(),
	}); err != nil {
		return err
	}
	return nil
}

func (c *clusterOperator) templateFuzzyFilter(obj runtime.Object, q *query.Query) []runtime.Object {
	templates, ok := obj.(*v1.TemplateList)
	if !ok {
		return nil
	}
	objs := make([]runtime.Object, 0, len(templates.Items))
	for index, template := range templates.Items {
		selected := true
		for k, v := range q.FuzzySearch {
			if !models.ObjectMetaFilter(template.ObjectMeta, k, v) {
				selected = false
			}
		}
		if selected {
			objs = append(objs, &templates.Items[index])
		}
	}
	return objs
}
