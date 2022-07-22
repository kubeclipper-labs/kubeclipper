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
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/kubeclipper/kubeclipper/pkg/component"
	"github.com/kubeclipper/kubeclipper/pkg/component/utils"
	"github.com/kubeclipper/kubeclipper/pkg/query"
	"github.com/kubeclipper/kubeclipper/pkg/scheme/common"
	v1 "github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1"
	"github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1/cri"
	"github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1/k8s"
	bs "github.com/kubeclipper/kubeclipper/pkg/simple/backupstore"
)

/*func reverseComponents(components []v1.Component) {
	length := len(components)
	for i := 0; i < length/2; i++ {
		components[i], components[length-(i+1)] = components[length-(i+1)], components[i]
	}
}*/

func getCriStep(ctx context.Context, c *v1.ContainerRuntime, action v1.StepAction, nodes []v1.StepNode) ([]v1.Step, error) {
	switch c.Type {
	case v1.CRIDocker:
		r := cri.DockerRunnable{}
		err := r.InitStep(ctx, &c.Docker, nodes)
		if err != nil {
			return nil, err
		}
		return r.GetActionSteps(action), nil
	case v1.CRIContainerd:
		r := cri.ContainerdRunnable{}
		err := r.InitStep(ctx, &c.Containerd, nodes)
		if err != nil {
			return nil, err
		}
		return r.GetActionSteps(action), nil
	}
	return nil, fmt.Errorf("no support %v type cri", c.Type)
}

func getK8sSteps(ctx context.Context, c *v1.Cluster, action v1.StepAction) ([]v1.Step, error) {
	runnable := k8s.KubeadmRunnable(*c.Kubeadm)

	return runnable.GetStep(ctx, action)
}

func getSteps(c component.Interface, action v1.StepAction) ([]v1.Step, error) {
	switch action {
	case v1.ActionInstall:
		return c.GetInstallSteps(), nil
	case v1.ActionUninstall:
		return c.GetUninstallSteps(), nil
	case v1.ActionUpgrade:
		return c.GetUpgradeSteps(), nil
	default:
		return nil, fmt.Errorf("unsupported step action %s", action)
	}
}

func (h *handler) parseOperationFromCluster(extraMetadata *component.ExtraMetadata, c *v1.Cluster, action v1.StepAction) (*v1.Operation, error) {
	var steps []v1.Step
	region := extraMetadata.Masters[0].Region
	if c.Labels == nil {
		c.Labels = map[string]string{
			common.LabelTopologyRegion: region,
		}
	}
	op := &v1.Operation{}
	op.Name = uuid.New().String()
	op.Labels = map[string]string{
		common.LabelClusterName:    c.Name,
		common.LabelTopologyRegion: region,
	}

	// Container runtime should be installed on all nodes.
	ctx := component.WithExtraMetadata(context.TODO(), *extraMetadata)
	stepNodes := utils.UnwrapNodeList(extraMetadata.GetAllNodes())
	cSteps, err := getCriStep(ctx, &c.Kubeadm.ContainerRuntime, action, stepNodes)
	if err != nil {
		return nil, err
	}

	// kubernetes
	k8sSteps, err := getK8sSteps(ctx, c, action)
	if err != nil {
		return nil, err
	}

	carr := make([]v1.Component, len(c.Kubeadm.Components))
	if action == v1.ActionUninstall {
		// do not need to delete the component logic when deleting a cluster, set carr nil
		// reverse order of the components
		// reverseComponents(carr)
	} else {
		copy(carr, c.Kubeadm.Components)
		steps = append(steps, cSteps...)
		steps = append(steps, k8sSteps...)
	}

	for _, com := range carr {
		comp, ok := component.Load(fmt.Sprintf(component.RegisterFormat, com.Name, com.Version))
		if !ok {
			continue
		}
		compMeta := comp.NewInstance()
		if err := json.Unmarshal(com.Config.Raw, compMeta); err != nil {
			return nil, err
		}
		newComp, _ := compMeta.(component.Interface)
		// worker around  delete cluster
		if action == v1.ActionInstall {
			if err := h.initComponentExtraCluster(ctx, newComp); err != nil {
				return nil, err
			}
		}
		if err := newComp.Validate(); err != nil {
			return nil, err
		}
		if err := newComp.InitSteps(ctx); err != nil {
			return nil, err
		}
		s, err := getSteps(newComp, action)
		if err != nil {
			return nil, err
		}
		steps = append(steps, s...)
	}

	if action == v1.ActionUninstall {
		steps = append(steps, k8sSteps...)
		steps = append(steps, cSteps...)
	}

	op.Steps = steps
	return op, nil
}

func (h *handler) initComponentExtraCluster(ctx context.Context, p component.Interface) error {
	cluNames := p.RequireExtraCluster()
	extraClulsterMeta := make(map[string]component.ExtraMetadata, len(cluNames))
	for _, cluName := range cluNames {
		if clu, err := h.clusterOperator.GetClusterEx(ctx, cluName, "0"); err == nil {
			extra, err := h.getClusterMetadata(ctx, clu)
			if err != nil {
				return err
			}
			extraClulsterMeta[cluName] = *extra
		} else {
			return err
		}
	}
	return p.CompleteWithExtraCluster(extraClulsterMeta)
}

func (h *handler) parseRecoverySteps(c *v1.Cluster, b *v1.Backup, restoreDir string, action v1.StepAction) ([]v1.Step, error) {
	steps := make([]v1.Step, 0)

	q := query.New()
	q.LabelSelector = fmt.Sprintf("%s=%s", common.LabelClusterName, c.Name)
	nodeList, err := h.clusterOperator.ListNodes(context.TODO(), q)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)
	ips := make([]string, 0)
	for _, node := range nodeList.Items {
		if node.Labels[common.LabelNodeRole] != "master" {
			continue
		}
		names = append(names, node.Status.NodeInfo.Hostname)
		ips = append(ips, node.Status.Ipv4DefaultIP)
	}

	bp, err := h.clusterOperator.GetBackupPoint(context.TODO(), b.BackupPointName, "0")
	if err != nil {
		return nil, err
	}

	recoveryStep, err := getRecoveryStep(c, bp, b, restoreDir, names, ips, action)
	if err != nil {
		return nil, err
	}

	steps = append(steps, recoveryStep...)

	return steps, nil
}

func getRecoveryStep(c *v1.Cluster, bp *v1.BackupPoint, b *v1.Backup, restoreDir string, nodeNames, nodeIPs []string, action v1.StepAction) (steps []v1.Step, err error) {
	meta := component.ExtraMetadata{
		ClusterName: c.Name,
	}
	for _, node := range c.Kubeadm.Masters.GetNodeIDs() {
		meta.Masters = append(meta.Masters, component.Node{ID: node})
	}
	ctx := component.WithExtraMetadata(context.TODO(), meta)

	f := k8s.FileDir{
		EtcdDataDir:    c.Kubeadm.KubeComponents.Etcd.DataDir,
		ManifestsYaml:  k8s.KubeManifestsDir,
		TmpEtcdDataDir: filepath.Join(restoreDir, "etcd"),
		TmpStaticYaml:  filepath.Join(restoreDir, "manifests"),
	}

	r := k8s.Recovery{
		StoreType:      bp.StorageType,
		RestoreDir:     restoreDir,
		BackupFileName: b.FileName,
		NodeNameList:   nodeNames,
		NodeIPList:     nodeIPs,
		BackupFileSize: b.Status.BackupFileSize,
		BackupFileMD5:  b.Status.BackupFileMD5,
		FileDir:        f,
	}

	switch bp.StorageType {
	case bs.FSStorage:
		r.BackupPointRootDir = bp.FsConfig.BackupRootDir
	case bs.S3Storage:
		r.AccessKeyID = bp.S3Config.AccessKeyID
		r.AccessKeySecret = bp.S3Config.AccessKeySecret
		r.Bucket = bp.S3Config.Bucket
		r.Endpoint = bp.S3Config.Endpoint
	}

	err = r.InitSteps(ctx)
	if err != nil {
		return nil, err
	}

	switch action {
	case v1.ActionInstall:
		return r.GetInstallSteps(), nil
	}
	return nil, nil
}

// parseOperationFromComponent parse operation instance from component
func (h *handler) parseOperationFromComponent(extraMetadata *component.ExtraMetadata, components []v1.Component, c *v1.Cluster, action v1.StepAction) (*v1.Operation, error) {
	var steps []v1.Step
	op := &v1.Operation{}
	op.Name = uuid.New().String()
	op.Labels = map[string]string{
		common.LabelClusterName:    c.Name,
		common.LabelTopologyRegion: extraMetadata.Masters[0].Region,
	}
	// with extra meta data
	ctx := component.WithExtraMetadata(context.TODO(), *extraMetadata)
	for _, comp := range components {
		cInterface, ok := component.Load(fmt.Sprintf(component.RegisterFormat, comp.Name, comp.Version))
		if !ok {
			continue
		}
		instance := cInterface.NewInstance()
		if err := json.Unmarshal(comp.Config.Raw, instance); err != nil {
			return nil, err
		}
		newComp, ok := instance.(component.Interface)
		if !ok {
			continue
		}
		if err := newComp.Validate(); err != nil {
			return nil, err
		}
		if err := newComp.InitSteps(ctx); err != nil {
			return nil, err
		}
		s, err := getSteps(newComp, action)
		if err != nil {
			return nil, err
		}
		steps = append(steps, s...)
	}

	op.Steps = steps
	return op, nil
}

func (h *handler) parseActBackupSteps(c *v1.Cluster, b *v1.Backup, action v1.StepAction) ([]v1.Step, error) {
	steps := make([]v1.Step, 0)

	q := query.New()
	q.LabelSelector = fmt.Sprintf("%s=%s", common.LabelClusterName, c.Name)

	bp, err := h.clusterOperator.GetBackupPointEx(context.TODO(), c.Labels[common.LabelBackupPoint], "0")
	if err != nil {
		return nil, err
	}
	// obtain the preferred node information
	pNode, err := h.clusterOperator.GetNodeEx(context.TODO(), b.PreferredNode, "0")
	if err != nil {
		return nil, err
	}
	actBackupStep, err := getActBackupStep(c, b, bp, pNode, action)
	if err != nil {
		return nil, err
	}

	steps = append(steps, actBackupStep...)

	return steps, nil
}

func getActBackupStep(c *v1.Cluster, b *v1.Backup, bp *v1.BackupPoint, pNode *v1.Node, action v1.StepAction) (steps []v1.Step, err error) {
	var actBackup *k8s.ActBackup
	meta := component.ExtraMetadata{
		ClusterName: c.Name,
	}
	meta.Masters = []component.Node{{
		ID:       b.PreferredNode, // the preferred node is used by default
		IPv4:     pNode.Status.Ipv4DefaultIP,
		Hostname: pNode.Status.NodeInfo.Hostname,
	}}
	ctx := component.WithExtraMetadata(context.TODO(), meta)

	switch bp.StorageType {
	case bs.S3Storage:
		actBackup = &k8s.ActBackup{
			StoreType:       bp.StorageType,
			BackupFileName:  b.FileName,
			AccessKeyID:     bp.S3Config.AccessKeyID,
			AccessKeySecret: bp.S3Config.AccessKeySecret,
			Bucket:          bp.S3Config.Bucket,
			Endpoint:        bp.S3Config.Endpoint,
		}
	case bs.FSStorage:
		actBackup = &k8s.ActBackup{
			StoreType:          bp.StorageType,
			BackupFileName:     b.FileName,
			BackupPointRootDir: bp.FsConfig.BackupRootDir,
		}
	}

	if err = actBackup.InitSteps(ctx); err != nil {
		return
	}

	return actBackup.GetStep(action), nil
}

func (h *handler) getBackupPoint(backups *v1.BackupList, name string) bool {
	for _, item := range backups.Items {
		if item.BackupPointName == name {
			return true
		}
	}
	return false
}
