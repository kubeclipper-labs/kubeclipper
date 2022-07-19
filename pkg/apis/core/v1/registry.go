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
	"net/http"

	ctrl "github.com/kubeclipper/kubeclipper/pkg/controller-runtime"
	"github.com/kubeclipper/kubeclipper/pkg/models/cluster"

	"github.com/kubeclipper/kubeclipper/pkg/models/platform"

	"github.com/kubeclipper/kubeclipper/pkg/errors"
	"github.com/kubeclipper/kubeclipper/pkg/models/lease"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubeclipper/kubeclipper/pkg/models"
	"github.com/kubeclipper/kubeclipper/pkg/models/operation"
	"github.com/kubeclipper/kubeclipper/pkg/query"
	corev1 "github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1"
	"github.com/kubeclipper/kubeclipper/pkg/server/runtime"
	"github.com/kubeclipper/kubeclipper/pkg/service"
)

var GroupVersion = schema.GroupVersion{Group: corev1.GroupName, Version: "v1"}

const (
	CoreClusterTag = "Core-Cluster"
	CoreNodeTag    = "Core-Node"
	CoreRegionTag  = "Core-Region"
)

/*
this is how set up web service route only
in that case cli tool can simply call it to get api route by pass in a nil parameter
*/
func SetupWebService(h *handler) *restful.WebService {
	webservice := runtime.NewWebService(GroupVersion)

	webservice.Route(webservice.GET("/clusters").
		To(h.ListClusters).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("List clusters.").
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterWatch, "watch request").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterTimeoutSeconds, "watch timeout seconds").
			DataType("integer").
			DefaultValue("60").
			Required(false)).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}))

	webservice.Route(webservice.GET("/clusters/{name}/terminal").
		To(h.SSHToPod).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("kubectl web terminal").
		Param(webservice.PathParameter(query.ParameterName, "cluster name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(ParameterToken, "auth token").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil))

	webservice.Route(webservice.POST("/clusters").
		To(h.CreateClusters).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Create clusters.").
		Reads(corev1.Cluster{}).
		Param(webservice.QueryParameter(query.ParamDryRun, "dry run create clusters").
			Required(false).DataType("boolean")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Cluster{}))

	webservice.Route(webservice.PUT("/clusters/{name}").
		To(h.UpdateClusters).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Update clusters.").
		Reads(corev1.Cluster{}).
		Param(webservice.PathParameter("name", "cluster name")).
		Param(webservice.QueryParameter(query.ParamDryRun, "dry run update clusters").
			Required(false).DataType("boolean")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil))

	webservice.Route(webservice.DELETE("/clusters/{name}").
		To(h.DeleteCluster).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Delete clusters.").
		Param(webservice.PathParameter("name", "cluster name")).
		Param(webservice.QueryParameter(query.ParamDryRun, "dry run delete clusters").
			Required(false).DataType("boolean")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil))

	webservice.Route(webservice.GET("/clusters/{name}").
		To(h.DescribeCluster).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Describe cluster.").
		Param(webservice.PathParameter(query.ParameterName, "cluster name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParameterResourceVersion, "resource version to query").
			Required(false).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Cluster{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.PUT("/clusters/{name}/nodes").
		To(h.AddOrRemoveNodes).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Add or remove cluster node.").
		Reads(PatchNodes{}).
		Param(webservice.PathParameter(query.ParameterName, "cluster name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Cluster{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.GET("/clusters/{name}/backups").
		To(h.ListBackupsWithCluster).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("List backups.").
		Param(webservice.PathParameter(query.ParameterName, "cluster name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.POST("/clusters/{name}/backups").
		To(h.CreateBackup).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Create backups.").
		Reads(corev1.Backup{}).
		Param(webservice.PathParameter(query.ParameterName, "cluster name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParamDryRun, "dry run create clusters").
			Required(false).DataType("boolean")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Backup{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.DELETE("/clusters/{cluster}/backups/{backup}").
		To(h.DeleteBackup).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Delete backups.").
		Param(webservice.PathParameter("cluster", "cluster name")).
		Param(webservice.PathParameter("backup", "backup name")).
		Param(webservice.QueryParameter(query.ParamDryRun, "dry run create clusters").
			Required(false).DataType("boolean")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.PUT("/clusters/{cluster}/backups/{backup}").
		To(h.UpdateBackup).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Update backups.").
		Param(webservice.PathParameter("cluster", "cluster name")).
		Param(webservice.PathParameter("backup", "backup name")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Backup{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.POST("/clusters/{cluster}/recovery").
		To(h.CreateRecovery).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("create recovery.").
		Reads(corev1.Recovery{}).
		Param(webservice.PathParameter("cluster", "cluster name")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Recovery{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.PATCH("/clusters/{cluster}/plugins").
		To(h.InstallOrUninstallPlugins).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Install or uninstall plugins").
		Reads(PatchComponents{}).
		Param(webservice.PathParameter("cluster", "cluster name")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Cluster{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.GET("/nodes").
		To(h.ListNodes).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreNodeTag}).
		Doc("List nodes.").
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterWatch, "watch request").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterTimeoutSeconds, "watch timeout seconds").
			DataType("integer").
			DefaultValue("60").
			Required(false)).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}))

	webservice.Route(webservice.GET("/regions").
		To(h.ListRegions).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreRegionTag}).
		Doc("List regions.").
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterWatch, "watch request").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterTimeoutSeconds, "watch timeout seconds").
			DataType("integer").
			DefaultValue("60").
			Required(false)).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}))

	webservice.Route(webservice.GET("/regions/{name}").
		To(h.DescribeRegion).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreRegionTag}).
		Doc("Describe region.").
		Param(webservice.PathParameter(query.ParameterName, "region name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParameterResourceVersion, "resource version to query").
			Required(false).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Region{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.GET("/nodes/{name}").
		To(h.DescribeNode).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreNodeTag}).
		Doc("Describe nodes.").
		Param(webservice.PathParameter(query.ParameterName, "node name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParameterResourceVersion, "resource version to query").
			Required(false).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Node{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.PATCH("/nodes/{name}/disable").
		To(h.DisableNode).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreNodeTag}).
		Doc("Disable nodes.").
		Param(webservice.PathParameter(query.ParameterName, "node name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Node{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.PATCH("/nodes/{name}/enable").
		To(h.EnableNode).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreNodeTag}).
		Doc("Enable nodes.").
		Param(webservice.PathParameter(query.ParameterName, "node name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Node{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.DELETE("/nodes/{name}").
		To(h.DeleteNode).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreNodeTag}).
		Doc("Delete node.").
		Param(webservice.PathParameter(query.ParameterName, "node name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.GET("/logs").
		To(h.GetOperationLog).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreNodeTag}).
		Doc("Get operation log on node.").
		Param(webservice.QueryParameter(query.ParameterNode, "node name").
			Required(true).
			DataFormat("node=%s")).
		Param(webservice.QueryParameter(query.ParameterOperation, "operation id").
			Required(true).
			DataFormat("operation=%s")).
		Param(webservice.QueryParameter(query.ParameterStep, "step id").
			Required(true).
			DataFormat("step=%s")).
		Param(webservice.QueryParameter(query.ParameterOffset, "offset").
			Required(false).
			DataFormat("offset=%s")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), StepLog{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.GET("/operations").
		To(h.ListOperations).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("List operations.").
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterWatch, "watch request").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterTimeoutSeconds, "watch timeout seconds").
			DataType("integer").
			DefaultValue("60").
			Required(false)).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.OperationList{}))

	webservice.Route(webservice.GET("/operations/{name}").
		To(h.DescribeOperation).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreNodeTag}).
		Doc("Describe operations.").
		Param(webservice.PathParameter(query.ParameterName, "operation name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParameterResourceVersion, "resource version to query").
			Required(false).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Operation{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.POST("/operations/{name}/retry").
		To(h.RetryCluster).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("clusters retry operation.").
		Param(webservice.QueryParameter(query.ParamDryRun, "dry run clusters retry operation.").
			Required(false).DataType("boolean")).
		Param(webservice.PathParameter(query.ParameterName, "operation name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Cluster{}))

	webservice.Route(webservice.POST("/clusters/{name}/upgrade").
		To(h.UpgradeCluster).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("upgrade cluster.").
		Reads(ClusterUpgrade{}).
		Param(webservice.QueryParameter(query.ParamDryRun, "dry run upgrade cluster.").
			Required(false).DataType("boolean")).
		Param(webservice.PathParameter(query.ParameterName, "cluster name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil))

	webservice.Route(webservice.PATCH("/clusters/{name}/status").
		To(h.ResetClusterStatus).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("reset cluster status.").
		Param(webservice.QueryParameter(query.ParamDryRun, "dry run upgrade cluster.").
			Required(false).DataType("boolean")).
		Param(webservice.PathParameter(query.ParameterName, "cluster name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil))

	webservice.Route(webservice.GET("/leases").
		To(h.ListLeases).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreRegionTag}).
		Doc("List leases.").
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterWatch, "watch request").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterTimeoutSeconds, "watch timeout seconds").
			DataType("integer").
			DefaultValue("60").
			Required(false)).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}))

	webservice.Route(webservice.GET("/leases/{name}").
		To(h.DescribeLease).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreRegionTag}).
		Doc("Describe region.").
		Param(webservice.PathParameter(query.ParameterName, "region name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParameterResourceVersion, "resource version to query").
			Required(false).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Region{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.GET("/backups").
		To(h.ListBackups).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreRegionTag}).
		Doc("List backups.").
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterWatch, "watch request").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterTimeoutSeconds, "watch timeout seconds").
			DataType("integer").
			DefaultValue("60").
			Required(false)).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}))

	webservice.Route(webservice.GET("/backups/{name}").
		To(h.DescribeBackup).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreRegionTag}).
		Doc("Describe backup.").
		Param(webservice.PathParameter(query.ParameterName, "backup name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParameterResourceVersion, "resource version to query").
			Required(false).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Backup{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.GET("/nodes/{name}/terminal").
		To(h.SSHToNode).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreRegionTag}).
		Doc("connector node with ssh protocol").
		Param(webservice.PathParameter(query.ParameterName, "node name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(ParameterToken, "auth token").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(ParameterMsg, "user name, password, port").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(ParameterCols, "terminal cols").
			Required(false).
			DefaultValue("150").
			DataType("string")).
		Param(webservice.QueryParameter(ParameterRows, "terminal rows").
			Required(false).
			DefaultValue("35").
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusExpectationFailed, http.StatusText(http.StatusExpectationFailed), errors.HTTPError{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), errors.HTTPError{}))

	webservice.Route(webservice.GET("/domains").
		To(h.ListDomains).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("List domains.").
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterWatch, "watch request").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterTimeoutSeconds, "watch timeout seconds").
			DataType("integer").
			DefaultValue("60").
			Required(false)).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}))

	webservice.Route(webservice.GET("/domains/{name}").
		To(h.GetDomain).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Describe domain.").
		Param(webservice.PathParameter(query.ParameterName, "domain").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Domain{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.HEAD("/domains").
		To(h.CheckDomainExists).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("check domains exists.").
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.POST("/domains").
		To(h.CreateDomains).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Create domains.").
		Reads(corev1.Domain{}).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Domain{}).
		Returns(http.StatusBadRequest, http.StatusText(http.StatusBadRequest), errors.HTTPError{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.DELETE("/domains/{name}").
		To(h.DeleteDomain).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Delete domain and all record association to this domain.").
		Param(webservice.PathParameter(query.ParameterName, "domain").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.PUT("/domains/{name}").
		To(h.UpdateDomain).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Update domain.").
		Reads(corev1.Domain{}).
		Param(webservice.PathParameter(query.ParameterName, "domain").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Domain{}).
		Returns(http.StatusBadRequest, http.StatusText(http.StatusBadRequest), errors.HTTPError{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.GET("/domains/{name}/records").
		To(h.ListRecords).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("List records.").
		Param(webservice.PathParameter(query.ParameterName, "domain").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParameterSubDomain, "subdomain").
			Required(false).
			DataType("string")).
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}))

	webservice.Route(webservice.GET("/domains/{name}/records/{subdomain}").
		To(h.GetRecord).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Describe records.").
		Param(webservice.PathParameter(query.ParameterName, "domain").
			Required(true).
			DataType("string")).
		Param(webservice.PathParameter(query.ParameterSubDomain, "subdomain").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Region{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.POST("/domains/{name}/records").
		To(h.CreateRecords).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Create records.").
		Reads(corev1.Record{}).
		Param(webservice.PathParameter(query.ParameterName, "domain").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Record{}).
		Returns(http.StatusBadRequest, http.StatusText(http.StatusBadRequest), errors.HTTPError{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.PUT("/domains/{name}/records").
		To(h.UpdateRecord).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Update records.").
		Reads(corev1.Record{}).
		Param(webservice.PathParameter(query.ParameterName, "domain").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Record{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.DELETE("/domains/{name}/records/{subdomain}").
		To(h.DeleteRecord).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Delete records.").
		Param(webservice.PathParameter(query.ParameterName, "domain").
			Required(true).
			DataType("string")).
		Param(webservice.PathParameter(query.ParameterSubDomain, "subdomain").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.GET("/templates").
		To(h.ListTemplates).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("List templates.").
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.GET("/templates/{name}").
		To(h.DescribeTemplate).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Describe template.").
		Param(webservice.PathParameter(query.ParameterName, "name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Template{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.POST("/templates").
		To(h.CreateTemplate).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Create template.").
		Reads(corev1.Template{}).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Template{}).
		Returns(http.StatusBadRequest, http.StatusText(http.StatusBadRequest), errors.HTTPError{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.PUT("/templates/{name}").
		To(h.UpdateTemplate).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Update template.").
		Reads(corev1.Template{}).
		Param(webservice.PathParameter(query.ParameterName, "name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.Template{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.DELETE("/templates/{name}").
		To(h.DeleteTemplate).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Delete template.").
		Param(webservice.PathParameter(query.ParameterName, "name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.GET("/backuppoints").
		Doc("List of backup point").
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		To(h.ListBackupPoints).
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterFuzzySearch, "fuzzy search conditions").
			DataFormat("foo~bar,bar~baz").
			Required(false)).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.GET("/backuppoints/{name}").
		Doc("Get a backup point by name").
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Param(webservice.PathParameter(query.ParameterName, "backup point name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParameterResourceVersion, "resource version to query").
			Required(false).
			DataType("string")).
		To(h.DescribeBackupPoint).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.BackupPoint{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.POST("/backuppoints").
		Doc("Create a backup point").
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		To(h.CreateBackupPoint).
		Reads(corev1.BackupPoint{}).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.BackupPoint{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.PUT("/backuppoints/{name}").
		To(h.UpdateBackupPoint).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Update backup point.").
		Param(webservice.PathParameter("name", "backup point name")).
		Param(webservice.QueryParameter(query.ParameterResourceVersion, "resource version to query").
			Required(false).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.BackupPoint{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.DELETE("/backuppoints/{name}").
		To(h.DeleteBackupPoint).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Delete a backup point.").
		Param(webservice.PathParameter(query.ParameterName, "backup point").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.GET("/cronbackups").
		Doc("List of cronbackup.").
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		To(h.ListCronBackups).
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(query.ParameterLabelSelector, "resource filter by metadata label").
			Required(false).
			DataFormat("labelSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParameterFieldSelector, "resource filter by field").
			Required(false).
			DataFormat("fieldSelector=%s=%s")).
		Param(webservice.QueryParameter(query.ParamReverse, "resource sort reverse or not").Required(false).
			DataType("boolean")).
		Param(webservice.QueryParameter(query.ParameterFuzzySearch, "fuzzy search conditions").
			DataFormat("foo~bar,bar~baz").
			Required(false)).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.GET("/cronbackups/{name}").
		Doc("Get a cronbackup by name.").
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Param(webservice.PathParameter(query.ParameterName, "cronbackup name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParameterResourceVersion, "resource version to query").
			Required(false).
			DataType("string")).
		To(h.DescribeCronBackup).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.CronBackup{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.POST("/cronbackups").
		Doc("Create a cronbackup.").
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		To(h.CreateCronBackup).
		Reads(corev1.CronBackup{}).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.CronBackup{}).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.PUT("/cronbackups/{name}").
		To(h.UpdateCronBackup).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Update cronbackup.").
		Param(webservice.PathParameter(query.ParameterName, "cronbackup name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.ParameterResourceVersion, "resource version to query").
			Required(false).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), corev1.CronBackup{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.DELETE("/cronbackups/{name}").
		To(h.DeleteCronBackup).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("Delete a cronbackup.").
		Param(webservice.PathParameter(query.ParameterName, "cronbackup name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.PATCH("/cronbackups/{name}/enable").
		To(h.EnableCronBackup).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("enable a cronbackup.").
		Param(webservice.PathParameter(query.ParameterName, "cronbackup name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.PATCH("/cronbackups/{name}/disable").
		To(h.DisableCronBackup).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("disable a cronbackup.").
		Param(webservice.PathParameter(query.ParameterName, "cronbackup name").
			Required(true).
			DataType("string")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), nil).
		Returns(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), errors.HTTPError{}))

	webservice.Route(webservice.GET("/cronbackups/{name}/backups").
		To(h.ListBackupsWithCronBackup).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("List backups created by cronBackup.").
		Param(webservice.PathParameter(query.ParameterName, "cronBackup name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	webservice.Route(webservice.GET("/clusters/{name}/cronbackups").
		To(h.ListCronBackupsWithCluster).
		Metadata(restfulspec.KeyOpenAPITags, []string{CoreClusterTag}).
		Doc("List cronBackups of a cluster by name.").
		Param(webservice.PathParameter("name", "cluster name").
			Required(true).
			DataType("string")).
		Param(webservice.QueryParameter(query.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Returns(http.StatusOK, http.StatusText(http.StatusOK), models.PageableResponse{}).
		Returns(http.StatusNotFound, http.StatusText(http.StatusNotFound), nil))

	return webservice
}

func AddToContainer(c *restful.Container, clusterOperator cluster.Operator, op operation.Operator, platform platform.Operator,
	leaseOperator lease.Operator, delivery service.IDelivery, mgr ctrl.Manager) error {
	h := newHandler(clusterOperator, op, leaseOperator, platform, delivery, mgr)
	webservice := SetupWebService(h)
	c.Add(webservice)
	return nil
}
