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

package main

import (
	"encoding/json"
	"io/ioutil"

	iamv1 "github.com/kubeclipper/kubeclipper/pkg/scheme/iam/v1"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var roles = []iamv1.GlobalRole{
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/module":              "Access Control",
				"kubeclipper.io/role-template-rules": "{\"users\": \"view\"}",
				"kubeclipper.io/alias-name":          "Users View",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-view-users",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"users", "users/loginrecords"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-users\"]",
				"kubeclipper.io/module":              "Access Control",
				"kubeclipper.io/role-template-rules": "{\"roles\": \"view\"}",
				"kubeclipper.io/alias-name":          "Roles View",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-view-roles",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"roles"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-roles\"]",
				"kubeclipper.io/module":              "Access Control",
				"kubeclipper.io/role-template-rules": "{\"roles\": \"create\"}",
				"kubeclipper.io/alias-name":          "Roles Create",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-create-roles",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"roles"},
				Verbs:     []string{"create"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-roles\"]",
				"kubeclipper.io/module":              "Access Control",
				"kubeclipper.io/role-template-rules": "{\"roles\": \"edit\"}",
				"kubeclipper.io/alias-name":          "Roles Edit",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-edit-roles",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"roles"},
				Verbs:     []string{"update", "patch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-roles\"]",
				"kubeclipper.io/module":              "Access Control",
				"kubeclipper.io/role-template-rules": "{\"roles\": \"delete\"}",
				"kubeclipper.io/alias-name":          "Roles Delete",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-delete-roles",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"roles"},
				Verbs:     []string{"delete"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-users\",\"role-template-view-roles\"]",
				"kubeclipper.io/module":              "Access Control",
				"kubeclipper.io/role-template-rules": "{\"users\": \"create\"}",
				"kubeclipper.io/alias-name":          "Users Create",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-create-users",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"users"},
				Verbs:     []string{"create"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-users\",\"role-template-view-roles\"]",
				"kubeclipper.io/module":              "Access Control",
				"kubeclipper.io/role-template-rules": "{\"users\": \"edit\"}",
				"kubeclipper.io/alias-name":          "Users Edit",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-edit-users",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"users", "users/password", "users/enable", "users/disable"},
				Verbs:     []string{"update", "patch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-users\",\"role-template-view-roles\"]",
				"kubeclipper.io/module":              "Access Control",
				"kubeclipper.io/role-template-rules": "{\"users\": \"delete\"}",
				"kubeclipper.io/alias-name":          "Users Delete",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-delete-users",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"users"},
				Verbs:     []string{"delete"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/module":              "Cluster Management",
				"kubeclipper.io/role-template-rules": "{\"clusters\": \"view\"}",
				"kubeclipper.io/alias-name":          "Cluster View",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-view-clusters",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"clusters", "nodes", "regions", "operations", "logs", "clusters/upgrade", "nodes/terminal"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"config.kubeclipper.io"},
				Resources: []string{"terminal.key"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-clusters\"]",
				"kubeclipper.io/module":              "Cluster Management",
				"kubeclipper.io/role-template-rules": "{\"clusters\": \"create\"}",
				"kubeclipper.io/alias-name":          "Cluster Create",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-create-clusters",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"clusters", "nodes", "regions", "operations/retry", "clusters/backups", "clusters/upgrade"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{"config.kubeclipper.io"},
				Resources: []string{"template"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"templates"},
				Verbs:     []string{"get", "list", "watch", "create"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"backuppoints"},
				Verbs:     []string{"get", "list", "watch", "create"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"cronbackups"},
				Verbs:     []string{"get", "list", "watch", "create"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-clusters\"]",
				"kubeclipper.io/module":              "Cluster Management",
				"kubeclipper.io/role-template-rules": "{\"clusters\": \"edit\"}",
				"kubeclipper.io/alias-name":          "Cluster Edit",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-edit-clusters",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"clusters", "clusters/plugins", "clusters/nodes", "clusters/status", "nodes/disable", "nodes/enable"},
				Verbs:     []string{"update", "patch"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"clusters/plugins", "clusters/nodes"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"clusters/terminal"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"backuppoints"},
				Verbs:     []string{"update", "patch"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"cronbackups"},
				Verbs:     []string{"update", "patch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-clusters\"]",
				"kubeclipper.io/module":              "Cluster Management",
				"kubeclipper.io/role-template-rules": "{\"clusters\": \"delete\"}",
				"kubeclipper.io/alias-name":          "Cluster Delete",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-delete-clusters",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"clusters", "nodes", "regions", "clusters/plugins", "clusters/nodes"},
				Verbs:     []string{"update", "patch"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"clusters/plugins", "clusters/nodes"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"backuppoints"},
				Verbs:     []string{"delete"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"cronbackups"},
				Verbs:     []string{"delete"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/module":              "Platform Setting",
				"kubeclipper.io/role-template-rules": "{\"platform\": \"view\"}",
				"kubeclipper.io/alias-name":          "Platform View",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-view-platform",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"config.kubeclipper.io"},
				Resources: []string{"template"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"templates"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-platform\"]",
				"kubeclipper.io/module":              "Platform Setting",
				"kubeclipper.io/role-template-rules": "{\"platform\": \"edit\"}",
				"kubeclipper.io/alias-name":          "Platform Edit",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-edit-platform",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"config.kubeclipper.io"},
				Resources: []string{"template"},
				Verbs:     []string{"update", "patch"},
			},
			{
				APIGroups: []string{"config.kubeclipper.io"},
				Resources: []string{"terminal.key"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{"config.kubeclipper.io"},
				Resources: []string{"templates"},
				Verbs:     []string{"update", "patch", "create", "delete"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/module":              "Audit",
				"kubeclipper.io/role-template-rules": "{\"audit\": \"view\"}",
				"kubeclipper.io/alias-name":          "Audit View",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-view-audit",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"audit.kubeclipper.io"},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/module":              "DNS",
				"kubeclipper.io/role-template-rules": "{\"dns\": \"view\"}",
				"kubeclipper.io/alias-name":          "DNS View",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-view-dns",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"domains", "domains/records"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-dns\"]",
				"kubeclipper.io/module":              "DNS",
				"kubeclipper.io/role-template-rules": "{\"dns\": \"delete\"}",
				"kubeclipper.io/alias-name":          "DNS Delete",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-delete-dns",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"domains", "domains/records"},
				Verbs:     []string{"delete"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-dns\"]",
				"kubeclipper.io/module":              "DNS",
				"kubeclipper.io/role-template-rules": "{\"dns\": \"create\"}",
				"kubeclipper.io/alias-name":          "DNS Create",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-create-dns",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"domains", "domains/records"},
				Verbs:     []string{"create"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/dependencies":        "[\"role-template-view-dns\"]",
				"kubeclipper.io/module":              "DNS",
				"kubeclipper.io/role-template-rules": "{\"dns\": \"edit\"}",
				"kubeclipper.io/alias-name":          "DNS Edit",
				"kubeclipper.io/internal":            "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/role-template": "true",
			},
			Name: "role-template-edit-dns",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"domains", "domains/records"},
				Verbs:     []string{"update", "patch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/aggregation-roles": "[\"role-template-create-clusters\",\"role-template-edit-clusters\",\"role-template-delete-clusters\",\"role-template-view-clusters\",\"role-template-view-roles\",\"role-template-create-roles\",\"role-template-edit-roles\",\"role-template-delete-roles\",\"role-template-create-users\",\"role-template-edit-users\",\"role-template-delete-users\",\"role-template-view-users\",\"role-template-view-platform\",\"role-template-edit-platform\",\"role-template-view-audit\",\"role-template-create-dns\",\"role-template-edit-dns\",\"role-template-delete-dns\",\"role-template-view-dns\"]",
				"kubeclipper.io/internal":          "true",
			},
			Name: "platform-admin",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				NonResourceURLs: []string{"*"},
				Verbs:           []string{"*"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/aggregation-roles": "[\"role-template-view-clusters\",\"role-template-view-roles\",\"role-template-view-users\",\"role-template-view-platform\",\"role-template-view-audit\",\"role-template-view-dns\"]",
				"kubeclipper.io/internal":          "true",
			},
			Name: "platform-view",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/rego-override": "package authz\ndefault allow = false\nallow = true {\n  allowedResources := [\"users\"]\n  allowedResources[_] == input.Resource\n  input.User.Name == input.Name\n}",
				"kubeclipper.io/internal":      "true",
			},
			Labels: map[string]string{
				"kubeclipper.io/hidden": "true",
			},
			Name: "authenticated",
		},
		Rules: []rbacv1.PolicyRule{
			{
				NonResourceURLs: []string{"*"},
				Verbs:           []string{"*"},
			},
			{
				APIGroups: []string{"config.kubeclipper.io"},
				Resources: []string{"configz", "components", "componentmeta"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"config.kubeclipper.io"},
				Resources: []string{"oauth"},
				Verbs:     []string{"get", "list", "watch", "update", "patch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "anonymous",
			Labels: map[string]string{
				"kubeclipper.io/hidden": "true",
			},
			Annotations: map[string]string{
				"kubeclipper.io/internal": "true",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				NonResourceURLs: []string{"*"},
				Verbs:           []string{"*"},
			},
			{
				APIGroups: []string{"config.kubeclipper.io"},
				Resources: []string{"oauth"},
				Verbs:     []string{"get", "list", "watch", "update", "patch"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/aggregation-roles": "[\"role-template-view-roles\",\"role-template-create-roles\",\"role-template-edit-roles\",\"role-template-delete-roles\",\"role-template-create-users\",\"role-template-edit-users\",\"role-template-delete-users\",\"role-template-view-users\"]",
				"kubeclipper.io/internal":          "true",
			},
			Name: "iam-manager",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"users"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"users", "users/password"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"roles"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"iam.kubeclipper.io"},
				Resources: []string{"roles"},
				Verbs:     []string{"*"},
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       iamv1.KindGlobalRole,
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubeclipper.io/aggregation-roles": "[\"role-template-create-clusters\",\"role-template-edit-clusters\",\"role-template-delete-clusters\",\"role-template-view-clusters\"]",
				"kubeclipper.io/internal":          "true",
			},
			Name: "cluster-manager",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"clusters", "nodes", "regions", "operations"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"core.kubeclipper.io"},
				Resources: []string{"clusters", "clusters/nodes", "clusters/plugins", "nodes", "regions", "operations"},
				Verbs:     []string{"*"},
			},
		},
	},
}

var rolebindins = []iamv1.GlobalRoleBinding{
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GlobalRoleBinding",
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "anonymous",
			Annotations: map[string]string{
				"kubeclipper.io/internal": "true",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "iam.kubeclipper.io",
			Kind:     "GlobalRole",
			Name:     "anonymous",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:unauthenticated",
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GlobalRoleBinding",
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "authenticated",
			Annotations: map[string]string{
				"kubeclipper.io/internal": "true",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "iam.kubeclipper.io",
			Kind:     "GlobalRole",
			Name:     "authenticated",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:authenticated",
			},
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GlobalRoleBinding",
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "platform-admin",
			Annotations: map[string]string{
				"kubeclipper.io/internal": "true",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "iam.kubeclipper.io",
			Kind:     "GlobalRole",
			Name:     "platform-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "User",
				Name:     "system:kc-server",
			},
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "User",
				Name:     "admin",
			},
		},
	},
}

var users = []iamv1.User{
	{
		TypeMeta: metav1.TypeMeta{
			Kind:       "User",
			APIVersion: iamv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "admin",
			Annotations: map[string]string{
				"kubeclipper.io/internal": "true",
			},
		},
		Spec: iamv1.UserSpec{
			Email:             "admin@kubeclipper.com",
			Lang:              "",
			Phone:             "",
			Description:       "Platform Admin",
			DisplayName:       "admin",
			Groups:            nil,
			EncryptedPassword: "Thinkbig1",
		},
	},
}

func main() {
	rByte, err := json.MarshalIndent(roles, "", "\t")
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile("role.json", rByte, 0644); err != nil {
		panic(err)
	}
	rByte, err = json.MarshalIndent(rolebindins, "", "\t")
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile("rolebinding.json", rByte, 0644); err != nil {
		panic(err)
	}
	rByte, err = json.MarshalIndent(users, "", "\t")
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile("user.json", rByte, 0644); err != nil {
		panic(err)
	}
}
