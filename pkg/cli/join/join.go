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

package join

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"text/template"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kubeclipper/kubeclipper/pkg/cli/config"
	"github.com/kubeclipper/kubeclipper/pkg/cli/sudo"
	"github.com/kubeclipper/kubeclipper/pkg/utils/sshutils"

	"github.com/kubeclipper/kubeclipper/cmd/kcctl/app/options"
	"github.com/kubeclipper/kubeclipper/pkg/cli/logger"
	"github.com/kubeclipper/kubeclipper/pkg/cli/utils"
)

/*
join kubeclipper agent node

Usage:
  kcctl join

Examples:
  kcctl join --agent=1.1.1.1 --deploy-config=~/.kc/deploy-config.yaml
  kcctl join --agent=1.1.1.1 --agent=2.2.2.2 --deploy-config=~/.kc/deploy-config.yaml

Flags:
      --deploy-config string   kcctl deploy config path (default "~/.kc/deploy-config.yaml")
  -h, --help                   help for join
*/

const (
	longDescription = `
  Add Server and Agent nodes on kubeclipper platform.

  At least one Server node must be installed before adding an Agent node.
  deploy-config.yaml file is used to check whether a node can be added correctly.`
	joinExample = `
  # Add agent node use default config.
  kcctl join --agent 192.168.10.123

  # Add agent node specify region.
  kcctl join --agent us-west-1:192.168.10.123

  # Add agent node specify config.
  kcctl join --agent 192.168.10.123 --deploy-config ~/.kc/deploy-config.yaml

  # Add many agent node.
  kcctl join --agent 192.168.10.123,192.168.10.124

  # Add many agent node in same region.
  kcctl join --agent us-west-1:192.168.10.123,192.168.10.124

  # Add many agent node in different region
  kcctl join --agent us-west-1:1.2.3.4 --agent us-west-2:2.3.4.5

  # add many node which has orderly ip.
  # this will add 10 agent,1.1.1.1, 1.1.1.2, ... 1.1.1.10.
  kcctl join --agent us-west-1:1.1.1.1-1.1.1.10


  Please read 'kcctl join -h' get more deploy flags`
)

type JoinOptions struct {
	options.IOStreams
	deployConfig *options.DeployConfig

	agents      []string       // user input agents,maybe with region,need to parse.
	agentRegion options.Agents // format agents
	servers     []string
}

func NewJoinOptions(streams options.IOStreams) *JoinOptions {
	return &JoinOptions{
		IOStreams:    streams,
		deployConfig: options.NewDeployOptions(),
	}
}

func NewCmdJoin(streams options.IOStreams) *cobra.Command {
	o := NewJoinOptions(streams)
	cmd := &cobra.Command{
		Use:                   "join [flags]",
		DisableFlagsInUseLine: true,
		Short:                 "join kubeclipper agent and server node",
		Long:                  longDescription,
		Example:               joinExample,
		Args:                  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			utils.CheckErr(o.Complete())
			utils.CheckErr(o.ValidateArgs())
			if !o.preCheck() {
				return
			}
			utils.CheckErr(o.RunJoinFunc())
		},
	}

	cmd.Flags().StringArrayVar(&o.agents, "agent", o.agents, "join agent node.")
	cmd.Flags().StringVar(&o.deployConfig.Config, "deploy-config", options.DefaultDeployConfigPath, "kcctl deploy config path")
	utils.CheckErr(cmd.MarkFlagRequired("agent"))
	return cmd
}

func (c *JoinOptions) preCheck() bool {
	if !sudo.PreCheck("sudo", c.deployConfig.SSHConfig, c.IOStreams, append(c.agentRegion.ListIP(), c.servers...)) {
		return false
	}
	// check if the node is already added
	for _, agent := range c.agentRegion.ListIP() {
		if !c.preCheckKcAgent(agent) {
			return false
		}
	}

	return true
}

func (c *JoinOptions) Complete() error {
	// deploy config Complete
	if err := c.deployConfig.Complete(); err != nil {
		return err
	}

	agents, err := BuildAgentRegion(c.agents, c.deployConfig.DefaultRegion)
	utils.CheckErr(err)
	c.agentRegion = agents
	c.servers = sets.NewString(c.servers...).List()
	return nil
}

func (c *JoinOptions) ValidateArgs() error {
	if len(c.agents) == 0 {
		return fmt.Errorf("must specified at least one agent node")
	}
	if len(c.deployConfig.ServerIPs) == 0 {
		logger.Error("join an agent node requires specifying at least one server node")
		logger.Info("example: kcctl join --agent 172.10.10.20 --server 172.10.10.10")
		return fmt.Errorf("join an agent node requires specifying at least one server node")
	}
	return nil
}

func (c *JoinOptions) RunJoinFunc() error {
	err := c.RunJoinNode()
	if err != nil {
		return err
	}

	return nil
}

func (c *JoinOptions) RunJoinNode() error {
	if err := c.runJoinServerNode(); err != nil {
		return fmt.Errorf("join server node failed: %s", err.Error())
	}

	if err := c.runJoinAgentNode(); err != nil {
		return fmt.Errorf("join agent node failed: %s", err.Error())
	}

	return nil
}

func (c *JoinOptions) runJoinAgentNode() error {
	var err error
	for region, agents := range c.agentRegion {
		for _, agent := range agents {
			if err = c.agentNodeFiles(region, agent); err != nil {
				return err
			}
			if err = c.enableAgent(region, agent); err != nil {
				return err
			}

		}
	}
	logger.Info("agent node join completed. show command: 'kcctl get node'")
	return nil
}

func (c *JoinOptions) preCheckKcAgent(ip string) bool {
	// check if the node is already in deploy config
	if c.deployConfig.AgentRegions.Exists(ip) {
		logger.Errorf("node %s is already deployed", ip)
		return false
	}
	// check if kc-agent is running
	ret, err := sshutils.SSHCmdWithSudo(c.deployConfig.SSHConfig, ip, "systemctl --all --type service | grep -Fq kc-agent")
	logger.V(2).Info(ret.String())
	if err != nil {
		logger.Errorf("check node %s failed: %s", ip, err.Error())
		return false
	}
	if ret.ExitCode == 0 && ret.Stdout != "" {
		logger.Errorf("kc-agent service exist on %s, please clean old environment", ip)
		return false
	}
	return true
}

func (c *JoinOptions) agentNodeFiles(region, node string) error {
	// send agent binary
	hook := fmt.Sprintf("rm -rf %s && tar -xvf %s -C %s && cp -rf %s /usr/local/bin/",
		filepath.Join(config.DefaultPkgPath, "kc"),
		filepath.Join(config.DefaultPkgPath, path.Base(c.deployConfig.Pkg)),
		config.DefaultPkgPath,
		filepath.Join(config.DefaultPkgPath, "kc/bin/kubeclipper-agent"))
	logger.V(3).Info("join agent node hook:", hook)
	err := utils.SendPackageV2(c.deployConfig.SSHConfig, c.deployConfig.Pkg, []string{node}, config.DefaultPkgPath, nil, &hook)
	if err != nil {
		return errors.Wrap(err, "SendPackageV2")
	}
	err = c.sendCerts()
	if err != nil {
		return err
	}
	agentConfig := c.getKcAgentConfigTemplateContent(region)
	cmdList := []string{
		sshutils.WrapEcho(config.KcAgentService, "/usr/lib/systemd/system/kc-agent.service"), // write systemd file
		"mkdir -pv /etc/kubeclipper-agent ",
		sshutils.WrapEcho(agentConfig, "/etc/kubeclipper-agent/kubeclipper-agent.yaml"), // write agent.yaml
	}
	for _, cmd := range cmdList {
		ret, err := sshutils.SSHCmdWithSudo(c.deployConfig.SSHConfig, node, cmd)
		if err != nil {
			return err
		}
		if err = ret.Error(); err != nil {
			return err
		}
	}
	return nil
}

func (c *JoinOptions) enableAgent(region, node string) error {
	// enable agent service
	ret, err := sshutils.SSHCmdWithSudo(c.deployConfig.SSHConfig, node, "systemctl daemon-reload && systemctl enable kc-agent --now")
	if err != nil {
		return errors.Wrap(err, "enable kc agent")
	}
	if err = ret.Error(); err != nil {
		return errors.Wrap(err, "enable kc agent")
	}
	// update deploy-config.yaml
	c.deployConfig.AgentRegions.Add(region, node)
	return c.deployConfig.Write()
}

func (c *JoinOptions) runJoinServerNode() error {
	for _, node := range c.deployConfig.ServerIPs {
		if err := c.checkServerNode(node); err != nil {
			return err
		}
	}
	return nil
}

func (c *JoinOptions) checkServerNode(node string) error {
	return nil
}

func (c *JoinOptions) serverNodeFiles() error {
	return nil
}

func (c *JoinOptions) enableServerService() error {
	return nil
}

func (c *JoinOptions) getKcAgentConfigTemplateContent(region string) string {
	tmpl, err := template.New("text").Parse(config.KcAgentConfigTmpl)
	if err != nil {
		logger.Fatalf("template parse failed: %s", err.Error())
	}

	var data = make(map[string]interface{})
	data["Region"] = region
	data["AgentID"] = uuid.New().String()
	data["StaticServerAddress"] = fmt.Sprintf("http://%s:%d", c.deployConfig.ServerIPs[0], c.deployConfig.StaticServerPort)
	if c.deployConfig.Debug {
		data["LogLevel"] = "debug"
	} else {
		data["LogLevel"] = "info"
	}
	var endpoint []string
	for _, v := range c.deployConfig.MQ.IPs {
		endpoint = append(endpoint, fmt.Sprintf("%s:%d", v, c.deployConfig.MQ.Port))
	}
	data["MQServerEndpoints"] = endpoint
	data["MQExternal"] = c.deployConfig.MQ.External
	data["MQUser"] = c.deployConfig.MQ.User
	data["MQAuthToken"] = c.deployConfig.MQ.Secret
	data["MQTLS"] = c.deployConfig.MQ.TLS
	if c.deployConfig.MQ.TLS {
		if c.deployConfig.MQ.External {
			data["MQCaPath"] = c.deployConfig.MQ.CA
			data["MQClientCertPath"] = c.deployConfig.MQ.ClientCert
			data["MQClientKeyPath"] = c.deployConfig.MQ.ClientKey
		} else {
			data["MQCaPath"] = filepath.Join(options.DefaultKcAgentConfigPath, options.DefaultCaPath, filepath.Base(c.deployConfig.MQ.CA))
			data["MQClientCertPath"] = filepath.Join(options.DefaultKcAgentConfigPath, options.DefaultNatsPKIPath, filepath.Base(c.deployConfig.MQ.ClientCert))
			data["MQClientKeyPath"] = filepath.Join(options.DefaultKcAgentConfigPath, options.DefaultNatsPKIPath, filepath.Base(c.deployConfig.MQ.ClientKey))
		}
	}
	data["OpLogDir"] = c.deployConfig.OpLog.Dir
	data["OpLogThreshold"] = c.deployConfig.OpLog.Threshold
	var buffer bytes.Buffer
	if err = tmpl.Execute(&buffer, data); err != nil {
		logger.Fatalf("template execute failed: %s", err.Error())
	}
	return buffer.String()
}

func (c *JoinOptions) sendCerts() error {
	// download cert from server
	files := []string{
		c.deployConfig.MQ.CA,
		c.deployConfig.MQ.ClientCert,
		c.deployConfig.MQ.ClientKey,
	}

	for _, file := range files {
		exist, err := sshutils.IsFileExist(file)
		if err != nil {
			return errors.WithMessage(err, "check file exist")
		}
		if !exist {
			if err = c.deployConfig.SSHConfig.DownloadSudo(c.deployConfig.ServerIPs[0], file, file); err != nil {
				return errors.WithMessage(err, "download cert from server")
			}
		}
	}

	if c.deployConfig.MQ.TLS {
		destCa := filepath.Join(options.DefaultKcAgentConfigPath, options.DefaultCaPath)
		destCert := filepath.Join(options.DefaultKcAgentConfigPath, options.DefaultNatsPKIPath)
		destKey := filepath.Join(options.DefaultKcAgentConfigPath, options.DefaultNatsPKIPath)
		if c.deployConfig.MQ.External {
			destCa = filepath.Dir(c.deployConfig.MQ.CA)
			destCert = filepath.Dir(c.deployConfig.MQ.ClientCert)
			destKey = filepath.Dir(c.deployConfig.MQ.ClientKey)
		}

		err := utils.SendPackageV2(c.deployConfig.SSHConfig,
			c.deployConfig.MQ.CA, c.agentRegion.ListIP(), destCa, nil, nil)
		if err != nil {
			return err
		}
		err = utils.SendPackageV2(c.deployConfig.SSHConfig,
			c.deployConfig.MQ.ClientCert, c.agentRegion.ListIP(), destCert, nil, nil)
		if err != nil {
			return err
		}
		err = utils.SendPackageV2(c.deployConfig.SSHConfig,
			c.deployConfig.MQ.ClientKey, c.agentRegion.ListIP(), destKey, nil, nil)
		return err
	}

	return nil
}
