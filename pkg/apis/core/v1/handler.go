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
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"k8s.io/client-go/tools/remotecommand"

	"github.com/kubeclipper/kubeclipper/pkg/controller-runtime/client"

	bs "github.com/kubeclipper/kubeclipper/pkg/simple/backupstore"

	"github.com/kubeclipper/kubeclipper/pkg/controller"
	"github.com/kubeclipper/kubeclipper/pkg/oplog"

	"github.com/kubeclipper/kubeclipper/pkg/utils/certs"

	"github.com/gorilla/websocket"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/kubeclipper/kubeclipper/pkg/models/platform"
	"github.com/kubeclipper/kubeclipper/pkg/utils/sshutils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubeclipper/kubeclipper/pkg/scheme/common"
	"github.com/kubeclipper/kubeclipper/pkg/scheme/core/validation"

	"github.com/kubeclipper/kubeclipper/pkg/client/clientrest"

	"github.com/kubeclipper/kubeclipper/pkg/models/lease"

	"github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1/k8s"

	"github.com/kubeclipper/kubeclipper/pkg/utils/netutil"

	"github.com/google/uuid"

	apimachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/emicklei/go-restful"
	"go.uber.org/zap"

	"github.com/kubeclipper/kubeclipper/pkg/component"
	"github.com/kubeclipper/kubeclipper/pkg/logger"
	"github.com/kubeclipper/kubeclipper/pkg/models/cluster"
	"github.com/kubeclipper/kubeclipper/pkg/models/operation"
	"github.com/kubeclipper/kubeclipper/pkg/query"
	v1 "github.com/kubeclipper/kubeclipper/pkg/scheme/core/v1"
	"github.com/kubeclipper/kubeclipper/pkg/server/restplus"
	"github.com/kubeclipper/kubeclipper/pkg/service"
	"github.com/kubeclipper/kubeclipper/pkg/utils/strutil"
)

type handler struct {
	clusterOperator  cluster.Operator
	leaseOperator    lease.Operator
	opOperator       operation.Operator
	platformOperator platform.Operator
	delivery         service.IDelivery
}

const (
	ParameterMsg               = "msg"
	ParameterToken             = "token"
	ParameterCols              = "cols"
	ParameterRows              = "rows"
	resourceExistCheckerHeader = "X-CHECK-EXIST"
)

var (
	ErrNodesRegionDifferent = errors.New("nodes belongs to different region")
)

func newHandler(clusterOperator cluster.Operator, op operation.Operator, leaseOperator lease.Operator,
	platform platform.Operator, delivery service.IDelivery) *handler {
	return &handler{
		clusterOperator:  clusterOperator,
		delivery:         delivery,
		opOperator:       op,
		platformOperator: platform,
		leaseOperator:    leaseOperator,
	}
}

func (h *handler) ListClusters(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	if q.Watch {
		h.watchCluster(request, response, q)
		return
	}
	if clientrest.IsInformerRawQuery(request.Request) {
		result, err := h.clusterOperator.ListClusters(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	} else {
		result, err := h.clusterOperator.ListClusterEx(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	}
}

func (h *handler) DescribeCluster(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	resourceVersion := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	c, err := h.clusterOperator.GetClusterEx(request.Request.Context(), name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, c)
}

func (h *handler) AddOrRemoveNodes(request *restful.Request, response *restful.Response) {
	pn := &PatchNodes{}
	if err := request.ReadEntity(pn); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	// cluster name in path
	clu := request.PathParameter("name")
	timeoutSecs := v1.DefaultOperationTimeoutSecs
	if v := request.QueryParameter("timeout"); v != "" {
		timeoutSecs = v
	}

	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)
	ctx := request.Request.Context()
	c, err := h.clusterOperator.GetClusterEx(ctx, clu, "0")
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	// backing up old masters and workers
	nodeSet := c.GetAllNodes()

	// We need IP addresses of all master nodes later.
	extraMeta, err := h.getClusterMetadata(ctx, c)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) || err == ErrNodesRegionDifferent {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	if err := pn.MakeCompare(c); err != nil {
		if errors.Is(err, ErrInvalidNodesOperation) || errors.Is(err, ErrInvalidNodesRole) {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	// Get workers node information must be called before pn.MakeCompare, in order to ensure pn.Nodes = extraMeta.Workers.
	// Replace cluster worker nodes with nodes to be operated.
	nodes, err := h.getNodeInfo(ctx, pn.Nodes)
	if err != nil {
		if err == ErrNodesRegionDifferent {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	if pn.Role == common.NodeRoleWorker {
		extraMeta.Workers = append(extraMeta.Workers, nodes...)
	}

	if len(nodes) == 0 {
		restplus.HandleBadRequest(response, request, fmt.Errorf("nodes is already in use"))
		return
	}

	for _, n := range nodes {
		switch pn.Operation {
		case NodesOperationAdd:
			if n.Disable {
				restplus.HandleBadRequest(response, request, fmt.Errorf("this node(%s) is disabled", n.IPv4))
				return
			}
			if nodeSet.Has(n.ID) {
				restplus.HandleBadRequest(response, request, fmt.Errorf("this node(%s) is already in use", n.IPv4))
				return
			}
			if n.Region != extraMeta.Masters[0].Region {
				restplus.HandleBadRequest(response, request, fmt.Errorf("the node(%s) belongs to different region", n.IPv4))
				return
			}
		case NodesOperationRemove:
			if !nodeSet.Has(n.ID) {
				restplus.HandleBadRequest(response, request, fmt.Errorf("the node(%s) is not part of this cluster and cannot be removed", n.IPv4))
				return
			}
		}
	}

	op, err := pn.MakeOperation(*extraMeta, c)
	if err != nil {
		if errors.Is(err, ErrZeroNode) {
			// No node needs to be operated.
			_ = response.WriteHeaderAndEntity(http.StatusOK, c)
			return
		} else if errors.Is(err, ErrInvalidNodesOperation) || errors.Is(err, ErrInvalidNodesRole) {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	op.Labels[common.LabelTimeoutSeconds] = timeoutSecs
	op.Status.Status = v1.OperationStatusRunning
	if !dryRun {
		c.Status.Status = v1.ClusterStatusUpdating
		if c, err = h.clusterOperator.UpdateCluster(ctx, c); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		if op, err = h.opOperator.CreateOperation(ctx, op); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}

	// distribute tasks
	go h.doOperation(context.TODO(), op, &service.Options{DryRun: dryRun})

	_ = response.WriteHeaderAndEntity(http.StatusOK, c)
}

func (h *handler) watchCluster(req *restful.Request, resp *restful.Response, q *query.Query) {
	timeout := time.Duration(0)
	if q.TimeoutSeconds != nil {
		timeout = time.Duration(*q.TimeoutSeconds) * time.Second
	}
	if timeout == 0 {
		timeout = time.Duration(float64(query.MinTimeoutSeconds) * (rand.Float64() + 1.0))
	}

	watcher, err := h.clusterOperator.WatchClusters(req.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}
	restplus.ServeWatch(watcher, v1.SchemeGroupVersion.WithKind("Cluster"), req, resp, timeout)
}

func (h *handler) DeleteCluster(request *restful.Request, response *restful.Response) {
	name := request.PathParameter("name")
	var timeoutSecs string
	if v := request.QueryParameter("timeout"); v != "" {
		timeoutSecs = v
	} else {
		timeoutSecs = v1.DefaultOperationTimeoutSecs
	}
	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)
	c, err := h.clusterOperator.GetClusterEx(request.Request.Context(), name, "0")
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	extraMeta, err := h.getClusterMetadata(request.Request.Context(), c)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) || err == ErrNodesRegionDifferent {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	q := query.New()
	q.LabelSelector = fmt.Sprintf("%s=%s", common.LabelClusterName, c.Name)
	backups, err := h.clusterOperator.ListBackupEx(request.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	if backups.TotalCount > 0 {
		restplus.HandleInternalError(response, request, fmt.Errorf("before deleting the cluster, please delete the cluster backup file first"))
		return
	}

	op, err := h.parseOperationFromCluster(extraMeta, c, v1.ActionUninstall)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	op.Status.Status = v1.OperationStatusRunning
	op.Labels[common.LabelTimeoutSeconds] = timeoutSecs
	op.Labels[common.LabelOperationAction] = v1.OperationDeleteCluster
	if !dryRun {
		c.Status.Status = v1.ClusterStatusDeleting
		_, err = h.clusterOperator.UpdateCluster(request.Request.Context(), c)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		op, err = h.opOperator.CreateOperation(request.Request.Context(), op)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}
	go func(o *v1.Operation, opts *service.Options) {
		if err := h.delivery.DeliverTaskOperation(context.TODO(), o, opts); err != nil {
			logger.Error("delivery task error", zap.Error(err))
		}
	}(op, &service.Options{DryRun: dryRun})
	response.WriteHeader(http.StatusOK)
}

func (h *handler) CreateClusters(request *restful.Request, response *restful.Response) {
	c := v1.Cluster{}
	if err := request.ReadEntity(&c); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	if c.Labels[common.LabelBackupPoint] != "" {
		_, err := h.clusterOperator.GetBackupPointEx(request.Request.Context(), c.Labels[common.LabelBackupPoint], "0")
		if err != nil {
			if apimachineryErrors.IsNotFound(err) {
				restplus.HandleBadRequest(response, request, err)
				return
			}
			restplus.HandleInternalError(response, request, err)
			return
		}
	}

	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)
	timeoutSecs := v1.DefaultOperationTimeoutSecs
	if v := request.QueryParameter("timeout"); v != "" {
		timeoutSecs = v
	}

	// validate node exist
	extraMeta, err := h.getClusterMetadata(request.Request.Context(), &c)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) || err == ErrNodesRegionDifferent {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	if err := h.createClusterCheck(request.Request.Context(), &c); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	c.Complete()

	op, err := h.parseOperationFromCluster(extraMeta, &c, v1.ActionInstall)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}

	// TODO: make dry run path to etcd
	if !dryRun {
		c.Status.Status = v1.ClusterStatusInstalling
		_, err = h.clusterOperator.CreateCluster(context.TODO(), &c)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}

	op.Labels[common.LabelTimeoutSeconds] = timeoutSecs
	op.Labels[common.LabelOperationAction] = v1.OperationCreateCluster
	op.Status.Status = v1.OperationStatusRunning
	if !dryRun {
		op, err = h.opOperator.CreateOperation(context.TODO(), op)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}

	go h.doOperation(context.TODO(), op, &service.Options{DryRun: dryRun})
	_ = response.WriteHeaderAndEntity(http.StatusOK, c)
}

func (h *handler) UpdateClusters(request *restful.Request, response *restful.Response) {
	name := request.PathParameter("name")
	c := v1.Cluster{}
	if err := request.ReadEntity(&c); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)

	// TODO: vlidate update-cluster struct
	if ip, ok := c.Labels[common.LabelExternalIP]; ok {
		if !netutil.IsValidIP(ip) {
			restplus.HandleBadRequest(response, request, fmt.Errorf("external IP %s is valid", ip))
			return
		}
	}

	if !dryRun {
		clu, err := h.clusterOperator.GetCluster(context.TODO(), name)
		if err != nil {
			if apimachineryErrors.IsNotFound(err) {
				restplus.HandleNotFound(response, request, err)
				return
			}
			restplus.HandleInternalError(response, request, err)
			return
		}

		clu.Labels = c.Labels
		clu.Annotations = c.Annotations
		_, err = h.clusterOperator.UpdateCluster(context.TODO(), clu)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}

	response.WriteHeader(http.StatusOK)
}

func (h *handler) ListNodes(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	if q.Watch {
		h.watchNodes(request, response, q)
		return
	}
	if clientrest.IsInformerRawQuery(request.Request) {
		result, err := h.clusterOperator.ListNodes(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		// response.PrettyPrint(false)
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	} else {
		result, err := h.clusterOperator.ListNodesEx(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	}
}

func (h *handler) DescribeNode(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	resourceVersion := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	c, err := h.clusterOperator.GetNodeEx(request.Request.Context(), name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, c)
}

func (h *handler) DisableNode(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	ctx := request.Request.Context()

	resourceVersion := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	node, err := h.clusterOperator.GetNodeEx(ctx, name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	err = h.syncNodeDisable(node, true)
	if err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	updateNode, err := h.clusterOperator.UpdateNode(ctx, node)
	if err != nil {
		if apimachineryErrors.IsConflict(err) {
			restplus.HandleBadRequest(response, request, fmt.Errorf("the node %s has been modified; please apply "+
				"your changes to the latest version and try again", node.Name))
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, updateNode)
}

func (h *handler) EnableNode(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	ctx := request.Request.Context()
	resourceVersion := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	node, err := h.clusterOperator.GetNodeEx(ctx, name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	err = h.syncNodeDisable(node, false)
	if err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	updateNode, err := h.clusterOperator.UpdateNode(ctx, node)
	if err != nil {
		if apimachineryErrors.IsConflict(err) {
			restplus.HandleBadRequest(response, request, fmt.Errorf("the node %s has been modified; please apply "+
				"your changes to the latest version and try again", node.Name))
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, updateNode)
}

func (h *handler) syncNodeDisable(node *v1.Node, reqDisable bool) error {
	_, nodeDisable := node.Labels[common.LabelNodeDisable]
	if reqDisable == nodeDisable {
		return fmt.Errorf("the node %s is already disable:%v", node.Name, nodeDisable)
	}
	// 1.only can disable/enable idle node.
	_, inCluster := node.Labels[common.LabelNodeRole]
	if inCluster {
		return fmt.Errorf("node %s are in used,cannot disable/enable", node.Name)
	}
	// 2. cannot enable unknown state node.
	if !reqDisable {
		_, condition := controller.GetNodeCondition(&node.Status, v1.NodeReady)
		if condition != nil && condition.Status == v1.ConditionUnknown {
			return fmt.Errorf("node %s state are %s,cannot enable", node.Name, v1.ConditionUnknown)
		}
	}
	// 3.update node label
	if reqDisable {
		node.Labels[common.LabelNodeDisable] = "true"
	} else {
		delete(node.Labels, common.LabelNodeDisable)
	}

	return nil
}

// DeleteNode delete node record from etcd,only called by kcctl now.
func (h *handler) DeleteNode(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	resourceVersion := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	ctx := request.Request.Context()
	_, err := h.clusterOperator.GetNodeEx(ctx, name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	err = h.clusterOperator.DeleteNode(ctx, name)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusOK)
}

func (h *handler) watchNodes(req *restful.Request, resp *restful.Response, q *query.Query) {
	timeout := time.Duration(0)
	if q.TimeoutSeconds != nil {
		timeout = time.Duration(*q.TimeoutSeconds) * time.Second
	}
	if timeout == 0 {
		timeout = time.Duration(float64(query.MinTimeoutSeconds) * (rand.Float64() + 1.0))
	}

	watcher, err := h.clusterOperator.WatchNodes(req.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}
	restplus.ServeWatch(watcher, v1.SchemeGroupVersion.WithKind("Node"), req, resp, timeout)
}

func (h *handler) DescribeOperation(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	resourceVersion := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	c, err := h.opOperator.GetOperationEx(request.Request.Context(), name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, c)
}

func (h *handler) ListOperations(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	if q.Watch {
		h.watchOperations(request, response, q)
		return
	}
	if clientrest.IsInformerRawQuery(request.Request) {
		result, err := h.opOperator.ListOperations(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	} else {
		result, err := h.opOperator.ListOperationsEx(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	}
}

func (h *handler) watchOperations(req *restful.Request, resp *restful.Response, q *query.Query) {
	timeout := query.MinTimeoutSeconds * time.Second
	if q.TimeoutSeconds != nil {
		timeout = time.Duration(*q.TimeoutSeconds) * time.Second
	}
	watcher, err := h.opOperator.WatchOperations(req.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}
	restplus.ServeWatch(watcher, v1.SchemeGroupVersion.WithKind("Operation"), req, resp, timeout)
}

func (h *handler) getClusterMetadata(ctx context.Context, c *v1.Cluster) (*component.ExtraMetadata, error) {
	meta := &component.ExtraMetadata{
		ClusterName:   c.Name,
		Offline:       c.Kubeadm.Offline,
		LocalRegistry: c.Kubeadm.LocalRegistry,
		CRI:           c.Kubeadm.ContainerRuntime.Type.String(),
		KubeVersion:   c.Kubeadm.KubernetesVersion,
	}
	masters, err := h.getNodeInfo(ctx, c.Kubeadm.Masters)
	if err != nil {
		return nil, err
	}
	meta.Masters = append(meta.Masters, masters...)
	workers, err := h.getNodeInfo(ctx, c.Kubeadm.Workers)
	if err != nil {
		return nil, err
	}
	meta.Workers = append(meta.Workers, workers...)
	err = h.regionCheck(meta.Masters, meta.Workers)
	if err != nil {
		return nil, err
	}
	return meta, nil
}

func (h *handler) regionCheck(master, worker []component.Node) error {
	list := sets.NewString()
	for _, node := range master {
		list.Insert(node.Region)
	}
	for _, node := range worker {
		list.Insert(node.Region)
	}
	if list.Len() > 1 {
		return ErrNodesRegionDifferent
	}
	return nil
}

func (h *handler) getNodeInfo(ctx context.Context, nodes v1.WorkerNodeList) ([]component.Node, error) {
	var meta []component.Node
	for _, node := range nodes {
		n, err := h.clusterOperator.GetNodeEx(ctx, node.ID, "0")
		if err != nil {
			return nil, err
		}
		item := component.Node{
			ID:       n.Name,
			IPv4:     n.Status.Ipv4DefaultIP,
			Region:   n.Labels[common.LabelTopologyRegion],
			Hostname: n.Labels[common.LabelHostname],
			Role:     n.Labels[common.LabelNodeRole],
		}
		_, item.Disable = n.Labels[common.LabelNodeDisable]
		meta = append(meta, item)
	}

	return meta, nil
}

func (h *handler) GetOperationLog(request *restful.Request, response *restful.Response) {
	ctx := request.Request.Context()
	resourceVer := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	nodeName := request.QueryParameter(query.ParameterNode)
	if nodeName == "" {
		restplus.HandleBadRequest(response, request, errors.New("node name is required"))
		return
	}
	_, err := h.clusterOperator.GetNodeEx(ctx, nodeName, resourceVer)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	opID := request.QueryParameter(query.ParameterOperation)
	if opID == "" {
		restplus.HandleBadRequest(response, request, errors.New("operation ID is required"))
		return
	}
	op, err := h.opOperator.GetOperationEx(ctx, opID, resourceVer)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	stepID := request.QueryParameter(query.ParameterStep)
	if stepID == "" {
		restplus.HandleBadRequest(response, request, errors.New("step ID is required"))
		return
	}
	// validate operation contains the step
	step, ok := op.GetStep(stepID)
	if !ok {
		restplus.HandleBadRequest(response, request, errors.New("step ID is invalid"))
		return
	}
	offset, err := strconv.ParseInt(strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterOffset)), 10, 64)
	if err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	stepKey := fmt.Sprintf("%s-%s", stepID, step.Name)
	req, err := json.Marshal(oplog.LogContentRequest{
		OpID:   opID,
		StepID: stepKey,
		Offset: offset,
		Length: 0, // callers are not currently supported to set the length of the fetch data
	})
	if err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	resp, err := h.delivery.DeliverLogRequest(ctx, &service.LogOperation{
		Op:                service.OperationStepLog,
		OperationIdentity: string(req),
		To:                nodeName,
	})
	if err != nil {
		logger.Error("request step log error", zap.Error(err))
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, StepLog{
		Content:      resp.Content,
		Node:         nodeName,
		Timeout:      step.Timeout,
		DeliverySize: resp.DeliverySize,
		LogSize:      resp.LogSize,
	})
}

func (h *handler) ListRegions(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	if q.Watch {
		h.watchRegions(request, response, q)
		return
	}
	if clientrest.IsInformerRawQuery(request.Request) {
		result, err := h.clusterOperator.ListRegions(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	} else {
		result, err := h.clusterOperator.ListRegionEx(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	}
}

func (h *handler) watchRegions(req *restful.Request, resp *restful.Response, q *query.Query) {
	timeout := time.Duration(0)
	if q.TimeoutSeconds != nil {
		timeout = time.Duration(*q.TimeoutSeconds) * time.Second
	}
	if timeout == 0 {
		timeout = time.Duration(float64(query.MinTimeoutSeconds) * (rand.Float64() + 1.0))
	}

	watcher, err := h.clusterOperator.WatchRegions(req.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}
	restplus.ServeWatch(watcher, v1.SchemeGroupVersion.WithKind("Region"), req, resp, timeout)
}

func (h *handler) DescribeRegion(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	resourceVersion := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	c, err := h.clusterOperator.GetRegionEx(request.Request.Context(), name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, c)
}

// doOperation should be called in goroutine.
func (h *handler) doOperation(ctx context.Context, op *v1.Operation, opts *service.Options) {
	if err := h.delivery.DeliverTaskOperation(ctx, op, opts); err != nil {
		logger.Error("distribute task error", zap.Error(err))
	}
}

func (h *handler) createClusterCheck(ctx context.Context, c *v1.Cluster) error {
	if len(c.Kubeadm.Masters) == 0 {
		return fmt.Errorf("cluster must have one master node")
	}

	cluInfo, err := h.clusterOperator.GetClusterEx(ctx, c.Name, "0")
	if err != nil && !apimachineryErrors.IsNotFound(err) {
		return err
	}
	if cluInfo != nil {
		return fmt.Errorf("cluster %s already exists", c.Name)
	}

	// check if the node is already in use
	nodeList, err := h.clusterOperator.ListNodes(ctx, &query.Query{
		Pagination:           query.NoPagination(),
		ResourceVersion:      "0",
		Watch:                false,
		LabelSelector:        fmt.Sprintf("!%s,!%s", common.LabelNodeRole, common.LabelNodeDisable),
		ResourceVersionMatch: query.ResourceVersionMatchNotOlderThan,
	})
	if err != nil {
		return fmt.Errorf("failed to list node: %s", err.Error())
	}

	freeNodes := sets.NewString()
	for _, node := range nodeList.Items {
		freeNodes.Insert(node.Name)
	}
	cluNodes := sets.NewString()
	for _, node := range append(c.Kubeadm.Masters, c.Kubeadm.Workers...) {
		cluNodes.Insert(node.ID)
	}

	if freeNodes.HasAll(cluNodes.List()...) {
		return nil
	}

	return fmt.Errorf("some nodes in used or disabled")
}

func (h *handler) ListBackupsWithCluster(request *restful.Request, response *restful.Response) {
	// cluster name in path
	clusterName := request.PathParameter("name")
	ctx := request.Request.Context()
	cluster, err := h.clusterOperator.GetClusterEx(ctx, clusterName, "0")
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	q := query.ParseQueryParameter(request)
	labels := []string{fmt.Sprintf("%s=%s", common.LabelClusterName, cluster.Name)} // always select by cluster name
	q.LabelSelector = strings.Join(labels, ",")
	result, err := h.clusterOperator.ListBackupEx(ctx, q)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}

	_ = response.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h *handler) ListBackups(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	if q.Watch {
		h.watchBackups(request, response, q)
		return
	}
	if clientrest.IsInformerRawQuery(request.Request) {
		result, err := h.clusterOperator.ListBackups(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	} else {
		result, err := h.clusterOperator.ListBackupEx(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	}
}

func (h *handler) DescribeBackup(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	resourceVersion := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	c, err := h.clusterOperator.GetBackupEx(request.Request.Context(), name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, c)
}

func (h *handler) watchBackups(req *restful.Request, resp *restful.Response, q *query.Query) {
	timeout := time.Duration(0)
	if q.TimeoutSeconds != nil {
		timeout = time.Duration(*q.TimeoutSeconds) * time.Second
	}
	if timeout == 0 {
		timeout = time.Duration(float64(query.MinTimeoutSeconds) * (rand.Float64() + 1.0))
	}

	watcher, err := h.clusterOperator.WatchBackups(req.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}
	restplus.ServeWatch(watcher, v1.SchemeGroupVersion.WithKind("Backup"), req, resp, timeout)
}

func (h *handler) CreateBackup(request *restful.Request, response *restful.Response) {
	backup := &v1.Backup{}
	if err := request.ReadEntity(backup); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)
	// cluster name in path
	clusterName := request.PathParameter("name")
	ctx := request.Request.Context()
	c, err := h.clusterOperator.GetClusterEx(ctx, clusterName, "0")
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	if c.Status.Status != v1.ClusterStatusRunning {
		restplus.HandleInternalError(response, request, fmt.Errorf("cluster %s current is %s, can't back up",
			c.Name, c.Status.Status))
		return
	}

	b, err := h.clusterOperator.GetBackup(ctx, clusterName, backup.Name)
	if err != nil && !apimachineryErrors.IsNotFound(err) {
		restplus.HandleInternalError(response, request, err)
		return
	}
	if b != nil {
		restplus.HandleInternalError(response, request, fmt.Errorf("backup %s already exists", backup.Name))
		return
	}

	// check the backup point exist or not
	if c.Labels[common.LabelBackupPoint] == "" {
		restplus.HandleBadRequest(response, request, fmt.Errorf("no backup point specified"))
		return
	}

	_, err = h.clusterOperator.GetBackupPoint(ctx, c.Labels[common.LabelBackupPoint], "0")
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleBadRequest(response, request, fmt.Errorf("backup point not found"))
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	backup.Name = fmt.Sprintf("%s-%s", backup.Name, clusterName)
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
		restplus.HandleBadRequest(response, request, fmt.Errorf("the node %s not a master node", backup.PreferredNode))
		return
	}

	q := query.New()
	q.LabelSelector = fmt.Sprintf("%s=%s", common.LabelClusterName, c.Name)
	nodeList, err := h.clusterOperator.ListNodes(context.TODO(), q)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}

	if backup.PreferredNode != "" {
		_, err := h.clusterOperator.GetNodeEx(ctx, backup.PreferredNode, "0")
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}
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
	backup.Labels[common.LabelTimeoutSeconds] = strconv.Itoa(v1.DefaultBackupTimeoutSec)
	backup.ClusterNodes = make(map[string]string)
	for _, node := range nodeList.Items {
		backup.ClusterNodes[node.Status.Ipv4DefaultIP] = node.Status.NodeInfo.Hostname
	}

	if !dryRun {
		op.Steps, err = h.parseActBackupSteps(c, backup, v1.ActionInstall)
		if err != nil {
			logger.Errorf("parse create backup step failed: %s", err.Error())
			restplus.HandleInternalError(response, request, err)
			return
		}
		if op, err = h.opOperator.CreateOperation(context.TODO(), op); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		if backup, err = h.clusterOperator.CreateBackup(context.TODO(), backup); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		// update cluster status to backing_up
		c.Status.Status = v1.ClusterStatusBackingUp
		if _, err = h.clusterOperator.UpdateCluster(context.TODO(), c); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		go h.doOperation(context.TODO(), op, &service.Options{DryRun: dryRun})
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, backup)
}

func (h *handler) DeleteBackup(request *restful.Request, response *restful.Response) {
	// cluster name in path
	clusterName := request.PathParameter("cluster")
	ctx := request.Request.Context()
	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)
	c, err := h.clusterOperator.GetClusterEx(ctx, clusterName, "0")
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	// backup name in path
	backupName := request.PathParameter("backup")
	b, err := h.clusterOperator.GetBackupEx(ctx, clusterName, backupName)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	if b.PreferredNode != "" {
		_, err = h.clusterOperator.GetNodeEx(ctx, b.PreferredNode, "0")
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}
	// create operation
	op := &v1.Operation{}
	op.Name = uuid.New().String()
	op.Labels = make(map[string]string)
	op.Labels[common.LabelOperationAction] = v1.OperationBackupCluster
	op.Labels[common.LabelTimeoutSeconds] = strconv.Itoa(v1.DefaultBackupTimeoutSec)
	op.Labels[common.LabelClusterName] = c.Name
	op.Labels[common.LabelBackupName] = b.Name
	op.Labels[common.LabelTopologyRegion] = c.Kubeadm.Masters[0].Labels[common.LabelTopologyRegion]
	op.Status.Status = v1.OperationStatusRunning

	if b.Status.ClusterBackupStatus == v1.ClusterBackupRestoring || b.Status.ClusterBackupStatus == v1.ClusterBackupCreating {
		restplus.HandleBadRequest(response, request, fmt.Errorf("backup is %s now, can't delete", b.Status.ClusterBackupStatus))
		return
	}

	if !dryRun {
		c, err = h.clusterOperator.GetCluster(ctx, clusterName)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		// build the backup steps instance
		op.Steps, err = h.parseActBackupSteps(c, b, v1.ActionUninstall)
		if err != nil {
			logger.Errorf("delete backup step parse failed: %s", err.Error())
			restplus.HandleInternalError(response, request, err)
			return
		}
		if op, err = h.opOperator.CreateOperation(context.TODO(), op); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		if err = h.clusterOperator.DeleteBackup(context.TODO(), backupName); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		go h.doOperation(context.TODO(), op, &service.Options{DryRun: dryRun})
	}
	response.WriteHeader(http.StatusOK)
}

func (h *handler) UpdateBackup(request *restful.Request, response *restful.Response) {
	b := &v1.Backup{}
	if err := request.ReadEntity(b); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	// cluster name in path
	clusterName := request.PathParameter("cluster")
	ctx := request.Request.Context()
	if _, err := h.clusterOperator.GetClusterEx(ctx, clusterName, "0"); err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	// backup name in path
	backupName := request.PathParameter("backup")
	backup, err := h.clusterOperator.GetBackupEx(ctx, clusterName, backupName)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	// only description can be modified
	backup.Annotations[common.AnnotationDescription] = b.Annotations[common.AnnotationDescription]

	backup, err = h.clusterOperator.UpdateBackup(ctx, backup)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, backup)
}

func (h *handler) RetryCluster(request *restful.Request, response *restful.Response) {
	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)
	name := request.PathParameter(query.ParameterName)

	op, err := h.opOperator.GetOperationEx(request.Request.Context(), name, "0")
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}

	// backup/recovery/upgrade does not support retries
	switch op.Labels[common.LabelOperationAction] {
	case v1.OperationBackupCluster, v1.OperationRecoverCluster, v1.OperationUpgradeCluster:
		restplus.HandleBadRequest(response, request, fmt.Errorf("backup/recovery/upgrade operation-action does not support retries"))
		return
	case "":
		restplus.HandleBadRequest(response, request, fmt.Errorf("operation %s action is empty", name))
		return
	}

	// only the last retry is supported
	q := query.New()
	q.LabelSelector = fmt.Sprintf("%s=%s", common.LabelClusterName, op.Labels[common.LabelClusterName])
	q.Pagination.Offset = 0
	q.Pagination.Limit = 1
	opList, err := h.opOperator.ListOperationsEx(request.Request.Context(), q)
	if err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	if len(opList.Items) == 0 {
		restplus.HandleBadRequest(response, request, fmt.Errorf("the cluster did not query the operation"))
		return
	}

	op = opList.Items[0].(*v1.Operation)
	if op.Status.Status == v1.OperationStatusSuccessful || op.Status.Status == v1.OperationStatusRunning || op.Name != name {
		restplus.HandleBadRequest(response, request, fmt.Errorf("only the latest faild operation can do a retry"))
		return
	}

	// error step index
	failedIndex := len(op.Status.Conditions) - 1
	ctx := component.WithRetry(context.TODO(), true)

	// if there is an uninstall error, continue directly from the current step
	var continueSteps []v1.Step
	if op.Steps[0].Action == v1.ActionInstall {
		findStepNode := func(nodes []v1.StepNode, nodeID string) v1.StepNode {
			for _, v := range nodes {
				if v.ID == nodeID {
					return v
				}
			}
			return v1.StepNode{}
		}
		var failedNodes []v1.StepNode
		successStatus := make([]v1.StepStatus, 0)
		for _, status := range op.Status.Conditions[failedIndex].Status {
			if status.Status == v1.StepStatusFailed {
				// select the nodes whose execution fails
				if node := findStepNode(op.Steps[0].Nodes, status.Node); node.ID != "" {
					failedNodes = append(failedNodes, node)
				}
				continue
			}
			successStatus = append(successStatus, status)
		}
		// the step to continue
		continueSteps = op.Steps[failedIndex:]

		// the node that failed to execute the task
		continueSteps[0].Nodes = failedNodes

		if len(successStatus) != 0 {
			// failed status
			failedStepStatus := op.Status.Conditions[failedIndex]
			// retain successful status
			failedStepStatus.Status = successStatus
			// Remove the failed status and keep the successful status
			op.Status.Conditions = op.Status.Conditions[0:failedIndex]
			op.Status.Conditions = append(op.Status.Conditions, failedStepStatus)
		} else {
			op.Status.Conditions = op.Status.Conditions[0:failedIndex]
		}
		if failedIndex > 0 && op.Status.Conditions[failedIndex-1].Status[0].Response != nil {
			ctx = component.WithExtraData(ctx, op.Status.Conditions[failedIndex-1].Status[0].Response)
		}
	}

	// if there is an uninstall error, start from the beginning
	if op.Steps[0].Action == v1.ActionUninstall {
		continueSteps = op.Steps
		op.Status.Conditions = make([]v1.OperationCondition, 0)
	}

	op.Status.Status = v1.OperationStatusRunning

	if !dryRun {
		_, err = h.opOperator.UpdateOperation(context.TODO(), op)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		var c *v1.Cluster
		if c, err = h.clusterOperator.GetClusterEx(context.TODO(), op.Labels[common.LabelClusterName], "0"); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		c.Status.Status = v1.ClusterStatusUpdating
		if _, err = h.clusterOperator.UpdateCluster(context.TODO(), c); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}

	op.Steps = continueSteps

	go h.doOperation(ctx, op, &service.Options{DryRun: dryRun})
	_ = response.WriteHeaderAndEntity(http.StatusOK, nil)
}

func (h *handler) CreateRecovery(request *restful.Request, response *restful.Response) {
	r := &v1.Recovery{}
	if err := request.ReadEntity(r); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)
	// cluster name in path
	clusterName := request.PathParameter("cluster")
	ctx := request.Request.Context()

	c, err := h.clusterOperator.GetClusterEx(ctx, clusterName, "0")
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	q := query.New()
	q.LabelSelector = fmt.Sprintf("%s=%s", common.LabelClusterName, c.Name)
	nodeList, err := h.clusterOperator.ListNodes(context.TODO(), q)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}

	b, err := h.clusterOperator.GetBackup(ctx, clusterName, r.UseBackupName)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	for _, node := range nodeList.Items {
		val, ok := b.ClusterNodes[node.Status.Ipv4DefaultIP]
		if !ok {
			restplus.HandleInternalError(response, request,
				fmt.Errorf("the current node IP(%s) is not included in the backup file information(%v)",
					node.Status.Ipv4DefaultIP, b.ClusterNodes))
			return
		}
		if val != node.Status.NodeInfo.Hostname {
			restplus.HandleInternalError(response, request,
				fmt.Errorf("the current node Hostname(%s) is not included in the backup file information(%v)",
					node.Status.NodeInfo.Hostname, b.ClusterNodes))
			return
		}
	}

	switch c.Status.Status {
	case v1.ClusterStatusRestoring, v1.ClusterStatusBackingUp, v1.ClusterStatusDeleting, v1.ClusterStatusUpdating, v1.ClusterStatusInstalling:
		restplus.HandleInternalError(response, request, fmt.Errorf("cluster %s current is %s, can't recovery",
			c.Name, c.Status.Conditions[0].OperationStatus))
		return
	}

	rName := uuid.New().String()
	oName := uuid.New().String()

	// update cluster status to recovering
	c.Status.Status = v1.ClusterStatusRestoring

	// create operation
	o := &v1.Operation{}
	o.Name = oName
	o.Labels = make(map[string]string)
	o.Labels[common.LabelOperationAction] = v1.OperationRecoverCluster
	o.Labels[common.LabelTimeoutSeconds] = strconv.Itoa(v1.DefaultRecoveryTimeoutSec)
	o.Labels[common.LabelClusterName] = c.Name
	o.Labels[common.LabelRecoveryName] = rName
	o.Labels[common.LabelTopologyRegion] = c.Kubeadm.Masters[0].Labels[common.LabelTopologyRegion]
	o.Status.Status = v1.OperationStatusRunning

	// add recovery
	r.Name = rName
	r.Labels = make(map[string]string)
	r.Labels[common.LabelClusterName] = c.Name
	r.Labels[common.LabelOperationName] = oName
	r.Labels[common.LabelTimeoutSeconds] = strconv.Itoa(v1.DefaultBackupTimeoutSec)

	if !dryRun {
		if c, err = h.clusterOperator.UpdateCluster(ctx, c); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}

	go func(c *v1.Cluster, op *v1.Operation, r *v1.Recovery, b *v1.Backup) {
		var err error
		restoreDir := filepath.Join("/var/lib/kube-restore", c.Name)
		steps, err := h.parseRecoverySteps(c, b, restoreDir, v1.ActionInstall)
		if err != nil {
			logger.Errorf("recovery step parse failed: %s", err.Error())
			return
		}
		op.Steps = steps
		newOP, err := h.opOperator.CreateOperation(context.TODO(), op)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		// update backup operation label
		b.Labels[common.LabelOperationName] = oName
		if _, err = h.clusterOperator.UpdateBackup(context.TODO(), b); err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		h.doOperation(context.TODO(), newOP, &service.Options{DryRun: dryRun})
	}(c, o, r, b)

	_ = response.WriteHeaderAndEntity(http.StatusOK, r)
}

func (h *handler) InstallOrUninstallPlugins(request *restful.Request, response *restful.Response) {
	pcs := &PatchComponents{}
	if err := request.ReadEntity(pcs); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	clusterName := request.PathParameter("cluster")
	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)
	timeoutSecs := v1.DefaultOperationTimeoutSecs
	if v := request.QueryParameter("timeout"); v != "" {
		timeoutSecs = v
	}

	ctx := request.Request.Context()
	clu, err := h.clusterOperator.GetClusterEx(ctx, clusterName, "0")
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	// We need IP addresses of all master nodes later.
	extraMeta, err := h.getClusterMetadata(ctx, clu)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) || err == ErrNodesRegionDifferent {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	if err = pcs.checkComponents(clu); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	action, operationAction := v1.ActionInstall, v1.OperationInstallComponents
	if pcs.Uninstall {
		action = v1.ActionUninstall
		operationAction = v1.OperationUninstallComponents
	}
	op, err := h.parseOperationFromComponent(extraMeta, pcs.Components, clu, action)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	// The current component does not support uninstallation, so the steps will be empty
	if len(op.Steps) == 0 {
		restplus.HandleBadRequest(response, request, errors.New("the current operation steps is empty and cannot be performed"))
		return
	}
	if !dryRun {
		clu.Status.Status = v1.ClusterStatusUpdating
		_, err = h.clusterOperator.UpdateCluster(context.TODO(), clu)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}
	op.Labels[common.LabelTimeoutSeconds] = timeoutSecs
	op.Labels[common.LabelOperationAction] = operationAction
	op.Status.Status = v1.OperationStatusRunning
	if !dryRun {
		op, err = h.opOperator.CreateOperation(context.TODO(), op)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}
	go func(o *v1.Operation, opts *service.Options, oPcs *PatchComponents) {
		if err := h.delivery.DeliverTaskOperation(context.TODO(), o, opts); err != nil {
			logger.Error("delivery task error", zap.Error(err))
			return
		}
		logger.Debugf("the install or uninstall plugins message was delivered successfully")
		// the database is not updated until the message is delivered successfully
		if !opts.DryRun {
			latestCluster, err := h.clusterOperator.GetClusterEx(ctx, clusterName, "0")
			if err != nil {
				logger.Error("get the latest cluster info error", zap.Error(err))
				return
			}
			newCluster, err := oPcs.addOrRemoveComponentFromCluster(latestCluster)
			if err != nil {
				logger.Error("add or remove component from cluster", zap.Error(err))
				return
			}
			_, err = h.clusterOperator.UpdateCluster(context.TODO(), newCluster)
			if err != nil {
				logger.Error("update cluster metadata error", zap.Error(err))
			}
		}
	}(op, &service.Options{DryRun: dryRun}, pcs)

	_ = response.WriteHeaderAndEntity(http.StatusOK, clu)
}

func (h *handler) UpgradeCluster(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	body := &ClusterUpgrade{}
	if err := request.ReadEntity(body); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	clu, err := h.clusterOperator.GetClusterEx(request.Request.Context(), name, "0")
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)
	// TODO: validate cluster struct
	timeoutSecs := v1.DefaultOperationTimeoutSecs
	if v := request.QueryParameter("timeout"); v != "" {
		timeoutSecs = v
	}
	extraMeta, err := h.getClusterMetadata(request.Request.Context(), clu)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) || err == ErrNodesRegionDifferent {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	extraMeta.Offline = body.Offline
	extraMeta.KubeVersion = body.Version
	extraMeta.LocalRegistry = body.LocalRegistry
	upgradeComp := &k8s.Upgrade{}
	upgradeComp.InitStepper(extraMeta, clu.Kubeadm)
	if err := upgradeComp.Validate(); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	if err := upgradeComp.InitSteps(component.WithExtraMetadata(context.TODO(), *extraMeta)); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	op := &v1.Operation{}
	op.Name = uuid.New().String()
	op.Labels = map[string]string{
		common.LabelClusterName:    clu.Name,
		common.LabelTopologyRegion: extraMeta.Masters[0].Region,
	}
	op.Steps = upgradeComp.GetInstallSteps()

	// TODO: make dry run path to etcd
	if !dryRun {
		clu.Status.Status = v1.ClusterStatusUpgrading
		_, err = h.clusterOperator.UpdateCluster(request.Request.Context(), clu)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}

	op.Labels[common.LabelTimeoutSeconds] = timeoutSecs
	op.Labels[common.LabelOperationAction] = v1.OperationUpgradeCluster
	op.Labels[common.LabelUpgradeVersion] = body.Version
	op.Status.Status = v1.OperationStatusRunning
	if !dryRun {
		op, err = h.opOperator.CreateOperation(context.TODO(), op)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
	}
	go h.doOperation(context.TODO(), op, &service.Options{DryRun: dryRun})
	response.WriteHeader(http.StatusOK)
}

func (h *handler) ResetClusterStatus(request *restful.Request, response *restful.Response) {
	dryRun := query.GetBoolValueWithDefault(request, query.ParamDryRun, false)
	cluName := request.PathParameter(query.ParameterName)
	clu, err := h.clusterOperator.GetClusterEx(request.Request.Context(), cluName, "0")
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	switch clu.Status.Status {
	case v1.ClusterStatusUpdateFailed, v1.ClusterStatusInstallFailed,
		v1.ClusterStatusUpgradeFailed, v1.ClusterStatusDeleteFailed,
		v1.ClusterStatusUpgrading, v1.ClusterStatusUpdating,
		v1.ClusterStatusBackingUp, v1.ClusterStatusRestoring, v1.ClusterStatusRestoreFailed:
		if dryRun {
			response.WriteHeader(http.StatusOK)
			return
		}
		clu.Status.Status = v1.ClusterStatusRunning
		_, err := h.clusterOperator.UpdateCluster(request.Request.Context(), clu)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		response.WriteHeader(http.StatusOK)
	default:
		restplus.HandleBadRequest(response, request, fmt.Errorf("not supported reset %s status", clu.Status.Status))
		return
	}
}

func (h *handler) ListLeases(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	if q.Watch {
		h.watchLeases(request, response, q)
		return
	}
	if clientrest.IsInformerRawQuery(request.Request) {
		result, err := h.leaseOperator.ListLeases(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
		return
	}
	response.WriteHeader(http.StatusOK)
}

func (h *handler) watchLeases(req *restful.Request, resp *restful.Response, q *query.Query) {
	timeout := time.Duration(0)
	if q.TimeoutSeconds != nil {
		timeout = time.Duration(*q.TimeoutSeconds) * time.Second
	}
	if timeout == 0 {
		timeout = time.Duration(float64(query.MinTimeoutSeconds) * (rand.Float64() + 1.0))
	}

	watcher, err := h.leaseOperator.WatchLease(req.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}
	restplus.ServeWatch(watcher, v1.SchemeGroupVersion.WithKind("Lease"), req, resp, timeout)
}

func (h *handler) DescribeLease(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	resourceVersion := request.QueryParameter(query.ParameterResourceVersion)
	c, err := h.leaseOperator.GetLease(request.Request.Context(), name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, c)
}

// ---- domain

func (h *handler) ListDomains(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	if q.Watch {
		h.watchDomain(request, response, q)
		return
	}

	if clientrest.IsInformerRawQuery(request.Request) {
		result, err := h.clusterOperator.ListDomains(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
		return
	}
	result, err := h.clusterOperator.ListDomainsEx(request.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h *handler) watchDomain(req *restful.Request, resp *restful.Response, q *query.Query) {
	timeout := time.Duration(0)
	if q.TimeoutSeconds != nil {
		timeout = time.Duration(*q.TimeoutSeconds) * time.Second
	}
	if timeout == 0 {
		timeout = time.Duration(float64(query.MinTimeoutSeconds) * (rand.Float64() + 1.0))
	}

	watcher, err := h.clusterOperator.WatchDomain(req.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}
	restplus.ServeWatch(watcher, v1.SchemeGroupVersion.WithKind("Domain"), req, resp, timeout)
}

func (h *handler) CheckDomainExists(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	result, err := h.clusterOperator.ListDomainsEx(request.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}

	if len(result.Items) > 0 {
		response.Header().Set(resourceExistCheckerHeader, "true")
		response.WriteHeader(http.StatusOK)
		return
	}

	// parse domain from selector ,e.g. metadata.name=github.com
	spilt := strings.Split(q.FieldSelector, "=")
	if len(spilt) != 2 {
		restplus.HandleBadRequest(response, request, err)
		response.WriteHeader(http.StatusOK)
		return
	}
	checkDomain := spilt[1]

	exists, err := h.checkDomain(request.Request.Context(), checkDomain)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	if exists {
		response.Header().Set(resourceExistCheckerHeader, "true")
		return
	}
	response.Header().Set(resourceExistCheckerHeader, "false")
	response.WriteHeader(http.StatusOK)
}

func (h *handler) checkDomain(ctx context.Context, checkDomain string) (bool, error) {
	checkDomain = strings.ToLower(checkDomain)
	domains, err := h.clusterOperator.ListDomains(ctx, &query.Query{})
	if err != nil {
		return false, err
	}
	for _, domain := range domains.Items {
		domain.Name = strings.ToLower(domain.Name)
		// check a.example.com, exists example.com return true
		// check example.com, exists a.example.com return true
		if strings.Contains(checkDomain, "."+domain.Name) || strings.Contains(domain.Name, "."+checkDomain) {
			return true, nil
		}
	}
	return false, nil
}

func (h *handler) GetDomain(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	result, err := h.clusterOperator.GetDomain(request.Request.Context(), name)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h *handler) CreateDomains(request *restful.Request, response *restful.Response) {
	d := new(v1.Domain)
	if err := request.ReadEntity(d); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	if errs := validation.ValidateDomain(d); len(errs) > 0 {
		restplus.HandleBadRequest(response, request, errs.ToAggregate())
		return
	}

	exists, err := h.checkDomain(request.Request.Context(), d.Name)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	err = h.checkSyncCluster(d.Spec.SyncCluster)
	if err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	if exists {
		restplus.HandleBadRequest(response, request, fmt.Errorf("domain %s is already exist", d.Name))
		return
	}

	createDomain, err := h.clusterOperator.CreateDomain(request.Request.Context(), d)
	if err != nil {
		if apimachineryErrors.IsAlreadyExists(err) {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, createDomain)
}

func (h *handler) checkSyncCluster(syncCluster []string) error {
	clusters, err := h.clusterOperator.ListClusters(context.Background(), &query.Query{
		ResourceVersion:      "0",
		ResourceVersionMatch: query.ResourceVersionMatchNotOlderThan,
	})
	if err != nil {
		return err
	}
	m := make(map[string]struct{}, len(clusters.Items))
	for _, item := range clusters.Items {
		m[item.Name] = struct{}{}
	}

	for _, clusterName := range syncCluster {
		_, ok := m[clusterName]
		if !ok {
			return fmt.Errorf("cluster %s is not exist", clusterName)
		}
	}
	return nil
}

func (h *handler) UpdateDomain(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	d := new(v1.Domain)
	if err := request.ReadEntity(d); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	if name != d.Name {
		restplus.HandleBadRequest(response, request, fmt.Errorf("the name of the object (%s) does not match "+
			"the name on the URL (%s)", d.Name, name))
		return
	}

	err := h.checkSyncCluster(d.Spec.SyncCluster)
	if err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	updated, err := h.updateDomain(request.Request.Context(), d)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, updated)
}

func (h *handler) updateDomain(ctx context.Context, domain *v1.Domain) (*v1.Domain, error) {
	d, err := h.clusterOperator.GetDomain(ctx, domain.Name)
	if err != nil {
		return nil, err
	}
	d.Spec.Description = domain.Spec.Description
	d.Spec.SyncCluster = domain.Spec.SyncCluster
	return h.clusterOperator.UpdateDomain(ctx, d)
}

func (h *handler) DeleteDomain(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)

	_, err := h.clusterOperator.GetDomain(request.Request.Context(), name)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			logger.Debug("domain has already not exist when delete", zap.String("domain", name))
			response.WriteHeader(http.StatusOK)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}

	err = h.clusterOperator.DeleteDomain(request.Request.Context(), name)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			logger.Debug("domain has already not exist when delete", zap.String("domain", name))
			response.WriteHeader(http.StatusOK)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusOK)
}

// ------- record

func (h *handler) ListRecords(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	subdomain := request.QueryParameter(query.ParameterSubDomain)
	q := query.ParseQueryParameter(request)

	result, err := h.clusterOperator.ListRecordsEx(request.Request.Context(), name, subdomain, q)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h *handler) GetRecord(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	subdomain := request.PathParameter(query.ParameterSubDomain)
	var (
		result *v1.Domain
		err    error
	)
	result, err = h.clusterOperator.GetDomain(request.Request.Context(), name)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	record, ok := result.Spec.Records[subdomain]
	if !ok {
		restplus.HandleNotFound(response, request, apimachineryErrors.NewNotFound(v1.Resource("record"), subdomain))
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, record)
}

func (h *handler) CreateRecords(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)

	record := new(v1.Record)
	if err := request.ReadEntity(record); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	if name != record.Domain {
		restplus.HandleBadRequest(response, request, fmt.Errorf("the name of the object (%s) does not match "+
			"the name on the URL (%s)", name, record.Domain))
		return
	}
	err := checkRecord(record)
	if err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	getDomain, err := h.clusterOperator.GetDomain(request.Request.Context(), name)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	key := record.RR + "." + getDomain.Name
	_, ok := getDomain.Spec.Records[key]
	if ok {
		restplus.HandleBadRequest(response, request, fmt.Errorf("record %s already exists", key))
		return
	}
	// ignore lower upper case
	key = strings.ToLower(key)
	for k := range getDomain.Spec.Records {
		if key == strings.ToLower(k) {
			restplus.HandleBadRequest(response, request, fmt.Errorf("record %s already exists", k))
			return
		}
	}

	createRecord, err := h.updateRecord(request.Request.Context(), getDomain, *record)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, createRecord)
}

func checkRecord(r *v1.Record) error {
	if len(r.ParseRecord) == 0 {
		return fmt.Errorf("resolve record cann not be empty")
	}
	return nil
}

func (h *handler) UpdateRecord(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	record := new(v1.Record)
	if err := request.ReadEntity(record); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	if name != record.Domain {
		restplus.HandleBadRequest(response, request, fmt.Errorf("the name of the object (%s) does not match "+
			"the domain on the URL (%s)", record.Domain, name))
		return
	}

	err := checkRecord(record)
	if err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	getDomain, err := h.clusterOperator.GetDomain(request.Request.Context(), name)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	key := record.RR + "." + getDomain.Name
	if _, ok := getDomain.Spec.Records[key]; !ok {
		restplus.HandleNotFound(response, request, fmt.Errorf("record %s not found", key))
		return
	}

	updated, err := h.updateRecord(request.Request.Context(), getDomain, *record)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, updated)
}

func (h *handler) updateRecord(ctx context.Context, domain *v1.Domain, record v1.Record) (*v1.Domain, error) {
	if domain.Spec.Records == nil {
		domain.Spec.Records = make(map[string]v1.Record)
	}
	key := record.RR + "." + record.Domain
	//  新建时填充 createTime
	if _, ok := domain.Spec.Records[key]; !ok {
		record.CreateTime = metav1.NewTime(time.Now())
	}

	domain.Spec.Records[key] = record
	return h.clusterOperator.UpdateDomain(ctx, domain)
}

func (h *handler) DeleteRecord(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	subdomain := request.PathParameter(query.ParameterSubDomain)

	domain, err := h.clusterOperator.GetDomain(request.Request.Context(), name)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			logger.Debug("domain has already not exist when delete", zap.String("domain", name))
			response.WriteHeader(http.StatusOK)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_, ok := domain.Spec.Records[subdomain]
	if !ok {
		restplus.HandleNotFound(response, request, fmt.Errorf("record %s not found", subdomain))
		return
	}
	deleted, err := h.deleteRecord(request.Request.Context(), domain, subdomain)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, deleted)
}

func (h *handler) deleteRecord(ctx context.Context, domain *v1.Domain, subdomain string) (*v1.Domain, error) {
	d, err := h.clusterOperator.GetDomain(ctx, domain.Name)
	if err != nil {
		return nil, err
	}
	delete(d.Spec.Records, subdomain)
	return h.clusterOperator.UpdateDomain(ctx, d)
}

var (
	upGrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024 * 1024 * 10,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type SSHCredential struct {
	Username []byte `json:"username"`
	Password []byte `json:"password"`
	Port     int    `json:"port"`
}

func (h *handler) SSHToPod(request *restful.Request, response *restful.Response) {
	clusterName := request.PathParameter(query.ParameterName)
	clu, err := h.clusterOperator.GetCluster(context.TODO(), clusterName)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleBadRequest(response, request, fmt.Errorf("cluster %s not exists", clusterName))
			return
		}
		logger.Errorf("get cluster %s failed: %v", clusterName, err)
		restplus.HandleInternalError(response, request, err)
		return
	}
	if clu.KubeConfig == nil {
		restplus.HandleBadRequest(response, request, fmt.Errorf("cluster %s clientset not init", clusterName))
		return
	}

	clientcfg, clientset, err := client.FromKubeConfig(clu.KubeConfig)
	if err != nil {
		logger.Errorf("cluster %s generate clientset failed: %v", clusterName, err)
		restplus.HandleInternalError(response, request, err)
		return
	}

	// find kubectl pod by label
	list, err := clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "k8s-app=kc-kubectl",
	})
	if err != nil {
		logger.Errorf("kubectl console, find kubectl pod failed: %v", err)
		restplus.HandleInternalError(response, request, err)
		return
	}
	if len(list.Items) == 0 {
		restplus.HandleInternalError(response, request, errors.New("kubectl pod not exists"))
		logger.Errorf("kubectl console, kubectl pod not exists: %v", err)
		return
	}
	podName := list.Items[0].GetName()

	wsConn, err := upGrader.Upgrade(response.ResponseWriter, request.Request, nil)
	if err != nil {
		logger.Errorf("upgrade err: %v", err)
		return
	}
	session := &sshutils.TerminalSession{Conn: wsConn, SizeChan: make(chan remotecommand.TerminalSize)}
	t := sshutils.NewTerminaler(clientset, clientcfg)
	err = t.StartProcess("kube-system", podName, "kc-kubectl", []string{"sh"}, session)
	if err != nil {
		logger.Errorf("start process err: %v", err)
		session.Close(2, err.Error())
		return
	}
	session.Close(1, "Process exited")
}

func (h *handler) SSHToNode(request *restful.Request, response *restful.Response) {
	var (
		priKey           []byte
		setting          = &v1.PlatformSetting{}
		quitChan         = make(chan struct{}, 3)
		gracefulExitChan = make(chan struct{})
	)
	nodeName := request.PathParameter(query.ParameterName)
	msg := request.QueryParameter("msg")
	cols := query.GetIntValueWithDefault(request, ParameterCols, 150)
	rows := query.GetIntValueWithDefault(request, ParameterRows, 35)

	wsConn, err := upGrader.Upgrade(response.ResponseWriter, request.Request, nil)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	defer wsConn.Close()

	err = wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
	if err != nil {
		logger.Errorf("%s", err)
	}
	wsConn.SetPongHandler(func(appData string) error {
		logger.Debug("in pong handler...")
		err = wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
		if err != nil {
			logger.Errorf("%s", err)
		}
		return nil
	})
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	go func() {
		defer logger.Debug("websocket send ping return...")
		for {
			select {
			case <-ticker.C:
				// wsConn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-quitChan:
				return
			}
		}
	}()
	wsConn.SetCloseHandler(func(code int, text string) error {
		logger.Debug("in close handler...")
		message := websocket.FormatCloseMessage(code, text)
		err = wsConn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
		if err != nil {
			logger.Errorf("%s", err)
		}
		quitChan <- struct{}{}
		return nil
	})

	node, err := h.clusterOperator.GetNodeEx(request.Request.Context(), nodeName, "0")
	if err != nil {
		_ = wsConn.CloseHandler()(4000, "BadRequest: parameter error")
		return
	}
	credential, err := decodeMsgToSSH(msg)
	if err != nil {
		_ = wsConn.CloseHandler()(4000, "BadRequest: parameter error")
		return
	}
	setting, err = h.platformOperator.GetPlatformSetting(request.Request.Context())
	if err != nil {
		_ = wsConn.CloseHandler()(4000, "BadRequest: parameter error")
		return
	}
	if setting == nil {
		_ = wsConn.CloseHandler()(4000, "BadRequest: parameter error")
		return
	}
	if setting.Terminal.PrivateKey != "" {
		priKey, err = base64.StdEncoding.DecodeString(setting.Terminal.PrivateKey)
		if err != nil {
			_ = wsConn.CloseHandler()(4000, "BadRequest: parameter error")
			return
		}
	} else {
		_ = wsConn.CloseHandler()(4000, "BadRequest: parameter error")
		return
	}

	credential.Username, err = certs.RsaDecrypt(credential.Username, priKey)
	if err != nil {
		_ = wsConn.CloseHandler()(4000, "BadRequest: parameter error")
		return
	}
	credential.Password, err = certs.RsaDecrypt(credential.Password, priKey)
	if err != nil {
		_ = wsConn.CloseHandler()(4000, "BadRequest: parameter error")
		return
	}
	sshClient, err := sshutils.NewSSHClient(string(credential.Username), string(credential.Password), node.Status.Ipv4DefaultIP, credential.Port)
	if err != nil {
		_ = wsConn.CloseHandler()(4001, "username or password incorrect")
		return
	}
	defer sshClient.Close()

	sshConn, err := sshutils.NewLoginSSHWSSession(cols, rows, true, sshClient, wsConn)
	if err != nil {
		_ = wsConn.CloseHandler()(4002, "ssh connection failed")
		return
	}
	defer sshConn.Close()

	sshConn.Start(quitChan)
	go sshConn.Wait(quitChan, gracefulExitChan)
	select {
	case <-quitChan:
		return
	case <-gracefulExitChan:
		time.Sleep(1 * time.Second)
		message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		if err := wsConn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second)); err != nil {
			logger.Errorf("close websocket error due to: %s", err.Error())
		}
		return
	}
}

func decodeMsgToSSH(msg string) (*SSHCredential, error) {
	c := &SSHCredential{}
	decoded, err := base64.StdEncoding.DecodeString(msg)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(decoded, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (h *handler) ListTemplates(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	templates, err := h.clusterOperator.ListTemplatesEx(request.Request.Context(), q)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, templates)
}

func (h *handler) DescribeTemplate(request *restful.Request, response *restful.Response) {
	templateName := request.PathParameter(query.ParameterName)
	template, err := h.clusterOperator.GetTemplateEx(request.Request.Context(), templateName, "0")
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, template)
}

func (h *handler) CreateTemplate(request *restful.Request, response *restful.Response) {
	template := &v1.Template{}
	err := request.ReadEntity(template)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	template.ObjectMeta.GenerateName = "tmpl-"
	template, err = h.clusterOperator.CreateTemplate(request.Request.Context(), template)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusCreated, template)
}

func (h *handler) UpdateTemplate(request *restful.Request, response *restful.Response) {
	template := &v1.Template{}
	err := request.ReadEntity(template)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	templateName := request.PathParameter(query.ParameterName)
	if templateName != template.Name {
		restplus.HandleBadRequest(response, request, fmt.Errorf("template name not match"))
		return
	}
	template, err = h.clusterOperator.UpdateTemplate(request.Request.Context(), template)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, template)
}

func (h *handler) DeleteTemplate(request *restful.Request, response *restful.Response) {
	templateName := request.PathParameter(query.ParameterName)
	err := h.clusterOperator.DeleteTemplate(request.Request.Context(), templateName)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusOK)
}

func (h *handler) DescribeBackupPoint(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	resourceVersion := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	result, err := h.clusterOperator.GetBackupPointEx(request.Request.Context(), name, resourceVersion)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h *handler) ListBackupPoints(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	if clientrest.IsInformerRawQuery(request.Request) {
		result, err := h.clusterOperator.ListBackupPoints(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	} else {
		result, err := h.clusterOperator.ListBackupPointEx(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	}
}

func (h *handler) CreateBackupPoint(request *restful.Request, response *restful.Response) {
	bp := new(v1.BackupPoint)
	if err := request.ReadEntity(bp); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}

	bp.StorageType = strings.ToLower(bp.StorageType)
	if bp.StorageType == bs.S3Storage && len([]rune(bp.S3Config.Bucket)) <= 3 {
		restplus.HandleBadRequest(response, request, fmt.Errorf("bucket name cannot be shorter than 3 characters"))
		return
	}

	createdBp, err := h.clusterOperator.CreateBackupPoint(request.Request.Context(), bp)
	if err != nil {
		if apimachineryErrors.IsAlreadyExists(err) {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, createdBp)
}

func (h *handler) DeleteBackupPoint(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	err := h.clusterOperator.DeleteBackupPoint(request.Request.Context(), name)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			logger.Debug("backup point has already not exist when delete", zap.String("backupPoint", name))
			response.WriteHeader(http.StatusOK)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusOK)
}

func (h *handler) UpdateBackupPoint(req *restful.Request, resp *restful.Response) {
	bp := &v1.BackupPoint{}
	if err := req.ReadEntity(bp); err != nil {
		restplus.HandleBadRequest(resp, req, err)
		return
	}

	name := req.PathParameter(query.ParameterName)
	resourceVersion := strutil.StringDefaultIfEmpty("0", req.QueryParameter(query.ParameterResourceVersion))
	obp, err := h.clusterOperator.GetBackupPointEx(req.Request.Context(), name, resourceVersion)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}

	if bp.StorageType == bs.FSStorage && obp.StorageType == bs.FSStorage && bp.S3Config == nil {
		// only fs backup point description can be modified
		obp.FsConfig.Description = bp.FsConfig.Description
	}

	if bp.StorageType == bs.S3Storage && obp.StorageType == bs.S3Storage && bp.FsConfig == nil {
		obp.S3Config.AccessKeyID = bp.S3Config.AccessKeyID
		obp.S3Config.AccessKeySecret = bp.S3Config.AccessKeySecret
	}

	_, err = h.clusterOperator.UpdateBackupPoint(req.Request.Context(), obp)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}

	_ = resp.WriteHeaderAndEntity(http.StatusOK, obp)
}

func (h *handler) DescribeCronBackup(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	resourceVersion := strutil.StringDefaultIfEmpty("0", request.QueryParameter(query.ParameterResourceVersion))
	result, err := h.clusterOperator.GetCronBackupEx(request.Request.Context(), name, resourceVersion)
	if err != nil {
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h *handler) ListCronBackups(request *restful.Request, response *restful.Response) {
	q := query.ParseQueryParameter(request)
	if clientrest.IsInformerRawQuery(request.Request) {
		result, err := h.clusterOperator.ListCronBackups(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	} else {
		result, err := h.clusterOperator.ListCronBackupEx(request.Request.Context(), q)
		if err != nil {
			restplus.HandleInternalError(response, request, err)
			return
		}
		_ = response.WriteHeaderAndEntity(http.StatusOK, result)
	}
}

func (h *handler) CreateCronBackup(request *restful.Request, response *restful.Response) {
	cb := new(v1.CronBackup)
	if err := request.ReadEntity(cb); err != nil {
		restplus.HandleBadRequest(response, request, err)
		return
	}
	if cb.Spec.Schedule != "" {
		s, _ := cron.NewParser(4 | 8 | 16 | 32 | 64).Parse(cb.Spec.Schedule)
		// update the next schedule time
		nextRunAt := metav1.NewTime(s.Next(time.Now()))
		cb.Status.NextScheduleTime = &nextRunAt
	} else if cb.Spec.RunAt != nil {
		cb.Status.NextScheduleTime = cb.Spec.RunAt
	}

	cb.Labels[common.LabelCronBackupEnable] = ""
	createdCB, err := h.clusterOperator.CreateCronBackup(request.Request.Context(), cb)
	if err != nil {
		if apimachineryErrors.IsAlreadyExists(err) {
			restplus.HandleBadRequest(response, request, err)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	_ = response.WriteHeaderAndEntity(http.StatusOK, createdCB)
}

func (h *handler) DeleteCronBackup(request *restful.Request, response *restful.Response) {
	name := request.PathParameter(query.ParameterName)
	err := h.clusterOperator.DeleteCronBackup(request.Request.Context(), name)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			logger.Debug("cronBackup not exist when delete", zap.String("cronBackup", name))
			response.WriteHeader(http.StatusOK)
			return
		}
		restplus.HandleInternalError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusOK)
}

func (h *handler) UpdateCronBackup(req *restful.Request, resp *restful.Response) {
	cb := &v1.CronBackup{}
	if err := req.ReadEntity(cb); err != nil {
		restplus.HandleBadRequest(resp, req, err)
		return
	}
	name := req.PathParameter(query.ParameterName)
	resourceVersion := strutil.StringDefaultIfEmpty("0", req.QueryParameter(query.ParameterResourceVersion))
	ocb, err := h.clusterOperator.GetCronBackupEx(req.Request.Context(), name, resourceVersion)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}
	// disposable performed backup cannot be edited
	if cb.Spec.RunAt != nil {
		if after := metav1.NewTime(time.Now()).After(cb.Status.LastSuccessfulTime.Time); after {
			restplus.HandleInternalError(resp, req, err)
			return
		}
		ocb.Spec.RunAt = cb.Spec.RunAt
	}
	ocb.Labels[common.AnnotationDescription] = cb.Labels[common.AnnotationDescription]
	ocb.Spec.Schedule = cb.Spec.Schedule
	_, err = h.clusterOperator.UpdateCronBackup(req.Request.Context(), ocb)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}
	_ = resp.WriteHeaderAndEntity(http.StatusOK, ocb)
}

func (h *handler) EnableCronBackup(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter(query.ParameterName)
	ctx := req.Request.Context()
	resourceVersion := strutil.StringDefaultIfEmpty("0", req.QueryParameter(query.ParameterResourceVersion))
	cronBackup, err := h.clusterOperator.GetCronBackupEx(ctx, name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(resp, req, err)
			return
		}
		restplus.HandleInternalError(resp, req, err)
		return
	}

	if _, ok := cronBackup.Labels[common.LabelCronBackupEnable]; ok {
		restplus.HandleBadRequest(resp, req, fmt.Errorf("the cronbackup %s is already enable:%v", cronBackup.Name, cronBackup.Labels[common.LabelCronBackupEnable]))
	}
	delete(cronBackup.Labels, common.LabelCronBackupDisable)
	cronBackup.Labels[common.LabelCronBackupEnable] = ""
	updateCronBackup, err := h.clusterOperator.UpdateCronBackup(ctx, cronBackup)
	if err != nil {
		if apimachineryErrors.IsConflict(err) {
			restplus.HandleBadRequest(resp, req, fmt.Errorf("the cronBackup %s has been modified; please apply "+
				"your changes to the latest version and try again", cronBackup.Name))
		}
		restplus.HandleInternalError(resp, req, err)
		return
	}
	_ = resp.WriteHeaderAndEntity(http.StatusOK, updateCronBackup)
}

func (h *handler) DisableCronBackup(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter(query.ParameterName)
	ctx := req.Request.Context()
	resourceVersion := strutil.StringDefaultIfEmpty("0", req.QueryParameter(query.ParameterResourceVersion))
	cronBackup, err := h.clusterOperator.GetCronBackupEx(ctx, name, resourceVersion)
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(resp, req, err)
			return
		}
		restplus.HandleInternalError(resp, req, err)
		return
	}

	if _, ok := cronBackup.Labels[common.LabelCronBackupDisable]; ok {
		restplus.HandleBadRequest(resp, req, fmt.Errorf("the cronbackup %s is already disable:%v", cronBackup.Name, cronBackup.Labels[common.LabelCronBackupDisable]))
	}
	delete(cronBackup.Labels, common.LabelCronBackupEnable)
	cronBackup.Labels[common.LabelCronBackupDisable] = ""
	updateCronBackup, err := h.clusterOperator.UpdateCronBackup(ctx, cronBackup)
	if err != nil {
		if apimachineryErrors.IsConflict(err) {
			restplus.HandleBadRequest(resp, req, fmt.Errorf("the cronbackup %s has been modified; please apply "+
				"your changes to the latest version and try again", updateCronBackup.Name))
		}
		restplus.HandleInternalError(resp, req, err)
		return
	}
	_ = resp.WriteHeaderAndEntity(http.StatusOK, updateCronBackup)
}

func (h *handler) ListBackupsWithCronBackup(req *restful.Request, resp *restful.Response) {
	var result []v1.Backup
	// cronBackup name in path
	cronBackupName := req.PathParameter(query.ParameterName)
	ctx := req.Request.Context()
	cronBackup, err := h.clusterOperator.GetCronBackupEx(ctx, cronBackupName, "0")
	if err != nil {
		if apimachineryErrors.IsNotFound(err) {
			restplus.HandleNotFound(resp, req, err)
			return
		}
		restplus.HandleInternalError(resp, req, err)
		return
	}
	q := query.ParseQueryParameter(req)

	backupList, err := h.clusterOperator.ListBackups(ctx, q)
	if err != nil {
		restplus.HandleInternalError(resp, req, err)
		return
	}
	for _, b := range backupList.Items {
		if len(b.GetOwnerReferences()) != 0 {
			if b.GetOwnerReferences()[0].UID == cronBackup.UID {
				result = append(result, b)
			}
		}
	}

	_ = resp.WriteHeaderAndEntity(http.StatusOK, result)
}
