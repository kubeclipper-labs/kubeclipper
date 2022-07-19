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

package cronbackupcontroller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	apimachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/kubeclipper/kubeclipper/pkg/client/informers"
	listerv1 "github.com/kubeclipper/kubeclipper/pkg/client/lister/core/v1"
	"github.com/kubeclipper/kubeclipper/pkg/component"
	ctrl "github.com/kubeclipper/kubeclipper/pkg/controller-runtime"
	"github.com/kubeclipper/kubeclipper/pkg/controller-runtime/controller"
	"github.com/kubeclipper/kubeclipper/pkg/controller-runtime/handler"
	"github.com/kubeclipper/kubeclipper/pkg/controller-runtime/manager"
	"github.com/kubeclipper/kubeclipper/pkg/controller-runtime/source"
	"github.com/kubeclipper/kubeclipper/pkg/errors"
	"github.com/kubeclipper/kubeclipper/pkg/logger"
	"github.com/kubeclipper/kubeclipper/pkg/models/cluster"
	"github.com/kubeclipper/kubeclipper/pkg/models/operation"
	"github.com/kubeclipper/kubeclipper/pkg/scheme/common"
	v1 "github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1"
	"github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1/k8s"
	"github.com/kubeclipper/kubeclipper/pkg/service"
	bs "github.com/kubeclipper/kubeclipper/pkg/simple/backupstore"
)

type CronBackupReconciler struct {
	ClusterWriter     cluster.ClusterWriter
	ClusterLister     listerv1.ClusterLister
	NodeLister        listerv1.NodeLister
	BackupLister      listerv1.BackupLister
	OperationWriter   operation.Writer
	BackupWriter      cluster.BackupWriter
	BackupPointLister listerv1.BackupPointLister
	CronBackupLister  listerv1.CronBackupLister
	CronBackupWriter  cluster.CronBackupWriter
	CmdDelivery       service.CmdDelivery
}

func (r *CronBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.FromContext(ctx)
	cronBackups, err := r.CronBackupLister.List(labels.Everything())
	if err != nil {
		log.Error("Failed to list cronBackups", zap.Error(err))
		return ctrl.Result{}, err
	}

	for _, cronBackup := range cronBackups {
		_, cErr := r.ClusterLister.Get(cronBackup.Labels[common.LabelClusterName])
		if cErr != nil && !errors.IsNotFound(cErr) {
			log.Warn("unexpected error, cronBackup should always has a cluster name label")
			return ctrl.Result{}, cErr
		}

		if cronBackup.Spec.RunAt != nil {
			// the disposable cronBackup is done
			if cronBackup.Status.LastSuccessfulTime == nil {
				if time.Now().After(cronBackup.Spec.RunAt.Time) {
					// delivery the create backup operation
					err = r.createBackup(log, cronBackup)
					if err != nil {
						log.Error("Failed to delivery operation to create backup", zap.Error(err))
						return ctrl.Result{}, err
					}
					cronBackup.Status.LastScheduleTime = cronBackup.Status.NextScheduleTime
					cronBackup.Status.LastSuccessfulTime = cronBackup.Status.LastScheduleTime
					_, err = r.CronBackupWriter.UpdateCronBackup(ctx, cronBackup)
					if err != nil {
						log.Error("Failed to update cronBackup", zap.Error(err))
					}
				}
			}
		}

		if cronBackup.Spec.Schedule != "" {
			// keep the backup num within the maxNum
			requirement, err := labels.NewRequirement(common.LabelCronBackupCreated, selection.Equals, []string{cronBackup.Name})
			if err != nil {
				log.Error("Failed to create a requirement", zap.Error(err))
				return ctrl.Result{}, err
			}
			backupList, err := r.BackupLister.List(labels.NewSelector().Add(*requirement))
			if err != nil {
				log.Error("Failed to list backups of the cronBackup", zap.Error(err))
				return ctrl.Result{}, err
			}
			if len(backupList) == cronBackup.Spec.MaxBackupNum {
				// delete redundant backups
				firstTime := &time.Time{}
				var firstBackupName string
				for i, ab := range backupList {
					if i == 0 || ab.CreationTimestamp.Time.Before(*firstTime) {
						firstTime = &ab.CreationTimestamp.Time
						firstBackupName = ab.Name
					}
				}
				err = r.BackupWriter.DeleteBackup(ctx, firstBackupName)
				if err != nil {
					log.Error("Failed to delete backup", zap.Error(err))
					return ctrl.Result{}, err
				}
			}
			// time to create backup
			if after := cronBackup.Status.NextScheduleTime.Time.After(time.Now()); after {
				// delivery the create backup operation
				err = r.createBackup(log, cronBackup)
				if err != nil {
					log.Error("Failed to delivery operation to create backup", zap.Error(err))
					return ctrl.Result{}, err
				}
				// update cronBackup status
				cronBackup.Status.LastScheduleTime = cronBackup.Status.NextScheduleTime
				cronBackup.Status.LastSuccessfulTime = cronBackup.Status.LastScheduleTime

				s, _ := cron.NewParser(4 | 8 | 16 | 32 | 64).Parse(cronBackup.Spec.Schedule)
				// update the next schedule time
				nextRunAt := metav1.NewTime(s.Next(time.Now()))
				cronBackup.Status.NextScheduleTime = &nextRunAt
				// update cronBackup
				_, err = r.CronBackupWriter.UpdateCronBackup(ctx, cronBackup)
				if err != nil {
					log.Error("Failed to update cronBackup", zap.Error(err))
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *CronBackupReconciler) SetupWithManager(mgr manager.Manager, cache informers.InformerCache) error {
	c, err := controller.NewUnmanaged("cronbackup", controller.Options{
		MaxConcurrentReconciles: 2,
		Reconciler:              r,
		Log:                     mgr.GetLogger().WithName("cronbackup-controller"),
		RecoverPanic:            true,
	})
	if err != nil {
		return err
	}
	if err = c.Watch(source.NewKindWithCache(&v1.CronBackup{}, cache), &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	mgr.AddRunnable(c)
	return nil
}

func (r *CronBackupReconciler) createBackup(log logger.Logging, cronBackup *v1.CronBackup) error {
	backup := &v1.Backup{}
	c, err := r.ClusterLister.Get(cronBackup.Labels[common.LabelClusterName])
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			log.Error("Cluster is not found", zap.Error(err))
			return err
		}
		log.Error("Failed to get cluster", zap.Error(err))
		return err
	}

	if c.Status.Status != v1.ClusterStatusRunning {
		log.Errorf("the cluster is %v, create backup in next reconcile", c.Status.Status)
		//return fmt.Errorf("the cluster is %v, create backup in next reconcile", c.Status.Status)
	}

	randNum := rand.String(5)
	backup.Name = fmt.Sprintf("%s-%s-%s", c.Name, cronBackup.Name, randNum)
	b, err := r.BackupLister.Get(backup.Name)
	if err != nil && !apimachineryErrors.IsNotFound(err) {
		log.Error("Failed to get backup", zap.Error(err))
		return err
	}
	if b != nil {
		log.Errorf("backup %s already exists", backup.Name)
		return fmt.Errorf("backup %s already exists", backup.Name)
	}

	// check the backup point exist or not
	if c.Labels[common.LabelBackupPoint] == "" {
		log.Error("no backup point specified")
		return fmt.Errorf("no backup point specified")
	}

	bp, err := r.BackupPointLister.Get(c.Labels[common.LabelBackupPoint])
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			log.Error("backupPoint is not found", zap.Error(err))
			return err
		}
		log.Error("Failed to get backupPoint", zap.Error(err))
		return err
	}

	backup.KubernetesVersion = c.Kubeadm.KubernetesVersion
	backup.FileName = fmt.Sprintf("%s-%s", c.Name, backup.Name)
	backup.BackupPointName = c.Labels[common.LabelBackupPoint]
	// check preferred node in cluster
	if backup.PreferredNode == "" {
		backup.PreferredNode = c.Kubeadm.Masters[0].ID
	}
	if len(c.Kubeadm.Masters.Intersect(v1.WorkerNode{
		ID: backup.PreferredNode,
	})) == 0 {
		log.Errorf("the node %s not a master node", backup.PreferredNode)
		return fmt.Errorf("the node %s not a master node", backup.PreferredNode)
	}

	requirement, err := labels.NewRequirement(common.LabelClusterName, selection.Equals, []string{c.Name})
	if err != nil {
		log.Error("Failed to create a requirement", zap.Error(err))
		return err
	}
	nodeList, err := r.NodeLister.List(labels.NewSelector().Add(*requirement))
	if err != nil {
		log.Error("Failed to list nodes in the cluster", zap.Error(err))
		return err
	}

	if backup.PreferredNode != "" {
		_, err := r.NodeLister.Get(backup.PreferredNode)
		if err != nil {
			log.Error("Failed to get node by name", zap.Error(err))
			return err
		}
	}

	// update cluster status to backing_up
	c.Status.Status = v1.ClusterStatusBackingUp

	// create operation
	op := &v1.Operation{}
	op.Name = uuid.New().String()
	op.Labels = make(map[string]string)
	op.Labels[common.LabelOperationAction] = v1.OperationBackupCluster
	op.Labels[common.LabelTimeoutSeconds] = strconv.Itoa(v1.DefaultBackupTimeoutSec)
	op.Labels[common.LabelClusterName] = c.Name
	op.Labels[common.LabelBackupName] = backup.Name
	op.Labels[common.LabelTopologyRegion] = c.Kubeadm.Masters[0].Labels[common.LabelTopologyRegion]
	op.Status.Status = v1.OperationStatusRunning
	// add backup
	backup.Labels = make(map[string]string)
	backup.Labels[common.LabelClusterName] = c.Name
	backup.Labels[common.LabelOperationName] = op.Name
	backup.Labels[common.LabelCronBackupCreated] = cronBackup.Name
	backup.Labels[common.LabelTimeoutSeconds] = strconv.Itoa(v1.DefaultBackupTimeoutSec)
	backup.ClusterNodes = make(map[string]string)

	for _, node := range nodeList {
		backup.ClusterNodes[node.Status.Ipv4DefaultIP] = node.Status.NodeInfo.Hostname
	}

	steps := make([]v1.Step, 0)
	var actBackup *k8s.ActBackup
	meta := component.ExtraMetadata{
		ClusterName: c.Name,
	}
	meta.Masters = append(meta.Masters, component.Node{ID: backup.PreferredNode})
	ctx := component.WithExtraMetadata(context.TODO(), meta)

	switch bp.StorageType {
	case bs.S3Storage:
		actBackup = &k8s.ActBackup{
			StoreType:       bp.StorageType,
			BackupFileName:  backup.FileName,
			AccessKeyID:     bp.S3Config.AccessKeyID,
			AccessKeySecret: bp.S3Config.AccessKeySecret,
			Bucket:          bp.S3Config.Bucket,
			Endpoint:        bp.S3Config.Endpoint,
		}
	case bs.FSStorage:
		actBackup = &k8s.ActBackup{
			StoreType:          bp.StorageType,
			BackupFileName:     backup.FileName,
			BackupPointRootDir: bp.FsConfig.BackupRootDir,
		}
	}

	if err = actBackup.InitSteps(ctx); err != nil {
		log.Error("Failed to init steps", zap.Error(err))
		return err
	}

	actBackupStep := actBackup.GetStep(v1.ActionInstall)
	op.Steps = append(steps, actBackupStep...)
	op, err = r.OperationWriter.CreateOperation(ctx, op)
	if err != nil {
		log.Error("Failed to create operation", zap.Error(err))
		return err
	}
	_, err = r.BackupWriter.CreateBackup(ctx, backup)
	if err != nil {
		log.Error("Failed to create backup", zap.Error(err))
		return err
	}
	_, err = r.ClusterWriter.UpdateCluster(context.TODO(), c)
	if err != nil {
		log.Error("Failed to update cluster", zap.Error(err))
		return err
	}

	// delivery the create backup operation
	go func() {
		err = r.CmdDelivery.DeliverTaskOperation(ctx, op, &service.Options{DryRun: false})
		log.Error("Failed to delivery operation", zap.Error(err))
	}()
	return nil
}
