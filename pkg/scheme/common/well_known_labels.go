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

package common

const (
	LabelHostname          = "kubeclipper.io/hostname"
	LabelOSStable          = "kubeclipper.io/os"
	LabelArchStable        = "kubeclipper.io/arch"
	LabelTopologyZone      = "topology.kubeclipper.io/zone"
	LabelTopologyRegion    = "topology.kubeclipper.io/region"
	LabelNodeRole          = "kubeclipper.io/nodeRole"
	LabelNodeDisable       = "kubeclipper.io/nodeDisable"
	LabelCreator           = "kubeclipper.io/creator"
	LabelUsername          = "kubeclipper.io/username"
	LabelClusterName       = "kubeclipper.io/cluster"
	LabelBackupName        = "kubeclipper.io/backup"
	LabelRecoveryName      = "kubeclipper.io/recovery"
	LabelOperationAction   = "kubeclipper.io/operation"
	LabelOperationName     = "kubeclipper.io/operation-name"
	LabelTimeoutSeconds    = "kubeclipper.io/timeout"
	LabelRoleTemplate      = "kubeclipper.io/role-template"
	LabelHidden            = "kubeclipper.io/hidden"
	LabelUserReference     = "iam.kubeclipper.io/user-ref"
	LabelExternalIP        = "kubeclipper.io/externalIP"
	LabelUpgradeVersion    = "kubeclipper.io/upgrade-version"
	LabelBackupPoint       = "kubeclipper.io/backupPoint"
	LabelCronBackupDisable = "kubeclipper.io/cronBackupDisable"
	LabelCronBackupEnable  = "kubeclipper.io/cronBackupEnable"
)

const (
	ResourceKindGlobalRole = "GlobalRole"
)

const (
	AnnotationAggregationRoles = "kubeclipper.io/aggregation-roles"
	RegoOverrideAnnotation     = "kubeclipper.io/rego-override"
	RoleAnnotation             = "iam.kubeclipper.io/role"
	AnnotationInternal         = "kubeclipper.io/internal"
)

type NodeRole string // master/worker/ingress(worker)

const (
	NodeRoleMaster NodeRole = "master"
	NodeRoleWorker NodeRole = "worker"
)

func (nr NodeRole) String() string {
	return string(nr)
}

const (
	LabelIDP       = "iam.kubeclipper.io/idp"
	LabelOriginUID = "iam.kubeclipper.io/origin-uid"
)

const (
	// eg: cinder/v1
	LabelComponentName    = "kubeclipper.io/componentName"
	LabelComponentVersion = "kubeclipper.io/componentVersion"
	// eg: storage
	LabelCategory = "kubeclipper.io/category"
	// eg: name
	AnnotationDisplayName = "kubeclipper.io/display-name"
	AnnotationDescription = "kubeclipper.io/description"
)
